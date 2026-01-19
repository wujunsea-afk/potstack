package db

import (
	"database/sql"
	"time"
)

// User 用户模型
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Login     string    `json:"login"` // Gogs 兼容，与 Username 相同
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
	PublicKey string    `json:"public_key"`
	IsAdmin   bool      `json:"is_admin,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// CreateUser 创建用户
func CreateUser(username, email, fullName string) (*User, error) {
	result, err := db.Exec(
		`INSERT INTO user (username, email, full_name) VALUES (?, ?, ?)`,
		username, email, fullName,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return GetUserByID(id)
}

// GetUserByID 根据 ID 获取用户
func GetUserByID(id int64) (*User, error) {
	user := &User{}
	err := db.QueryRow(
		`SELECT id, username, email, full_name, avatar_url, public_key, is_admin, created_at, updated_at 
		 FROM user WHERE id = ?`, id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.FullName,
		&user.AvatarURL, &user.PublicKey, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	user.Login = user.Username
	return user, nil
}

// GetUserByUsername 根据用户名获取用户
func GetUserByUsername(username string) (*User, error) {
	user := &User{}
	err := db.QueryRow(
		`SELECT id, username, email, full_name, avatar_url, public_key, is_admin, created_at, updated_at 
		 FROM user WHERE username = ?`, username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.FullName,
		&user.AvatarURL, &user.PublicKey, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	user.Login = user.Username
	return user, nil
}

// DeleteUser 删除用户
func DeleteUser(username string) error {
	_, err := db.Exec(`DELETE FROM user WHERE username = ?`, username)
	return err
}

// UpdateUser 更新用户
func UpdateUser(id int64, email, fullName, avatarURL string) error {
	_, err := db.Exec(
		`UPDATE user SET email = ?, full_name = ?, avatar_url = ?, updated_at = CURRENT_TIMESTAMP 
		 WHERE id = ?`,
		email, fullName, avatarURL, id,
	)
	return err
}

// SetUserPublicKey 设置用户公钥
func SetUserPublicKey(username, publicKey string) error {
	_, err := db.Exec(
		`UPDATE user SET public_key = ?, updated_at = CURRENT_TIMESTAMP WHERE username = ?`,
		publicKey, username,
	)
	return err
}

// GetOrCreateUser 获取或创建用户
func GetOrCreateUser(username, email string) (*User, error) {
	user, err := GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}
	return CreateUser(username, email, "")
}
