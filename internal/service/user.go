package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"potstack/config"
	"potstack/internal/db"
)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) CreateUser(ctx context.Context, username, email, password string) (*db.User, error) {
	// Check if user exists
	existing, err := db.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternal, err)
	}
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	// Create user directory
	userPath := filepath.Join(config.RepoDir, username)
	if err := os.MkdirAll(userPath, 0755); err != nil {
		return nil, fmt.Errorf("%w: failed to create user directory: %v", ErrInternal, err)
	}

	// Create DB record
	user, err := db.CreateUser(username, email, password)
	if err != nil {
		// Rollback directory creation
		os.RemoveAll(userPath)
		return nil, fmt.Errorf("%w: failed to create user in db: %v", ErrInternal, err)
	}

	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, username string) error {
	// Delete from DB (cascades to repos)
	if err := db.DeleteUser(username); err != nil {
		return fmt.Errorf("%w: failed to delete user from db: %v", ErrInternal, err)
	}

	// Delete user directory
	userPath := filepath.Join(config.RepoDir, username)
	if err := os.RemoveAll(userPath); err != nil {
		return fmt.Errorf("%w: failed to delete user directory: %v", ErrInternal, err)
	}

	return nil
}

func (s *UserService) GetUser(ctx context.Context, username string) (*db.User, error) {
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternal, err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return user, nil
}

func (s *UserService) SetUserPublicKey(ctx context.Context, username, publicKey string) error {
	return db.SetUserPublicKey(username, publicKey)
}
