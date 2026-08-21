package database

import (
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"work-report/server/internal/model"
)

// Seed 初始化内置数据（幂等，启动时执行一次）：
// 1. 内置角色：admin（管理员）/ user（普通用户）
// 2. 存量用户按 is_admin 迁移到对应角色
// 3. 默认本地管理员账号 admin / 123456（首次登录强制改密）
func Seed(db *gorm.DB) error {
	var adminRole model.Role
	if err := db.Where("name = ?", model.RoleAdminName).First(&adminRole).Error; err == gorm.ErrRecordNotFound {
		adminRole = model.Role{
			Name:        model.RoleAdminName,
			Description: "管理员：拥有系统设置与全部数据权限",
			IsAdmin:     true,
			BuiltIn:     true,
		}
		if err := db.Create(&adminRole).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !adminRole.IsAdmin || !adminRole.BuiltIn {
		// 内置角色属性修复（防止被历史数据带偏后平台失去管理员入口）
		if err := db.Model(&adminRole).Updates(map[string]any{"is_admin": true, "built_in": true}).Error; err != nil {
			return err
		}
	}

	var userRole model.Role
	if err := db.Where("name = ?", model.RoleUserName).First(&userRole).Error; err == gorm.ErrRecordNotFound {
		userRole = model.Role{
			Name:        model.RoleUserName,
			Description: "普通用户：日常使用，无系统设置权限",
			IsAdmin:     false,
			BuiltIn:     true,
		}
		if err := db.Create(&userRole).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if userRole.IsAdmin || !userRole.BuiltIn {
		if err := db.Model(&userRole).Updates(map[string]any{"is_admin": false, "built_in": true}).Error; err != nil {
			return err
		}
	}

	// 存量用户角色迁移：未分配角色的按 is_admin 归位
	if err := db.Model(&model.User{}).Where("role_id IS NULL AND is_admin = ?", true).
		Update("role_id", adminRole.ID).Error; err != nil {
		return err
	}
	if err := db.Model(&model.User{}).Where("role_id IS NULL").
		Update("role_id", userRole.ID).Error; err != nil {
		return err
	}

	// 默认本地管理员账号：admin / 123456，首次登录强制修改密码
	var count int64
	if err := db.Model(&model.User{}).Where("casdoor_id = ?", model.LocalUserPrefix+"admin").Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		admin := model.User{
			CasdoorID:          model.LocalUserPrefix + "admin",
			Name:               "admin",
			PasswordHash:       string(hash),
			MustChangePassword: true,
			IsAdmin:            true,
			RoleID:             &adminRole.ID,
		}
		if err := db.Create(&admin).Error; err != nil {
			return err
		}
		log.Println("default admin account created (admin / 123456), password change required on first login")
	}

	// 内置 AI 模型：默认未启用，管理员在系统设置中填 API Key 并启用后可用
	for _, m := range []model.AIModel{
		{Name: "DeepSeek V4 Flash", Provider: "deepseek", ModelID: "deepseek-v4-flash", BaseURL: "https://api.deepseek.com/v1"},
		{Name: "DeepSeek V4 Pro", Provider: "deepseek", ModelID: "deepseek-v4-pro", BaseURL: "https://api.deepseek.com/v1"},
	} {
		if err := db.Where("provider = ? AND model_id = ?", m.Provider, m.ModelID).FirstOrCreate(&m).Error; err != nil {
			return err
		}
	}
	return nil
}
