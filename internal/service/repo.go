package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"potstack/config"
	"potstack/internal/db"
	"potstack/internal/git"
)

type RepoService struct{}

func NewRepoService() *RepoService {
	return &RepoService{}
}

// CreateRepo 创建仓库
func (s *RepoService) CreateRepo(ctx context.Context, owner, name string) (*db.Repository, error) {
	// Get User
	user, err := db.GetUserByUsername(owner)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternal, err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	// Check if repo exists
	existing, err := db.GetRepositoryByOwnerAndName(owner, name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternal, err)
	}
	if existing != nil {
		return nil, ErrRepoAlreadyExists
	}

	// Init Git Bare Repo
	repoPath := filepath.Join(config.RepoDir, owner, name+".git")
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return nil, fmt.Errorf("%w: failed to create parent dir", ErrInternal)
	}

	uuid, err := git.InitBare(repoPath)
	if err != nil {
		return nil, fmt.Errorf("%w: git init failed: %v", ErrInternal, err)
	}

	// Create DB Record
	repo, err := db.CreateRepository(user.ID, name, "", uuid)
	if err != nil {
		os.RemoveAll(repoPath)
		return nil, fmt.Errorf("%w: db create failed: %v", ErrInternal, err)
	}

	return repo, nil
}

// DeleteRepo 删除仓库
func (s *RepoService) DeleteRepo(ctx context.Context, owner, name string) error {
	// Delete from DB
	if err := db.DeleteRepository(owner, name); err != nil {
		return fmt.Errorf("%w: failed to delete from db: %v", ErrInternal, err)
	}

	// Delete repo directory
	repoPath := filepath.Join(config.RepoDir, owner, name+".git")
	if err := os.RemoveAll(repoPath); err != nil {
		return fmt.Errorf("%w: failed to delete repo directory: %v", ErrInternal, err)
	}

	return nil
}

// GetRepo 获取仓库信息
func (s *RepoService) GetRepo(ctx context.Context, owner, name string) (*db.Repository, error) {
	repo, err := db.GetRepositoryByOwnerAndName(owner, name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternal, err)
	}
	if repo == nil {
		return nil, ErrRepoNotFound
	}
	return repo, nil
}

// AddCollaborator 添加协作者
func (s *RepoService) AddCollaborator(ctx context.Context, owner, repoName, collaboratorName, permission string) error {
	repo, err := db.GetRepositoryByOwnerAndName(owner, repoName)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternal, err)
	}
	if repo == nil {
		return ErrRepoNotFound
	}

	// Get or Create Collaborator User
	user, err := db.GetOrCreateUser(collaboratorName, "")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternal, err)
	}

	if err := db.AddCollaborator(repo.ID, user.ID, permission); err != nil {
		// Replace standard sql error if dup
		// But db package handles it usually?
		// Assuming db.AddCollaborator returns error on duplicate or sql error
		return fmt.Errorf("%w: %v", ErrInternal, err)
	}

	return nil
}

// RemoveCollaborator 移除协作者
func (s *RepoService) RemoveCollaborator(ctx context.Context, owner, repoName, collaboratorName string) error {
	repo, err := s.GetRepo(ctx, owner, repoName)
	if err != nil {
		return err
	}

	user, err := db.GetUserByUsername(collaboratorName)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternal, err)
	}
	if user == nil {
		return nil // idempotent
	}

	if err := db.RemoveCollaborator(repo.ID, user.ID); err != nil {
		return fmt.Errorf("%w: %v", ErrInternal, err)
	}
	return nil
}

// ListCollaborators 列出协作者
func (s *RepoService) ListCollaborators(ctx context.Context, owner, repoName string) ([]*db.CollaboratorResponse, error) {
	repo, err := s.GetRepo(ctx, owner, repoName)
	if err != nil {
		return nil, err
	}

	collabs, err := db.GetCollaborators(repo.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternal, err)
	}

	var response []*db.CollaboratorResponse
	for _, c := range collabs {
		if resp := c.ToResponse(); resp != nil {
			response = append(response, resp)
		}
	}
	if response == nil {
		response = []*db.CollaboratorResponse{}
	}
	return response, nil
}

// IsCollaborator 检查是否为协作者
func (s *RepoService) IsCollaborator(ctx context.Context, owner, repoName, username string) (bool, error) {
	repo, err := s.GetRepo(ctx, owner, repoName)
	if err != nil {
		return false, err
	}

	user, err := db.GetUserByUsername(username)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrInternal, err)
	}
	if user == nil {
		return false, nil
	}

	return db.IsCollaborator(repo.ID, user.ID)
}
