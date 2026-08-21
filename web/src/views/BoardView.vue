<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NSelect, NInput, NEmpty, NDatePicker, useMessage } from 'naive-ui'
import draggable from 'vuedraggable'
import dayjs from 'dayjs'
import { api, type Project, type User, type WorkItem } from '../api'
import { useAuthStore } from '../stores/auth'
import { stripHTML } from '../utils/richText'
import TaskEditor from '../components/TaskEditor.vue'
import WorkItemDetail from '../components/WorkItemDetail.vue'
import BoardCard from '../components/BoardCard.vue'

const auth = useAuthStore()
const message = useMessage()
const route = useRoute()
const router = useRouter()

const projects = ref<Project[]>([])
const users = ref<User[]>([])
const items = ref<WorkItem[]>([])
const currentProjectId = ref<number | null>(null) // null = 全部项目
const loading = ref(false)

// ---- 筛选 / 搜索 / 排序 ----
// 负责人筛选：仅内置 admin 账号默认「全部负责人」，其他账号默认只看自己；可手动切换
const assigneeFilter = ref<number>(auth.user?.name === 'admin' ? 0 : (auth.user?.id ?? 0)) // 0 = 全部负责人
const priorityFilter = ref<string>('all')
const searchKeyword = ref('')
const sortBy = ref<'default' | 'priority' | 'due'>('default')

// 时间段：today / week / month / custom
type RangeMode = 'today' | 'week' | 'month' | 'custom'
const rangeMode = ref<RangeMode>('week')
const monthValue = ref<number>(Date.now())
const customRange = ref<[number, number] | null>(null)

const rangeModes: { key: RangeMode; label: string }[] = [
  { key: 'today', label: '今天' },
  { key: 'week', label: '本周' },
  { key: 'month', label: '按月' },
  { key: 'custom', label: '自定义' },
]

const priorityOptions = [
  { label: '全部优先级', value: 'all' },
  { label: '高', value: 'high' },
  { label: '中', value: 'medium' },
  { label: '低', value: 'low' },
]

const sortOptions = [
  { label: '默认排序', value: 'default' },
  { label: '按优先级', value: 'priority' },
  { label: '按截止时间', value: 'due' },
]

const assigneeOptions = computed(() => [
  { label: '全部负责人', value: 0 },
  ...users.value.map((u) => ({ label: u.name, value: u.id })),
])

// 计算查询日期范围 [from, to]（含当天）
const dateRange = computed<[dayjs.Dayjs, dayjs.Dayjs]>(() => {
  if (rangeMode.value === 'today') {
    return [dayjs().startOf('day'), dayjs().endOf('day')]
  }
  if (rangeMode.value === 'week') {
    // 中文习惯：本周一到周日
    const d = dayjs()
    const dow = d.day() // 0=周日
    const monday = d.subtract(dow === 0 ? 6 : dow - 1, 'day').startOf('day')
    return [monday, monday.add(6, 'day').endOf('day')]
  }
  if (rangeMode.value === 'month') {
    const m = dayjs(monthValue.value)
    return [m.startOf('month'), m.endOf('month')]
  }
  if (customRange.value) {
    return [dayjs(customRange.value[0]).startOf('day'), dayjs(customRange.value[1]).endOf('day')]
  }
  return [dayjs().startOf('week'), dayjs().endOf('week')]
})

const rangeLabel = computed(() => {
  const [from, to] = dateRange.value
  return `${from.format('MM-DD')} ~ ${to.format('MM-DD')}`
})

function setRangeMode(m: RangeMode) {
  rangeMode.value = m
  if (m === 'custom' && !customRange.value) {
    customRange.value = [dayjs().startOf('week').valueOf(), dayjs().endOf('week').valueOf()]
  }
  load()
}

const projectOptions = computed(() => [
  { label: '全部项目', value: 0 },
  ...projects.value.map((p) => ({ label: p.name, value: p.id })),
])

function buildParams() {
  const [from, to] = dateRange.value
  // 看板不做可见性限制（所有任务可见），通过负责人筛选聚焦，默认筛选自己
  // 时间过滤只作用于「已完成」列（done_date_*），待办/进行中任务始终全量展示
  const params: Record<string, string | number> = {
    done_date_from: from.format('YYYY-MM-DD'),
    done_date_to: to.format('YYYY-MM-DD'),
  }
  if (currentProjectId.value) params.project_id = currentProjectId.value
  return params
}

// ---- 三列看板数据 ----
type ColKey = 'todo' | 'doing' | 'done'
const todoList = ref<WorkItem[]>([])
const doingList = ref<WorkItem[]>([])
const doneList = ref<WorkItem[]>([])

// 进行中列的 WIP 上限：超过后列头红色告警，避免任务积压
const DOING_WIP_LIMIT = 5

const priorityWeight: Record<string, number> = { high: 0, medium: 1, low: 2 }

function matchFilters(it: WorkItem) {
  // 负责人筛选：匹配该用户负责的、发布的、以及参与的任务
  if (assigneeFilter.value) {
    const uid = assigneeFilter.value
    const involved =
      it.assignee?.id === uid || it.author_id === uid || (it.participants || []).some((p) => p.id === uid)
    if (!involved) return false
  }
  if (priorityFilter.value !== 'all' && it.priority !== priorityFilter.value) return false
  const kw = searchKeyword.value.trim().toLowerCase()
  // 详细内容为富文本 HTML，搜索时先转纯文本，避免命中标签名
  if (kw && !it.title.toLowerCase().includes(kw) && !stripHTML(it.content || '').toLowerCase().includes(kw)) return false
  return true
}

function sortItems(list: WorkItem[]) {
  const arr = [...list]
  if (sortBy.value === 'priority') {
    arr.sort((a, b) => (priorityWeight[a.priority] ?? 1) - (priorityWeight[b.priority] ?? 1))
  } else if (sortBy.value === 'due') {
    arr.sort((a, b) => {
      const da = a.due_date ? dayjs(a.due_date).valueOf() : Infinity
      const db = b.due_date ? dayjs(b.due_date).valueOf() : Infinity
      return da - db
    })
  } else {
    // 默认按开始日期倒序；未排期（work_date 为空）的待办排最后
    arr.sort((a, b) => {
      const da = a.work_date || ''
      const db = b.work_date || ''
      return da < db ? 1 : da > db ? -1 : 0
    })
  }
  return arr
}

// 从已加载的任务派生三列（前端筛选 + 排序；拖拽直接操作列数组）
function applyFilters() {
  const matched = items.value.filter(matchFilters)
  todoList.value = sortItems(matched.filter((i) => i.status === 'todo'))
  doingList.value = sortItems(matched.filter((i) => i.status === 'doing'))
  doneList.value = sortItems(matched.filter((i) => i.status === 'done'))
}

watch([searchKeyword, assigneeFilter, priorityFilter, sortBy], applyFilters)

const boardColumns = computed(() => [
  { key: 'todo' as ColKey, label: '待办', list: todoList.value, line: 'bg-slate-300 dark:bg-slate-600', iconColor: 'text-slate-400' },
  { key: 'doing' as ColKey, label: '进行中', list: doingList.value, line: 'bg-amber-400', iconColor: 'text-amber-500' },
  { key: 'done' as ColKey, label: '已完成', list: doneList.value, line: 'bg-emerald-500', iconColor: 'text-emerald-500' },
])

const totalCount = computed(() => todoList.value.length + doingList.value.length + doneList.value.length)

async function load() {
  loading.value = true
  try {
    const [p, w, u] = await Promise.all([api.projects(), api.workItems(buildParams()), api.usersCached()])
    projects.value = p.data
    items.value = w.data
    users.value = u.data
    applyFilters()
  } finally {
    loading.value = false
  }
}

// 30s 轮询只刷新任务列表（项目/成员变化不敏感，首次加载即可）；
// 标签页隐藏时暂停，避免多标签页空转打满后端
async function refreshItems() {
  if (document.visibilityState !== 'visible') return
  try {
    const { data } = await api.workItems(buildParams())
    items.value = data
    applyFilters()
  } catch { /* 轮询失败静默，下次再试 */ }
}

let timer: number | undefined
onMounted(() => {
  load()
  openItemFromQuery()
  timer = window.setInterval(refreshItems, 30000)
})
onUnmounted(() => clearInterval(timer))

// 深链接：/board?item=ID&comment=CID（通知/飞书消息跳转）打开任务详情并定位到评论
async function openItemFromQuery() {
  const id = Number(route.query.item)
  if (!id) return
  highlightCommentId.value = Number(route.query.comment) || undefined
  try {
    const { data } = await api.workItem(id)
    detailItem.value = data
    showDetail.value = true
  } catch {
    message.error('任务不存在或已被删除')
  }
  router.replace({ query: {} })
}

// ---- 拖拽切换阶段 ----
const celebratingId = ref<number | null>(null)
let celebrateTimer: number | undefined

function syncItem(id: number, patch: Partial<WorkItem>) {
  const it = items.value.find((i) => i.id === id)
  if (it) Object.assign(it, patch)
}

// vuedraggable 跨列拖动：added 表示元素落入本列；列内移动不持久化
async function onDragChange(evt: any, col: ColKey) {
  const added = evt.added
  if (!added) return
  const item = added.element as WorkItem
  item.status = col
  syncItem(item.id, { status: col })
  if (col === 'done') {
    celebratingId.value = item.id
    clearTimeout(celebrateTimer)
    celebrateTimer = window.setTimeout(() => (celebratingId.value = null), 1200)
  }
  if (col === 'doing' && doingList.value.length > DOING_WIP_LIMIT) {
    message.warning(`进行中已达 ${doingList.value.length} 项，超过上限 ${DOING_WIP_LIMIT}，注意任务积压`)
  }
  try {
    // 用后端返回值同步本地：拖回待办会清空日期与提醒，离开待办会补开始日期
    const { data } = await api.updateStatus(item.id, col)
    syncItem(item.id, data)
  } catch {
    message.error('状态更新失败')
    load()
  }
}

// ---- 多选与批量操作 ----
const selectedIds = ref<Set<number>>(new Set())

function toggleSelect(item: WorkItem) {
  const s = new Set(selectedIds.value)
  if (s.has(item.id)) s.delete(item.id)
  else s.add(item.id)
  selectedIds.value = s
}

function clearSelection() {
  selectedIds.value = new Set()
}

const selectedItems = computed(() => items.value.filter((i) => selectedIds.value.has(i.id)))

// ---- 撤销提示条：删除/批量完成后 6 秒内可撤销 ----
const undoBar = ref<{ text: string; undo: () => Promise<void> } | null>(null)
let undoTimer: number | undefined

function showUndoBar(text: string, undo: () => Promise<void>) {
  clearTimeout(undoTimer)
  undoBar.value = { text, undo }
  undoTimer = window.setTimeout(() => (undoBar.value = null), 6000)
}

async function onUndo() {
  const bar = undoBar.value
  undoBar.value = null
  clearTimeout(undoTimer)
  if (!bar) return
  await bar.undo()
}

// 删除（单个/批量共用）：软删除后可撤销（调 restore 恢复）
async function deleteItems(targets: WorkItem[]) {
  const ids = targets.map((t) => t.id)
  items.value = items.value.filter((i) => !ids.includes(i.id))
  applyFilters()
  clearSelection()
  try {
    for (const id of ids) await api.deleteWorkItem(id)
  } catch {
    message.error('删除失败')
    load()
    return
  }
  showUndoBar(`已删除 ${targets.length} 项任务`, async () => {
    try {
      for (const id of ids) await api.restoreWorkItem(id)
      message.success('已恢复')
    } catch {
      message.error('恢复失败')
    }
    load()
  })
}

// 批量完成：记录原状态，撤销时逐个恢复；无权限（非发布者/负责人/管理员）的任务跳过
async function completeSelected() {
  const canChange = (i: WorkItem) =>
    i.author_id === auth.user?.id || i.assignee?.id === auth.user?.id || !!auth.user?.is_admin
  const pending = selectedItems.value.filter((i) => i.status !== 'done')
  const targets = pending.filter(canChange)
  const skipped = pending.length - targets.length
  clearSelection()
  if (skipped > 0) message.warning(`${skipped} 项任务无权限变更状态，已跳过`)
  if (!targets.length) return
  const prev = targets.map((t) => ({ id: t.id, status: t.status }))
  try {
    for (const t of targets) await api.updateStatus(t.id, 'done')
    message.success(`已完成 ${targets.length} 项任务`)
  } catch {
    message.error('操作失败')
  }
  load()
  showUndoBar(`已完成 ${targets.length} 项任务`, async () => {
    try {
      for (const p of prev) await api.updateStatus(p.id, p.status)
      message.success('已撤销')
    } catch {
      message.error('撤销失败')
    }
    load()
  })
}

function deleteSelected() {
  deleteItems(selectedItems.value)
}

// 卡片内菜单切换状态（拖拽之外的另一种方式）
async function onChangeStatus(item: WorkItem, status: string) {
  if (item.status === status) return
  const old = item.status
  item.status = status as WorkItem['status']
  syncItem(item.id, { status: status as WorkItem['status'] })
  applyFilters()
  if (status === 'done') {
    celebratingId.value = item.id
    clearTimeout(celebrateTimer)
    celebrateTimer = window.setTimeout(() => (celebratingId.value = null), 1200)
  }
  if (status === 'doing' && doingList.value.length > DOING_WIP_LIMIT) {
    message.warning(`进行中已达 ${doingList.value.length} 项，超过上限 ${DOING_WIP_LIMIT}，注意任务积压`)
  }
  try {
    const { data } = await api.updateStatus(item.id, status)
    syncItem(item.id, data)
  } catch {
    item.status = old
    syncItem(item.id, { status: old })
    applyFilters()
    message.error('状态更新失败')
  }
}

// 移动端：状态快速切换（doing ↔ done）
async function toggleDone(item: WorkItem) {
  const next = item.status === 'done' ? 'doing' : 'done'
  const old = item.status
  item.status = next as WorkItem['status']
  syncItem(item.id, { status: next as WorkItem['status'] })
  applyFilters()
  try {
    const { data } = await api.updateStatus(item.id, next)
    syncItem(item.id, data)
  } catch {
    item.status = old
    syncItem(item.id, { status: old })
    applyFilters()
    message.error('状态更新失败')
  }
}

// 移动端：顶部状态标签 + 单列展示
const mobileTab = ref<ColKey>('todo')
const mobileTabs: { key: ColKey; label: string }[] = [
  { key: 'todo', label: '待办' },
  { key: 'doing', label: '进行中' },
  { key: 'done', label: '已完成' },
]
const mobileList = computed(() =>
  mobileTab.value === 'todo' ? todoList.value : mobileTab.value === 'doing' ? doingList.value : doneList.value,
)

// 新建 / 编辑 / 详情
const showEditor = ref(false)
const editItem = ref<WorkItem | null>(null)
const showDetail = ref(false)
const detailItem = ref<WorkItem | null>(null)
// 详情抽屉中需要定位高亮的评论（来自通知跳转）
const highlightCommentId = ref<number | undefined>(undefined)

function openDetail(item: WorkItem) {
  highlightCommentId.value = undefined
  detailItem.value = item
  showDetail.value = true
}

function openCreate() {
  if (projects.value.length === 0) {
    if (auth.user?.is_admin) {
      message.warning('任务必须归属到项目，请先在「系统设置 - 项目管理」中创建')
      router.push('/system?tab=projects')
    } else {
      message.warning('任务必须归属到项目，请联系管理员创建')
    }
    return
  }
  editItem.value = null
  showEditor.value = true
}

function openEdit(item: WorkItem) {
  editItem.value = item
  showEditor.value = true
}

function onSaved() {
  load()
}
</script>

<template>
  <div>
    <!-- 顶部：标题 / 搜索 / 新建 -->
    <div class="mb-3 flex flex-wrap items-center gap-3">
      <h2 class="text-lg font-bold">任务看板</h2>
      <n-input
        v-model:value="searchKeyword"
        size="small"
        clearable
        placeholder="搜索任务标题或内容…"
        style="width: 220px"
      />
      <div class="flex-1"></div>
      <div class="flex items-center gap-1.5 text-sm text-slate-500 dark:text-[#8a8f9c]">
        <span class="relative flex size-2">
          <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60"></span>
          <span class="relative inline-flex size-2 rounded-full bg-emerald-500"></span>
        </span>
        {{ totalCount }} 条任务
      </div>
      <n-button size="small" type="primary" @click="openCreate()">+ 新建任务</n-button>
    </div>

    <!-- 筛选行：项目 / 负责人 / 优先级 / 排序（时间段过滤在「已完成」列正上方，只控制该列） -->
    <div class="mb-4 flex flex-wrap items-center gap-2">
      <n-select
        :value="currentProjectId ?? 0"
        :options="projectOptions"
        style="width: 150px"
        size="small"
        @update:value="(v: number) => { currentProjectId = v || null; load() }"
      />
      <n-select v-model:value="assigneeFilter" :options="assigneeOptions" style="width: 130px" size="small" />
      <n-select v-model:value="priorityFilter" :options="priorityOptions" style="width: 110px" size="small" />
      <n-select v-model:value="sortBy" :options="sortOptions" style="width: 120px" size="small" />
    </div>

    <!-- 桌面端：三列看板 -->
    <div class="hidden md:grid md:grid-cols-3 md:items-start md:gap-4">
      <section
        v-for="col in boardColumns"
        :key="col.key"
        class="relative flex min-h-60 flex-col rounded-xl bg-slate-100/70 dark:bg-[#10131a]"
      >
        <!-- 列头：顶部细线 + 图标 + 文字 + 数量（不只靠颜色区分阶段） -->
        <span class="absolute inset-x-4 top-0 h-0.5 rounded-full" :class="col.line"></span>

        <!-- 已完成列正上方：时间段过滤（只控制已完成列表；待办/进行中始终全量展示） -->
        <div v-if="col.key === 'done'" class="px-3 pt-2.5">
          <div class="flex flex-wrap items-center gap-1">
            <div class="flex gap-0.5 rounded-lg bg-white p-0.5 dark:bg-[#1d212b]">
              <button
                v-for="m in rangeModes"
                :key="m.key"
                class="rounded-md px-2 py-1 text-xs font-medium transition-all"
                :class="
                  rangeMode === m.key
                    ? 'bg-emerald-500/10 text-emerald-600 shadow-sm ring-1 ring-emerald-500/30 dark:text-emerald-400'
                    : 'text-slate-500 hover:text-slate-700 dark:text-[#8a8f9c] dark:hover:text-[#c7cad1]'
                "
                @click="setRangeMode(m.key)"
              >
                {{ m.label }}
              </button>
            </div>
            <n-date-picker
              v-if="rangeMode === 'month'"
              v-model:value="monthValue"
              type="month"
              size="small"
              style="width: 112px"
              :clearable="false"
              @update:value="load"
            />
            <n-date-picker
              v-else-if="rangeMode === 'custom'"
              v-model:value="customRange"
              type="daterange"
              size="small"
              style="width: 248px"
              :clearable="false"
              @update:value="load"
            />
            <span v-else class="text-[.65rem] text-slate-400">{{ rangeLabel }}</span>
          </div>
        </div>
        <!-- 其他列：占位保持三列列头对齐 -->
        <div v-else class="h-[38px]"></div>

        <header class="flex items-center gap-2 px-4 pt-1 pb-2">
          <!-- 待办：空心圆 -->
          <svg v-if="col.key === 'todo'" class="size-3.5" :class="col.iconColor" viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="2" />
          </svg>
          <!-- 进行中：半满圆 -->
          <svg v-else-if="col.key === 'doing'" class="size-3.5" :class="col.iconColor" viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="2" />
            <path d="M8 1.5 A6.5 6.5 0 0 1 8 14.5 Z" fill="currentColor" stroke="none" />
          </svg>
          <!-- 已完成：对勾 -->
          <svg v-else class="size-3.5" :class="col.iconColor" viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="2" />
            <path d="M5 8.2 7.2 10.4 11 6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
          <span class="text-sm font-bold">{{ col.label }}</span>
          <span
            class="rounded-full bg-white px-1.5 py-0.5 text-[.65rem] font-bold dark:bg-[#1d212b]"
            :class="col.key === 'doing' && col.list.length > DOING_WIP_LIMIT ? 'text-red-500' : 'text-slate-500 dark:text-[#8a8f9c]'"
            :title="col.key === 'doing' ? `进行中上限 ${DOING_WIP_LIMIT} 项` : ''"
          >
            {{ col.list.length }}{{ col.key === 'doing' ? `/${DOING_WIP_LIMIT}` : '' }}
          </span>
        </header>

        <draggable
          :list="col.list"
          group="board"
          item-key="id"
          :animation="150"
          filter=".no-drag"
          ghost-class="drag-ghost"
          chosen-class="drag-chosen"
          class="board-scroll flex flex-1 flex-col gap-3 overflow-y-auto px-3 pb-3"
          @change="(evt: any) => onDragChange(evt, col.key)"
        >
          <template #item="{ element }">
            <BoardCard
              :item="element"
              :dimmed="col.key === 'done'"
              :celebrating="celebratingId === element.id"
              :selected="selectedIds.has(element.id)"
              @open="openDetail"
              @edit="openEdit"
              @remove="(i: WorkItem) => deleteItems([i])"
              @toggle-select="toggleSelect"
              @change-status="onChangeStatus"
            />
          </template>
          <template #footer>
            <div v-if="!col.list.length" class="grid place-items-center rounded-lg border border-dashed border-slate-200 py-6 text-xs text-slate-400 dark:border-[#2b2e37]">
              拖入卡片切换到此阶段
            </div>
          </template>
        </draggable>
      </section>
    </div>

    <!-- 移动端：顶部状态标签 + 单列 -->
    <div class="md:hidden">
      <div class="mb-3 flex gap-1 rounded-xl bg-slate-100 p-1 dark:bg-[#171a20]">
        <button
          v-for="t in mobileTabs"
          :key="t.key"
          class="flex-1 rounded-lg px-3 py-1.5 text-sm font-medium transition-all"
          :class="
            mobileTab === t.key
              ? 'bg-white text-violet-600 shadow-sm ring-1 ring-violet-500/30 dark:bg-[#23262f] dark:text-violet-400'
              : 'text-slate-500 dark:text-[#8a8f9c]'
          "
          @click="mobileTab = t.key"
        >
          {{ t.label }}
          {{ t.key === 'todo' ? todoList.length : t.key === 'doing' ? doingList.length : doneList.length }}
        </button>
      </div>

      <!-- 移动端：已完成标签下展示时间段过滤（只控制已完成列表） -->
      <div v-if="mobileTab === 'done'" class="mb-3 flex flex-wrap items-center gap-1.5">
        <div class="flex gap-0.5 rounded-lg bg-slate-100 p-0.5 dark:bg-[#171a20]">
          <button
            v-for="m in rangeModes"
            :key="m.key"
            class="rounded-md px-2 py-1 text-xs font-medium transition-all"
            :class="
              rangeMode === m.key
                ? 'bg-emerald-500/10 text-emerald-600 shadow-sm ring-1 ring-emerald-500/30 dark:text-emerald-400'
                : 'text-slate-500 dark:text-[#8a8f9c]'
            "
            @click="setRangeMode(m.key)"
          >
            {{ m.label }}
          </button>
        </div>
        <n-date-picker
          v-if="rangeMode === 'month'"
          v-model:value="monthValue"
          type="month"
          size="small"
          style="width: 112px"
          :clearable="false"
          @update:value="load"
        />
        <n-date-picker
          v-else-if="rangeMode === 'custom'"
          v-model:value="customRange"
          type="daterange"
          size="small"
          style="width: 248px; max-width: 100%"
          :clearable="false"
          @update:value="load"
        />
        <span v-else class="text-[.65rem] text-slate-400">{{ rangeLabel }}</span>
      </div>
      <div v-if="mobileList.length" class="flex flex-col gap-3">
        <BoardCard
          v-for="it in mobileList"
          :key="it.id"
          :item="it"
          :dimmed="mobileTab === 'done'"
          mobile
          :selected="selectedIds.has(it.id)"
          @open="openDetail"
          @edit="openEdit"
          @remove="(i: WorkItem) => deleteItems([i])"
          @toggle-select="toggleSelect"
          @toggle-done="toggleDone"
          @change-status="onChangeStatus"
        />
      </div>
      <n-empty v-else :description="loading ? '加载中…' : '该阶段暂无任务'" class="py-16" />
    </div>

    <!-- 批量操作浮动栏 -->
    <transition name="fade">
      <div
        v-if="selectedIds.size"
        class="fixed bottom-6 left-1/2 z-30 flex -translate-x-1/2 items-center gap-3 rounded-full border border-slate-200 bg-white px-4 py-2 shadow-lg dark:border-[#2b2e37] dark:bg-[#171a20]"
      >
        <span class="text-sm text-slate-600 dark:text-[#c7cad1]">已选 {{ selectedIds.size }} 项</span>
        <n-button size="tiny" type="primary" @click="completeSelected">标为完成</n-button>
        <n-button size="tiny" type="error" @click="deleteSelected">删除</n-button>
        <n-button size="tiny" quaternary @click="clearSelection">取消</n-button>
      </div>
    </transition>

    <!-- 撤销提示条 -->
    <transition name="fade">
      <div
        v-if="undoBar"
        class="fixed bottom-6 left-1/2 z-30 flex -translate-x-1/2 items-center gap-3 rounded-full bg-slate-800 px-4 py-2 text-sm text-white shadow-lg dark:bg-[#23262f]"
      >
        <span>{{ undoBar.text }}</span>
        <button class="font-bold text-violet-300 hover:text-violet-200" @click="onUndo">撤销</button>
        <button class="text-slate-400 hover:text-white" @click="undoBar = null">×</button>
      </div>
    </transition>

    <TaskEditor
      :show="showEditor"
      :projects="projects"
      :users="users"
      :edit-item="editItem"
      @close="showEditor = false"
      @saved="onSaved"
    />
    <WorkItemDetail :show="showDetail" :item="detailItem" :highlight-comment-id="highlightCommentId" @close="showDetail = false" />
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
