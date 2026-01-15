package service

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrRepoNotFound       = errors.New("repository not found")
	ErrRepoAlreadyExists  = errors.New("repository already exists")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrInvalidParam       = errors.New("invalid parameter")
	ErrCollaboratorExists = errors.New("collaborator already exists")
	ErrInternal           = errors.New("internal error")
)
