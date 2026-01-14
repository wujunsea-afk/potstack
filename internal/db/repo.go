package db

import (
	"database/sql"
	"time"
)

// Repository 仓库模型
type Repository struct {
	ID          int64     `json:"id"`
	OwnerID     int64     `json:"-"`
	Owner       *User     `json:"owner,omitempty"`
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	Description string    `json:"description,omitempty"`
	IsPrivate   bool      `json:"private"`
	UUID        string    `json:"uuid,omitempty"`
	CloneURL    string    `json:"clone_url,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// CreateRepository 创建仓库
func CreateRepository(ownerID int64, name, description, uuid string) (*Repository, error) {
	// 获取 owner 信息
	owner, err := GetUserByID(ownerID)
	if err != nil || owner == nil {
		return nil, err
	}

	fullName := owner.Username + "/" + name

	result, err := db.Exec(
		`INSERT INTO repository (owner_id, name, full_name, description, uuid) VALUES (?, ?, ?, ?, ?)`,
		ownerID, name, fullName, description, uuid,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return GetRepositoryByID(id)
}

// GetRepositoryByID 根据 ID 获取仓库
func GetRepositoryByID(id int64) (*Repository, error) {
	repo := &Repository{}
	err := db.QueryRow(
		`SELECT id, owner_id, name, full_name, description, is_private, uuid, created_at, updated_at 
		 FROM repository WHERE id = ?`, id,
	).Scan(&repo.ID, &repo.OwnerID, &repo.Name, &repo.FullName, &repo.Description,
		&repo.IsPrivate, &repo.UUID, &repo.CreatedAt, &repo.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// 加载 owner
	repo.Owner, _ = GetUserByID(repo.OwnerID)
	return repo, nil
}

// GetRepositoryByOwnerAndName 根据 owner 和仓库名获取仓库
func GetRepositoryByOwnerAndName(owner, name string) (*Repository, error) {
	fullName := owner + "/" + name
	repo := &Repository{}
	err := db.QueryRow(
		`SELECT id, owner_id, name, full_name, description, is_private, uuid, created_at, updated_at 
		 FROM repository WHERE full_name = ?`, fullName,
	).Scan(&repo.ID, &repo.OwnerID, &repo.Name, &repo.FullName, &repo.Description,
		&repo.IsPrivate, &repo.UUID, &repo.CreatedAt, &repo.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// 加载 owner
	repo.Owner, _ = GetUserByID(repo.OwnerID)
	return repo, nil
}

// DeleteRepository 删除仓库
func DeleteRepository(owner, name string) error {
	fullName := owner + "/" + name
	_, err := db.Exec(`DELETE FROM repository WHERE full_name = ?`, fullName)
	return err
}

// GetRepositoriesByOwner 获取用户的所有仓库
func GetRepositoriesByOwner(ownerID int64) ([]*Repository, error) {
	rows, err := db.Query(
		`SELECT id, owner_id, name, full_name, description, is_private, uuid, created_at, updated_at 
		 FROM repository WHERE owner_id = ?`, ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []*Repository
	for rows.Next() {
		repo := &Repository{}
		if err := rows.Scan(&repo.ID, &repo.OwnerID, &repo.Name, &repo.FullName, &repo.Description,
			&repo.IsPrivate, &repo.UUID, &repo.CreatedAt, &repo.UpdatedAt); err != nil {
			return nil, err
		}
		repo.Owner, _ = GetUserByID(repo.OwnerID)
		repos = append(repos, repo)
	}

	return repos, nil
}
