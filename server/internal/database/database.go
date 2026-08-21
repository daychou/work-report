package database

import (
	"fmt"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"work-report/server/internal/model"
)

// Init 连接 MySQL（自动创建数据库）并执行 AutoMigrate
// 约定：不使用数据库物理外键，表间关联仅靠字段值做逻辑映射（由应用层保证）
func Init(dsn string) (*gorm.DB, error) {
	if err := ensureDatabase(dsn); err != nil {
		return nil, err
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
		// 迁移时不创建外键约束
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}

	if err := db.AutoMigrate(model.All()...); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	// 清理历史版本已创建的物理外键约束
	if err := dropAllForeignKeys(db, dsn); err != nil {
		return nil, fmt.Errorf("drop foreign keys: %w", err)
	}
	return db, nil
}

// dropAllForeignKeys 删除当前库中所有物理外键约束（幂等，启动时执行一次）
func dropAllForeignKeys(db *gorm.DB, dsn string) error {
	_, dbName := splitDSN(dsn)
	if dbName == "" {
		return nil
	}
	type fk struct {
		TableName      string `gorm:"column:TABLE_NAME"`
		ConstraintName string `gorm:"column:CONSTRAINT_NAME"`
	}
	var fks []fk
	if err := db.Raw(
		`SELECT TABLE_NAME, CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS
		 WHERE CONSTRAINT_SCHEMA = ? AND CONSTRAINT_TYPE = 'FOREIGN KEY'`, dbName,
	).Scan(&fks).Error; err != nil {
		return err
	}
	for _, k := range fks {
		if err := db.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY `%s`", k.TableName, k.ConstraintName)).Error; err != nil {
			return fmt.Errorf("drop fk %s.%s: %w", k.TableName, k.ConstraintName, err)
		}
	}
	return nil
}

// ensureDatabase 先用不带库名的 DSN 建立连接，CREATE DATABASE IF NOT EXISTS
func ensureDatabase(dsn string) error {
	base, dbName := splitDSN(dsn)
	if dbName == "" {
		return fmt.Errorf("dsn must contain database name: %s", dsn)
	}
	tmp, err := gorm.Open(mysql.Open(base), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("connect mysql (no db): %w", err)
	}
	if err := tmp.Exec(fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName,
	)).Error; err != nil {
		return fmt.Errorf("create database %s: %w", dbName, err)
	}
	sqlDB, _ := tmp.DB()
	if sqlDB != nil {
		_ = sqlDB.Close()
	}
	return nil
}

// splitDSN 把 "user:pass@tcp(host:port)/dbname?params" 拆成
// 不带库名的 DSN 与库名
func splitDSN(dsn string) (base string, dbName string) {
	slash := strings.LastIndex(dsn, "/")
	if slash < 0 {
		return dsn, ""
	}
	base = dsn[:slash+1]
	rest := dsn[slash+1:]
	if i := strings.Index(rest, "?"); i >= 0 {
		dbName = rest[:i]
		base += "?" + rest[i+1:]
	} else {
		dbName = rest
	}
	return base, dbName
}
