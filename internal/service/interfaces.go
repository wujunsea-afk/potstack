package service

import (
	"context"

	"potstack/internal/db"
)

// IUserService 定义用户服务接口
type IUserService interface {
	CreateUser(ctx context.Context, username, email, password string) (*db.User, error)
	DeleteUser(ctx context.Context, username string) error
	GetUser(ctx context.Context, username string) (*db.User, error)
}

// IRepoService 定义仓库服务接口
type IRepoService interface {
	// 仓库管理
	CreateRepo(ctx context.Context, owner, name string) (*db.Repository, error)
	DeleteRepo(ctx context.Context, owner, name string) error
	GetRepo(ctx context.Context, owner, name string) (*db.Repository, error)

	// 协作者管理
	AddCollaborator(ctx context.Context, owner, repo, collaborator, permission string) error
	RemoveCollaborator(ctx context.Context, owner, repo, collaborator string) error
	ListCollaborators(ctx context.Context, owner, repo string) ([]*db.CollaboratorResponse, error)
	IsCollaborator(ctx context.Context, owner, repo, user string) (bool, error)
}
