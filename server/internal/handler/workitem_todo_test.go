package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"work-report/server/internal/database"
	"work-report/server/internal/middleware"
	"work-report/server/internal/model"
)

// 待办/排期逻辑：work_date 可空、未来开始强制待办、拖回待办清空日期、离开待办补开始日期
func TestTodoScheduling(t *testing.T) {
	db, err := database.Init("root:workreport123@tcp(127.0.0.1:3307)/work_report?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		t.Skipf("mysql not available: %v", err)
	}
	h := &WorkItemHandler{db: db}

	user := model.User{CasdoorID: ManualUserPrefix + "待办测试", Name: "待办测试"}
	db.Where("casdoor_id = ?", user.CasdoorID).FirstOrCreate(&user)
	project := model.Project{Name: "待办测试项目", OwnerID: user.ID}
	db.Where("name = ?", project.Name).FirstOrCreate(&project)
	t.Cleanup(func() {
		db.Unscoped().Where("author_id = ?", user.ID).Delete(&model.WorkItem{})
		db.Unscoped().Where("id = ?", project.ID).Delete(&model.Project{})
		db.Unscoped().Where("id = ?", user.ID).Delete(&model.User{})
	})

	create := func(body map[string]any) model.WorkItem {
		t.Helper()
		buf, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/work-items", bytes.NewReader(buf))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(middleware.CtxUserKey, &user)
		h.Create(c)
		if w.Code != 201 {
			t.Fatalf("create failed: %d %s", w.Code, w.Body.String())
		}
		var it model.WorkItem
		if err := json.Unmarshal(w.Body.Bytes(), &it); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return it
	}

	updateStatus := func(id uint, status string) model.WorkItem {
		t.Helper()
		buf, _ := json.Marshal(map[string]any{"status": status})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/", bytes.NewReader(buf))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(id))}}
		c.Set(middleware.CtxUserKey, &user)
		h.UpdateStatus(c)
		if w.Code != 200 {
			t.Fatalf("updateStatus failed: %d %s", w.Code, w.Body.String())
		}
		var it model.WorkItem
		if err := json.Unmarshal(w.Body.Bytes(), &it); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return it
	}

	future := time.Now().Add(72 * time.Hour).Format("2006-01-02")

	// 1. 无开始日期 → 待办（即使请求 doing）
	it1 := create(map[string]any{"title": "未排期待办", "project_id": project.ID, "work_date": "", "status": "doing"})
	if it1.Status != "todo" || it1.WorkDate != nil {
		t.Fatalf("无日期任务应为待办且无日期, got status=%s work_date=%v", it1.Status, it1.WorkDate)
	}

	// 2. 未来开始日期 → 待办（即使请求 doing）
	it2 := create(map[string]any{"title": "未来任务", "project_id": project.ID, "work_date": future, "status": "doing"})
	if it2.Status != "todo" || it2.WorkDate == nil {
		t.Fatalf("未来任务应为待办且保留开始日期, got status=%s work_date=%v", it2.Status, it2.WorkDate)
	}

	// 3. 待办（未排期）→ 进行中：开始日期补为今天
	it3 := updateStatus(it1.ID, "doing")
	today := time.Now().Format("2006-01-02")
	if it3.Status != "doing" || it3.WorkDate == nil || it3.WorkDate.Format("2006-01-02") != today {
		t.Fatalf("离开待办应补开始日期为今天, got work_date=%v", it3.WorkDate)
	}

	// 4. 进行中 → 待办：清空日期与提醒
	it4 := updateStatus(it3.ID, "todo")
	if it4.Status != "todo" || it4.WorkDate != nil || it4.DueDate != nil || it4.DueRemind || it4.StartRemind {
		t.Fatalf("拖回待办应清空日期与提醒, got work_date=%v due_date=%v due_remind=%v start_remind=%v",
			it4.WorkDate, it4.DueDate, it4.DueRemind, it4.StartRemind)
	}

	// 5. 待办（未排期）→ 完成：补开始日期 + done_at
	it5 := updateStatus(it4.ID, "done")
	if it5.Status != "done" || it5.WorkDate == nil || it5.DoneAt == nil {
		t.Fatalf("待办直接完成应补开始日期与完成时间, got work_date=%v done_at=%v", it5.WorkDate, it5.DoneAt)
	}
}

// 状态流转的日期兜底：编辑待办排期≤今天自动进行中；完成时无截止日期补当天；已完成→进行中按截止日期去留
func TestStatusDateTransitions(t *testing.T) {
	db, err := database.Init("root:workreport123@tcp(127.0.0.1:3307)/work_report?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		t.Skipf("mysql not available: %v", err)
	}
	h := &WorkItemHandler{db: db}

	user := model.User{CasdoorID: ManualUserPrefix + "流转测试", Name: "流转测试"}
	db.Where("casdoor_id = ?", user.CasdoorID).FirstOrCreate(&user)
	project := model.Project{Name: "流转测试项目", OwnerID: user.ID}
	db.Where("name = ?", project.Name).FirstOrCreate(&project)
	t.Cleanup(func() {
		db.Unscoped().Where("author_id = ?", user.ID).Delete(&model.WorkItem{})
		db.Unscoped().Where("id = ?", project.ID).Delete(&model.Project{})
		db.Unscoped().Where("id = ?", user.ID).Delete(&model.User{})
	})

	call := func(method, path string, id uint, body map[string]any) model.WorkItem {
		t.Helper()
		buf, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/api/work-items/"+path, bytes.NewReader(buf))
		c.Request.Header.Set("Content-Type", "application/json")
		if id > 0 {
			c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(id))}}
		}
		c.Set(middleware.CtxUserKey, &user)
		if method == "POST" {
			h.Create(c)
		} else if path == "status" {
			h.UpdateStatus(c)
		} else {
			h.Update(c)
		}
		if w.Code/100 != 2 {
			t.Fatalf("%s %s failed: %d %s", method, path, w.Code, w.Body.String())
		}
		var it model.WorkItem
		if err := json.Unmarshal(w.Body.Bytes(), &it); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return it
	}

	today := time.Now().Format("2006-01-02")
	future := time.Now().Add(72 * time.Hour).Format("2006-01-02")

	// 1. 编辑待办任务，开始日期排到今天 → 保存后自动进入进行中
	it := call("POST", "/", 0, map[string]any{"title": "待办排期", "project_id": project.ID, "work_date": "", "status": "todo"})
	if it.Status != "todo" {
		t.Fatalf("前置条件失败：应为待办, got %s", it.Status)
	}
	it = call("PUT", "update", it.ID, map[string]any{"work_date": today})
	if it.Status != "doing" {
		t.Fatalf("待办排期到今天应自动进入进行中, got %s", it.Status)
	}

	// 2. 进行中 → 已完成：无截止日期 → 自动补为当天
	it = call("PUT", "status", it.ID, map[string]any{"status": "done"})
	if it.DueDate == nil || it.DueDate.Format("2006-01-02") != today {
		t.Fatalf("完成时应补截止日期为当天, got %v", it.DueDate)
	}

	// 3. 已完成 → 进行中：截止日期=今天（≤今天）→ 清空
	it = call("PUT", "status", it.ID, map[string]any{"status": "doing"})
	if it.DueDate != nil {
		t.Fatalf("重开时已到期截止日期应清空, got %v", it.DueDate)
	}

	// 4. 已完成 → 进行中：截止日期为未来 → 保留
	it2 := call("POST", "/", 0, map[string]any{
		"title": "未来截止", "project_id": project.ID, "work_date": today, "due_date": future, "status": "doing",
	})
	it2 = call("PUT", "status", it2.ID, map[string]any{"status": "done"})
	it2 = call("PUT", "status", it2.ID, map[string]any{"status": "doing"})
	if it2.DueDate == nil || it2.DueDate.Format("2006-01-02") != future {
		t.Fatalf("未来截止日期重开后应保留, got %v", it2.DueDate)
	}
}
