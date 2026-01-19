package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"potstack/config"
	"potstack/internal/api"
	"potstack/internal/db"
	"potstack/internal/service"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite" // SQLite 驱动
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T, baseDir string) {
	// 创建系统仓库目录结构（数据库需要这个路径存在）
	repoDir := filepath.Join(baseDir, "repo")
	potstackDir := filepath.Join(repoDir, "potstack", "repo.git", "data")
	if err := os.MkdirAll(potstackDir, 0755); err != nil {
		t.Fatalf("Failed to create potstack dir: %v", err)
	}

	// 初始化数据库
	config.DataDir = baseDir
	config.RepoDir = repoDir
	if err := db.Init(repoDir); err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	// 初始化 Service (依赖已初始化的 DB)
	us := service.NewUserService()
	rs := service.NewRepoService()
	server := api.NewServer(us, rs)

	r := gin.New()
	v1 := r.Group("/api/v1")
	{
		admin := v1.Group("/admin")
		admin.POST("/users", server.CreateUserHandler)
		admin.POST("/users/:username/repos", server.CreateRepoHandler)
		admin.DELETE("/users/:username", server.DeleteUserHandler)

		repos := v1.Group("/repos")
		repos.GET("/:owner/:repo", server.GetRepoHandler)
		repos.DELETE("/:owner/:repo", server.DeleteRepoHandler)
		repos.GET("/:owner/:repo/collaborators", server.ListCollaboratorsHandler)
		repos.GET("/:owner/:repo/collaborators/:collaborator", server.CheckCollaboratorHandler)
		repos.PUT("/:owner/:repo/collaborators/:collaborator", server.AddCollaboratorHandler)
		repos.DELETE("/:owner/:repo/collaborators/:collaborator", server.RemoveCollaboratorHandler)
	}
	r.GET("/health", api.HealthCheckHandler)
	return r
}

// TestHealthCheck 健康检查接口测试
func TestHealthCheck(t *testing.T) {
	r := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "UP")
}

// TestUserCRUD 用户增删测试
func TestUserCRUD(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "potstack_test_user_*")
	defer os.RemoveAll(tmpDir)
	setupTestDB(t, tmpDir)
	defer db.Reset()

	r := setupRouter()

	// 1. 创建用户
	w := httptest.NewRecorder()
	body, _ := json.Marshal(api.CreateUserOption{Username: "alice", Email: "alice@example.com"})
	req, _ := http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.DirExists(t, filepath.Join(config.RepoDir, "alice"))
	t.Log("✅ 用户创建成功")

	// 2. 重复创建应失败
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	t.Log("✅ 重复创建用户正确返回冲突")

	// 3. 删除用户
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/admin/users/alice", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.NoDirExists(t, filepath.Join(config.RepoDir, "alice"))
	t.Log("✅ 用户删除成功")
}

// TestRepoCRUD 仓库增删查测试
func TestRepoCRUD(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "potstack_test_repo_*")
	defer os.RemoveAll(tmpDir)
	setupTestDB(t, tmpDir)
	defer db.Reset()

	r := setupRouter()

	// 1. 先创建用户
	w := httptest.NewRecorder()
	userBody, _ := json.Marshal(api.CreateUserOption{Username: "bob", Email: "bob@example.com"})
	req, _ := http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(userBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// 2. 创建仓库
	w = httptest.NewRecorder()
	repoBody, _ := json.Marshal(api.CreateRepoOption{Name: "myproject", Description: "Test project"})
	req, _ = http.NewRequest("POST", "/api/v1/admin/users/bob/repos", bytes.NewBuffer(repoBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.DirExists(t, filepath.Join(config.RepoDir, "bob", "myproject.git"))
	t.Log("✅ 仓库创建成功")

	// 3. 获取仓库信息
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/repos/bob/myproject", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var repo db.Repository
	json.Unmarshal(w.Body.Bytes(), &repo)
	assert.Equal(t, "myproject", repo.Name)
	assert.Equal(t, "bob/myproject", repo.FullName)
	t.Log("✅ 获取仓库信息成功")

	// 4. 删除仓库
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/repos/bob/myproject", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.NoDirExists(t, filepath.Join(config.RepoDir, "bob", "myproject.git"))
	t.Log("✅ 仓库删除成功")
}

// TestCollaboratorCRUD 协作者增删查测试
func TestCollaboratorCRUD(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "potstack_test_collab_*")
	defer os.RemoveAll(tmpDir)
	setupTestDB(t, tmpDir)
	defer db.Reset()

	r := setupRouter()

	// 1. 创建仓库所有者
	w := httptest.NewRecorder()
	body, _ := json.Marshal(api.CreateUserOption{Username: "owner1"})
	req, _ := http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// 2. 创建仓库
	w = httptest.NewRecorder()
	body, _ = json.Marshal(api.CreateRepoOption{Name: "shared-repo"})
	req, _ = http.NewRequest("POST", "/api/v1/admin/users/owner1/repos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// 3. 添加协作者
	w = httptest.NewRecorder()
	body, _ = json.Marshal(api.AddCollaboratorOption{Permission: "write"})
	req, _ = http.NewRequest("PUT", "/api/v1/repos/owner1/shared-repo/collaborators/collab1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	t.Log("✅ 添加协作者成功")

	// 4. 列出协作者
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/repos/owner1/shared-repo/collaborators", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var collaborators []db.CollaboratorResponse
	json.Unmarshal(w.Body.Bytes(), &collaborators)
	assert.Len(t, collaborators, 1)
	assert.Equal(t, "collab1", collaborators[0].Username)
	assert.True(t, collaborators[0].Permissions.Push)
	t.Log("✅ 列出协作者成功")

	// 5. 检查是否为协作者
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/repos/owner1/shared-repo/collaborators/collab1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	t.Log("✅ 检查协作者成功")

	// 6. 检查非协作者
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/repos/owner1/shared-repo/collaborators/unknown", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	t.Log("✅ 非协作者正确返回 404")

	// 7. 移除协作者
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/repos/owner1/shared-repo/collaborators/collab1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	t.Log("✅ 移除协作者成功")

	// 8. 确认已移除
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/repos/owner1/shared-repo/collaborators", nil)
	r.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &collaborators)
	assert.Len(t, collaborators, 0)
	t.Log("✅ 确认协作者已移除")
}

// TestUserNotFound 用户不存在测试
func TestUserNotFound(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "potstack_test_notfound_*")
	defer os.RemoveAll(tmpDir)
	setupTestDB(t, tmpDir)
	defer db.Reset()

	r := setupRouter()

	// 创建仓库时用户不存在
	w := httptest.NewRecorder()
	body, _ := json.Marshal(api.CreateRepoOption{Name: "test"})
	req, _ := http.NewRequest("POST", "/api/v1/admin/users/nonexistent/repos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	t.Log("✅ 用户不存在正确返回 404")
}

// TestRepoNotFound 仓库不存在测试
func TestRepoNotFound(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "potstack_test_repo_notfound_*")
	defer os.RemoveAll(tmpDir)
	setupTestDB(t, tmpDir)
	defer db.Reset()

	r := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/repos/unknown/repo", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	t.Log("✅ 仓库不存在正确返回 404")
}
