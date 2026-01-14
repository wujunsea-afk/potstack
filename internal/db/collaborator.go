package db

import (
	"database/sql"
	"time"
)

// Collaborator 协作者模型
type Collaborator struct {
	ID         int64     `json:"-"`
	RepoID     int64     `json:"-"`
	UserID     int64     `json:"-"`
	Permission string    `json:"-"`
	CreatedAt  time.Time `json:"-"`

	// 用于 API 响应
	User        *User        `json:"-"`
	Permissions *Permissions `json:"permissions,omitempty"`
}

// Permissions 权限结构（Gogs 兼容）
type Permissions struct {
	Admin bool `json:"admin"`
	Push  bool `json:"push"`
	Pull  bool `json:"pull"`
}

// PermissionToPermissions 将权限字符串转换为结构
func PermissionToPermissions(permission string) *Permissions {
	switch permission {
	case "admin":
		return &Permissions{Admin: true, Push: true, Pull: true}
	case "write":
		return &Permissions{Admin: false, Push: true, Pull: true}
	case "read":
		return &Permissions{Admin: false, Push: false, Pull: true}
	default:
		return &Permissions{Admin: false, Push: true, Pull: true}
	}
}

// AddCollaborator 添加协作者
func AddCollaborator(repoID, userID int64, permission string) error {
	if permission == "" {
		permission = "write"
	}

	// 使用 UPSERT
	_, err := db.Exec(
		`INSERT INTO collaborator (repo_id, user_id, permission) 
		 VALUES (?, ?, ?) 
		 ON CONFLICT(repo_id, user_id) DO UPDATE SET permission = ?`,
		repoID, userID, permission, permission,
	)
	return err
}

// RemoveCollaborator 移除协作者
func RemoveCollaborator(repoID, userID int64) error {
	_, err := db.Exec(
		`DELETE FROM collaborator WHERE repo_id = ? AND user_id = ?`,
		repoID, userID,
	)
	return err
}

// IsCollaborator 判断是否为协作者
func IsCollaborator(repoID, userID int64) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM collaborator WHERE repo_id = ? AND user_id = ?`,
		repoID, userID,
	).Scan(&count)

	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetCollaborator 获取协作者信息
func GetCollaborator(repoID, userID int64) (*Collaborator, error) {
	collab := &Collaborator{}
	err := db.QueryRow(
		`SELECT id, repo_id, user_id, permission, created_at 
		 FROM collaborator WHERE repo_id = ? AND user_id = ?`,
		repoID, userID,
	).Scan(&collab.ID, &collab.RepoID, &collab.UserID, &collab.Permission, &collab.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	collab.User, _ = GetUserByID(collab.UserID)
	collab.Permissions = PermissionToPermissions(collab.Permission)
	return collab, nil
}

// GetCollaborators 获取仓库的所有协作者（使用 JOIN 避免嵌套查询）
func GetCollaborators(repoID int64) ([]*Collaborator, error) {
	rows, err := db.Query(
		`SELECT c.id, c.repo_id, c.user_id, c.permission, c.created_at,
		        u.id, u.username, u.email, u.full_name, u.avatar_url, u.is_admin
		 FROM collaborator c
		 LEFT JOIN user u ON c.user_id = u.id
		 WHERE c.repo_id = ?`,
		repoID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collaborators []*Collaborator
	for rows.Next() {
		collab := &Collaborator{User: &User{}}
		if err := rows.Scan(
			&collab.ID, &collab.RepoID, &collab.UserID, &collab.Permission, &collab.CreatedAt,
			&collab.User.ID, &collab.User.Username, &collab.User.Email,
			&collab.User.FullName, &collab.User.AvatarURL, &collab.User.IsAdmin,
		); err != nil {
			return nil, err
		}
		collab.Permissions = PermissionToPermissions(collab.Permission)
		collaborators = append(collaborators, collab)
	}

	return collaborators, nil
}

// CollaboratorResponse Gogs 兼容的协作者响应
type CollaboratorResponse struct {
	ID          int64        `json:"id"`
	Username    string       `json:"username"`
	Login       string       `json:"login"`
	FullName    string       `json:"full_name"`
	Email       string       `json:"email"`
	AvatarURL   string       `json:"avatar_url"`
	Permissions *Permissions `json:"permissions"`
}

// ToResponse 转换为 API 响应格式
func (c *Collaborator) ToResponse() *CollaboratorResponse {
	if c.User == nil {
		return nil
	}
	return &CollaboratorResponse{
		ID:          c.User.ID,
		Username:    c.User.Username,
		Login:       c.User.Username,
		FullName:    c.User.FullName,
		Email:       c.User.Email,
		AvatarURL:   c.User.AvatarURL,
		Permissions: c.Permissions,
	}
}
