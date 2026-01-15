package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	db   *sql.DB
	once sync.Once
)

// DBPath 返回数据库文件路径
// 数据库位于 potstack/repo.git/data/potstack.db
func DBPath(repoDir string) string {
	return filepath.Join(repoDir, "potstack", "repo.git", "data", "potstack.db")
}

// Init 初始化数据库连接
func Init(repoDir string) error {
	var initErr error
	once.Do(func() {
		dbPath := DBPath(repoDir)

		// 确保目录存在
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			initErr = fmt.Errorf("failed to create db directory: %w", err)
			return
		}

		// 打开数据库
		var err error
		db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			initErr = fmt.Errorf("failed to open database: %w", err)
			return
		}

		// 设置连接池
		db.SetMaxOpenConns(1) // SQLite 单连接
		db.SetMaxIdleConns(1)

		// 启用外键约束
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			initErr = fmt.Errorf("failed to enable foreign keys: %w", err)
			return
		}

		// 初始化表结构
		if err := initTables(); err != nil {
			initErr = fmt.Errorf("failed to init tables: %w", err)
			return
		}

		log.Printf("Database initialized: %s", dbPath)
	})
	return initErr
}

// Get 获取数据库连接
func Get() *sql.DB {
	return db
}

// Close 关闭数据库连接
func Close() error {
	if db != nil {
		err := db.Close()
		db = nil
		once = sync.Once{} // 重置 once，允许重新初始化
		return err
	}
	return nil
}

// Reset 重置数据库（仅用于测试）
func Reset() {
	if db != nil {
		db.Close()
		db = nil
	}
	once = sync.Once{}
}

// initTables 初始化表结构
func initTables() error {
	schemas := []string{
		// 用户表
		`CREATE TABLE IF NOT EXISTS user (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			username    TEXT NOT NULL UNIQUE,
			email       TEXT DEFAULT '',
			full_name   TEXT DEFAULT '',
			avatar_url  TEXT DEFAULT '',
			is_admin    INTEGER DEFAULT 0,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_username ON user(username)`,

		// 仓库表
		`CREATE TABLE IF NOT EXISTS repository (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			owner_id    INTEGER NOT NULL,
			name        TEXT NOT NULL,
			full_name   TEXT NOT NULL,
			description TEXT DEFAULT '',
			is_private  INTEGER DEFAULT 0,
			uuid        TEXT DEFAULT '',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_id) REFERENCES user(id) ON DELETE CASCADE,
			UNIQUE(owner_id, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_repository_owner_id ON repository(owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_repository_full_name ON repository(full_name)`,

		// 协作者表
		`CREATE TABLE IF NOT EXISTS collaborator (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			repo_id     INTEGER NOT NULL,
			user_id     INTEGER NOT NULL,
			permission  TEXT DEFAULT 'write',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (repo_id) REFERENCES repository(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE,
			UNIQUE(repo_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_collaborator_repo_id ON collaborator(repo_id)`,
		`CREATE INDEX IF NOT EXISTS idx_collaborator_user_id ON collaborator(user_id)`,
	}

	for _, schema := range schemas {
		if _, err := db.Exec(schema); err != nil {
			return fmt.Errorf("failed to exec schema: %s, error: %w", schema, err)
		}
	}

	return nil
}

// IsReady 检查数据库是否已初始化
func IsReady() bool {
	return db != nil
}
