package api

import (
	"potstack/internal/service"
)

type Server struct {
	userService service.IUserService
	repoService service.IRepoService
}

func NewServer(us service.IUserService, rs service.IRepoService) *Server {
	return &Server{
		userService: us,
		repoService: rs,
	}
}
