package api

// CreateRepoOption 代表创建仓库的选项
type CreateRepoOption struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
	AutoInit    bool   `json:"auto_init"`
}

// CreateUserOption 代表创建用户的选项
type CreateUserOption struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email"`
}

// User 代表系统中的用户
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Repository 代表系统中的仓库
type Repository struct {
	ID          int64  `json:"id"`
	Owner       *User  `json:"owner"`
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
	CloneURL    string `json:"clone_url"`
	UUID        string `json:"uuid"`
}
