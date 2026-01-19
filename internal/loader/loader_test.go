package loader

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"potstack/internal/db"
	"potstack/internal/docker"
	"potstack/internal/models"
	"potstack/internal/service"

	"gopkg.in/yaml.v3"
)

// MockUserService 模拟用户服务
type MockUserService struct {
	users map[string]*db.User
}

func NewMockUserService() *MockUserService {
	return &MockUserService{
		users: make(map[string]*db.User),
	}
}

func (m *MockUserService) GetUser(ctx context.Context, username string) (*db.User, error) {
	if user, ok := m.users[username]; ok {
		return user, nil
	}
	return nil, service.ErrUserNotFound
}

func (m *MockUserService) CreateUser(ctx context.Context, username, email, password string) (*db.User, error) {
	if _, ok := m.users[username]; ok {
		return nil, service.ErrUserAlreadyExists
	}
	user := &db.User{
		ID:       int64(len(m.users) + 1),
		Username: username,
		Email:    email,
	}
	m.users[username] = user
	return user, nil
}

func (m *MockUserService) DeleteUser(ctx context.Context, username string) error {
	delete(m.users, username)
	return nil
}

func (m *MockUserService) SetUserPublicKey(ctx context.Context, username, publicKey string) error {
	if user, ok := m.users[username]; ok {
		user.PublicKey = publicKey
		return nil
	}
	return service.ErrUserNotFound
}

// MockRepoService 模拟仓库服务 (我们主要测试 Key Pinning，仓库部分可以简化)
type MockRepoService struct{}

func (m *MockRepoService) CreateRepo(ctx context.Context, owner, name string) (*db.Repository, error) {
	return &db.Repository{}, nil
}
func (m *MockRepoService) GetRepo(ctx context.Context, owner, name string) (*db.Repository, error) {
	return &db.Repository{}, nil
}
func (m *MockRepoService) GetRepoPath(owner, name string) string {
	return filepath.Join("/tmp/mock-repo", owner, name+".git")
}
func (m *MockRepoService) DeleteRepo(ctx context.Context, owner, name string) error {
	return nil
}
func (m *MockRepoService) AddCollaborator(ctx context.Context, owner, repo, username, permission string) error {
	return nil
}
func (m *MockRepoService) RemoveCollaborator(ctx context.Context, owner, repo, username string) error {
	return nil
}
func (m *MockRepoService) IsCollaborator(ctx context.Context, owner, repo, username string) (bool, error) {
	return true, nil
}
func (m *MockRepoService) ListCollaborators(ctx context.Context, owner, repo string) ([]*db.CollaboratorResponse, error) {
	return nil, nil
}

// Helper: 生成测试用的 PPK 文件
func generateTestPPK(t *testing.T, owner string, pub ed25519.PublicKey, priv ed25519.PrivateKey) string {
	// 1. 创建 Zip 内容
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	// 添加一个文件：owner/app/pot.yml
	f, _ := w.Create(fmt.Sprintf("%s/test-app/pot.yml", owner))
	f.Write([]byte("name: test-app"))
	w.Close()

	zipData := buf.Bytes()

	// 2. 计算签名
	sig := ed25519.Sign(priv, zipData)

	// 3. 构建 Header
	header := make([]byte, 128)
	// Magic: PPK\x00
	copy(header[0:4], []byte{'P', 'P', 'K', 0x00})
	header[4] = 0x01 // Version
	header[5] = 0x00 // Flags
	header[6] = 0x01 // SignAlgo
	header[7] = 0x00 // Reserved1

	// ContentLen (LittleEndian)
	length := uint64(len(zipData))
	for i := 0; i < 8; i++ {
		header[8+i] = byte(length >> (8 * i))
	}

	// PublicKey (32 bytes) offset 16
	copy(header[16:48], pub)

	// Signature (64 bytes) offset 48
	copy(header[48:112], sig)

	// Reserved2 offset 112 (16 bytes, default 0)

	// 4. 写入文件
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s.ppk", owner))
	out, err := os.Create(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	out.Write(header)
	out.Write(zipData)

	return tmpFile
}

func TestDeployPPK_TOFU(t *testing.T) {
	// Setup
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	mockUserSvc := NewMockUserService()
	mockRepoSvc := &MockRepoService{}

	loader := New(&Config{TempDir: os.TempDir()}, mockUserSvc, mockRepoSvc)

	owner := "newuser"
	ppkPath := generateTestPPK(t, owner, pub, priv)
	defer os.Remove(ppkPath)

	// Test TOFU
	err := loader.deployPPK(ppkPath)
	if err != nil {
		t.Fatalf("TOFU deployment failed: %v", err)
	}

	// Verify User Created
	user, err := mockUserSvc.GetUser(context.Background(), owner)
	if err != nil {
		t.Fatalf("User should be created")
	}

	// Verify Key Pinned
	expectedKey := fmt.Sprintf("%x", pub)
	if user.PublicKey != expectedKey {
		t.Errorf("Public key mismatch. Got %s, want %s", user.PublicKey, expectedKey)
	}
}

func TestDeployPPK_Pinning_Success(t *testing.T) {
	// Setup
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	mockUserSvc := NewMockUserService()
	mockRepoSvc := &MockRepoService{}

	owner := "existinguser"
	expectedKey := fmt.Sprintf("%x", pub)

	// Pre-create user with pinned key
	mockUserSvc.CreateUser(context.Background(), owner, "test@test.com", "")
	mockUserSvc.SetUserPublicKey(context.Background(), owner, expectedKey)

	loader := New(&Config{TempDir: os.TempDir()}, mockUserSvc, mockRepoSvc)
	ppkPath := generateTestPPK(t, owner, pub, priv)
	defer os.Remove(ppkPath)

	// Test
	err := loader.deployPPK(ppkPath)
	if err != nil {
		t.Fatalf("Valid deployment should succeed: %v", err)
	}
}

func TestDeployPPK_Pinning_Failure(t *testing.T) {
	// Setup
	pub1, _, _ := ed25519.GenerateKey(rand.Reader)     // Key in DB
	pub2, priv2, _ := ed25519.GenerateKey(rand.Reader) // Key in PPK (Attacker)

	mockUserSvc := NewMockUserService()
	mockRepoSvc := &MockRepoService{}

	owner := "victim"
	pinnedKey := fmt.Sprintf("%x", pub1)

	// Pre-create user with pinned key 1
	mockUserSvc.CreateUser(context.Background(), owner, "test@test.com", "")
	mockUserSvc.SetUserPublicKey(context.Background(), owner, pinnedKey)

	loader := New(&Config{TempDir: os.TempDir()}, mockUserSvc, mockRepoSvc)

	// Generate PPK signed with Key 2
	ppkPath := generateTestPPK(t, owner, pub2, priv2)
	defer os.Remove(ppkPath)

	// Test - Should Fail
	err := loader.deployPPK(ppkPath)
	if err == nil {
		t.Fatal("Deployment should fail due to key mismatch")
	}

	// Verify exact error message or type if needed, but for now checking it fails is good
	t.Logf("Got expected error: %v", err)
}

// ========== Docker 镜像拉取测试 ==========

func TestDockerImageExists(t *testing.T) {
	// 测试一个肯定不存在的镜像
	exists := docker.ImageExists("potstack/nonexistent-image-12345:latest")
	if exists {
		t.Error("Non-existent image should return false")
	}
}

func TestDockerLocalTagNaming(t *testing.T) {
	// 测试本地 Tag 命名规则
	owner := "test-org"
	potname := "my-app"
	expected := "potstack/test-org/my-app:latest"
	actual := fmt.Sprintf("potstack/%s/%s:latest", owner, potname)

	if actual != expected {
		t.Errorf("Local tag mismatch: got %s, want %s", actual, expected)
	}
}

func TestPotConfigDockerField(t *testing.T) {
	// 测试 pot.yml 解析 docker 字段
	yamlContent := `
title: "Test App"
type: "exe"
docker: "nginx:1.25"
`
	var cfg models.PotConfig
	if err := yaml.Unmarshal([]byte(yamlContent), &cfg); err != nil {
		t.Fatalf("Failed to parse pot.yml: %v", err)
	}

	if cfg.Docker != "nginx:1.25" {
		t.Errorf("Docker field mismatch: got %s, want nginx:1.25", cfg.Docker)
	}
}

func TestPotConfigWithoutDocker(t *testing.T) {
	// 测试没有 docker 字段的 pot.yml
	yamlContent := `
title: "Test App"
type: "exe"
`
	var cfg models.PotConfig
	if err := yaml.Unmarshal([]byte(yamlContent), &cfg); err != nil {
		t.Fatalf("Failed to parse pot.yml: %v", err)
	}

	if cfg.Docker != "" {
		t.Errorf("Docker field should be empty, got: %s", cfg.Docker)
	}
}

// ========== Docker 集成测试 ==========

// TestDockerPullAndTag_Integration 真实测试 Docker 拉取
// 使用 busybox:latest 作为测试镜像（很小，约 4MB）
func TestDockerPullAndTag_Integration(t *testing.T) {
	// 跳过条件：使用 -short 标志时跳过
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	remoteImage := "busybox:latest" // 网络不可用时会自动跳过
	localTag := "potstack/integration-test/docker-pull:latest"

	// 清理：先删除可能存在的旧 Tag
	_ = docker.RemoveTag(localTag)

	// 测试拉取并打 Tag
	t.Logf("Pulling %s -> %s", remoteImage, localTag)
	err := docker.PullAndTag(remoteImage, localTag)
	if err != nil {
		// 网络问题时跳过测试（而非失败）
		if strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "net/http") ||
			strings.Contains(err.Error(), "deadline exceeded") {
			t.Skipf("Skipping due to network issue: %v", err)
		}
		t.Fatalf("PullAndTag failed: %v", err)
	}

	// 验证镜像存在
	if !docker.ImageExists(localTag) {
		t.Error("Image should exist after PullAndTag")
	}

	// 清理
	t.Log("Cleaning up...")
	if err := docker.RemoveTag(localTag); err != nil {
		t.Logf("Warning: failed to remove tag: %v", err)
	}
}
