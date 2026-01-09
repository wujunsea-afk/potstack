package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"potstack/config"
	"potstack/internal/api"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupRouter() *gin.Engine {
	r := gin.Default()
	v1 := r.Group("/api/v1")
	{
		admin := v1.Group("/admin")
		admin.POST("/users", api.CreateUserHandler)
		admin.POST("/users/:username/repos", api.CreateRepoHandler)
		admin.DELETE("/users/:username", api.DeleteUserHandler)
	}
	v1.GET("/repos/:owner/:repo", api.GetRepoHandler)
	v1.DELETE("/repos/:username/:reponame", api.DeleteRepoHandler)
	return r
}

func TestUserLifecycle(t *testing.T) {
	// Setup temporary config
	tmpDir, _ := os.MkdirTemp("", "potstack_test")
	defer os.RemoveAll(tmpDir)
	config.RepoRoot = tmpDir

	r := setupRouter()

	// 1. Create User
	w := httptest.NewRecorder()
	body, _ := json.Marshal(api.CreateUserOption{Username: "testuser", Email: "test@example.com"})
	req, _ := http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.DirExists(t, tmpDir+"/testuser")

	// 2. Create Repository
	w = httptest.NewRecorder()
	repoBody, _ := json.Marshal(api.CreateRepoOption{Name: "myrepo"})
	req, _ = http.NewRequest("POST", "/api/v1/admin/users/testuser/repos", bytes.NewBuffer(repoBody))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.DirExists(t, tmpDir+"/testuser/myrepo.git")

	// 3. Get Repository
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/repos/testuser/myrepo", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var repo api.Repository
	json.Unmarshal(w.Body.Bytes(), &repo)
	assert.Equal(t, "myrepo", repo.Name)
	assert.NotEmpty(t, repo.UUID)

	// 4. Delete User (and repos)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/admin/users/testuser", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.NoDirExists(t, tmpDir+"/testuser")
}
