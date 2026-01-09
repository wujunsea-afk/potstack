package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"potstack/config"
	"potstack/internal/api"
	potgit "potstack/internal/git"
	"potstack/internal/router"

	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// setupTestRouter creates a gin Engine with the same routing as the main application,
// but without authentication middleware for testing purposes.
func setupTestRouter() *gin.Engine {
	r := gin.Default()

	// API route group
	v1 := r.Group("/api/v1")
	{
		// Admin user management (auth removed for testing)
		admin := v1.Group("/admin")
		{
			// The user creation endpoint is needed to create the repo owner
			admin.POST("/users", api.CreateUserHandler)
			// The repo creation endpoint
			admin.POST("/users/:username/repos", api.CreateRepoHandler)
		}
	}

	// Git Smart HTTP protocol (auth removed for testing)
	r.Any("/:owner/:reponame/*action", potgit.SmartHTTPServer())

	// Resource routes
	r.GET("/uri/*path", router.ResourceProcessor())
	r.GET("/cdn/*path", router.CDNProcessor())

	r.GET("/health", api.HealthCheckHandler)

	return r
}

// setupTestServer programmatically starts the potstack server for integration testing.
func setupTestServer(t *testing.T) string {
	gin.SetMode(gin.TestMode)
	// Use the new function to get a router with correct routes
	r := setupTestRouter()

	port := "58082" // Using a different port to avoid conflicts
	serverAddr := "localhost:" + port

	go func() {
		if err := r.Run(serverAddr); err != nil {
			t.Logf("Failed to start server: %v", err)
		}
	}()

	healthURL := fmt.Sprintf("http://%s/health", serverAddr)
	for i := 0; i < 20; i++ { // Increased retries
		resp, err := http.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			t.Log("Server is up and running.")
			return serverAddr
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("Server failed to start in time.")
	return ""
}

// TestGitWorkflow fully tests the repository creation, clone, push, and verification process.
func TestGitWorkflow(t *testing.T) {
	// 1. Setup: Create a temporary directory for test data and start the server
	dataDir, err := ioutil.TempDir("", "potstack_test_data_*")
	if err != nil {
		t.Fatalf("Failed to create temp data dir: %v", err)
	}
	defer os.RemoveAll(dataDir)
	t.Logf("Using temp data directory: %s", dataDir)

	// Set the repo root for the test
	config.RepoRoot = dataDir

	serverAddr := setupTestServer(t)
	apiURL := "http://" + serverAddr

	orgName := "test-org"   // This will be the 'owner' parameter in the git URL
	repoName := "test-repo"
	fileName := "README.md"
	fileContent := "Hello, Potstack!"

	// 2. Create Org/User and Repo via API
	// First, create a "user" that will own the repository. Based on main.go, this seems to be a required step.
	createUserURL := fmt.Sprintf("%s/api/v1/admin/users", apiURL)
	userReqBody, _ := json.Marshal(map[string]string{"username": orgName, "password": "password"})
	resp, err := http.Post(createUserURL, "application/json", bytes.NewBuffer(userReqBody))
	if err != nil {
		t.Fatalf("Failed to send create user request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("Expected user creation status 200 or 201, but got %d. Body: %s", resp.StatusCode, string(body))
	}
	t.Logf("Successfully created user '%s'", orgName)
	resp.Body.Close()
	
	// Now, create the repository under that user/org.
	createRepoURL := fmt.Sprintf("%s/api/v1/admin/users/%s/repos", apiURL, orgName)
	repoReqBody, _ := json.Marshal(map[string]string{"name": repoName})
	resp, err = http.Post(createRepoURL, "application/json", bytes.NewBuffer(repoReqBody))
	if err != nil {
		t.Fatalf("Failed to send create repo request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("Expected repo creation status 200 or 201, but got %d. Body: %s", resp.StatusCode, string(body))
	}
	t.Logf("Successfully created repository '%s/%s' via API", orgName, repoName)
	resp.Body.Close()

	// Correct Git URL, without the /git/ prefix
	repoURL := fmt.Sprintf("%s/%s/%s.git", apiURL, orgName, repoName)
	t.Logf("Using Git URL: %s", repoURL)

	// 3. Clone the empty repository
	cloneDir1, err := ioutil.TempDir("", "clone1_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir for first clone: %v", err)
	}
	defer os.RemoveAll(cloneDir1)

	r, err := git.PlainClone(cloneDir1, false, &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
	})
	if err != nil {
		t.Fatalf("Failed to clone repository: %v", err)
	}
	t.Logf("Successfully cloned repository to %s", cloneDir1)

	// 4. Add a new file, commit, and push
	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	filePath := filepath.Join(cloneDir1, fileName)
	err = ioutil.WriteFile(filePath, []byte(fileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write new file: %v", err)
	}

	_, err = w.Add(fileName)
	if err != nil {
		t.Fatalf("Failed to add file to worktree: %v", err)
	}

	commit, err := w.Commit("feat: add README.md", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test Bot",
			Email: "bot@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit changes: %v", err)
	}
	t.Logf("Committed file with hash: %s", commit.String())

	err = r.Push(&git.PushOptions{})
	if err != nil {
		t.Fatalf("Failed to push changes: %v", err)
	}
	t.Log("Successfully pushed changes to remote")

	// 5. Clone the repository again to a new directory
	cloneDir2, err := ioutil.TempDir("", "clone2_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir for second clone: %v", err)
	}
	defer os.RemoveAll(cloneDir2)

	_, err = git.PlainClone(cloneDir2, false, &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
	})
	if err != nil {
		t.Fatalf("Failed to perform second clone: %v", err)
	}
	t.Logf("Successfully cloned repository for verification to %s", cloneDir2)

	// 6. Verify the file exists in the new clone
	verifyFilePath := filepath.Join(cloneDir2, fileName)
	content, err := ioutil.ReadFile(verifyFilePath)
	if err != nil {
		t.Fatalf("Failed to read file in second clone: %v", err)
	}

	if string(content) != fileContent {
		t.Fatalf("File content mismatch. Expected '%s', got '%s'", fileContent, string(content))
	}

	t.Log("SUCCESS: File verification passed.")
}

func TestResourceProcessor(t *testing.T) {
	// This test now verifies the /uri/git/ path
	t.Log("--- Running TestResourceProcessor for /uri/git/ ---")
	// 1. Setup: Create a temporary directory for test data and start the server
	dataDir, err := ioutil.TempDir("", "potstack_test_data_rp_*")
	if err != nil {
		t.Fatalf("Failed to create temp data dir: %v", err)
	}
	defer os.RemoveAll(dataDir)
	t.Logf("Using temp data directory for ResourceProcessor test: %s", dataDir)

	// Set the repo root for the test
	config.RepoRoot = dataDir

	serverAddr := setupTestServer(t)
	apiURL := "http://" + serverAddr

	orgName := "test-org-rp"
	repoName := "test-repo-rp"
	fileName := "LATEST.txt"
	fileContent := "version 1.2.3"

	// 2. Create user and repo, and push the file to it.
	// Create user
	createUserURL := fmt.Sprintf("%s/api/v1/admin/users", apiURL)
	userReqBody, _ := json.Marshal(map[string]string{"username": orgName, "password": "password"})
	resp, err := http.Post(createUserURL, "application/json", bytes.NewBuffer(userReqBody))
	if err != nil || (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK) {
		t.Fatalf("Setup: Failed to create user for test. Status: %d, Err: %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	// Create repo
	createRepoURL := fmt.Sprintf("%s/api/v1/admin/users/%s/repos", apiURL, orgName)
	repoReqBody, _ := json.Marshal(map[string]string{"name": repoName})
	resp, err = http.Post(createRepoURL, "application/json", bytes.NewBuffer(repoReqBody))
	if err != nil || (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK) {
		t.Fatalf("Setup: Failed to create repo for test. Status: %d, Err: %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	// Clone, add file, commit, and push
	repoURL := fmt.Sprintf("%s/%s/%s.git", apiURL, orgName, repoName)
	cloneDir, err := ioutil.TempDir("", "clone_rp_*")
	if err != nil {
		t.Fatalf("Setup: Failed to create temp dir for clone: %v", err)
	}
	defer os.RemoveAll(cloneDir)

	r, err := git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
	})
	if err != nil {
		t.Fatalf("Setup: Failed to clone repo: %v", err)
	}

	filePath := filepath.Join(cloneDir, fileName)
	ioutil.WriteFile(filePath, []byte(fileContent), 0644)
	w, _ := r.Worktree()
	w.Add(fileName)
	w.Commit("feat: add LATEST.txt", &git.CommitOptions{Author: &object.Signature{Name: "RP Test", Email: "rp@test.com", When: time.Now()}})
	err = r.Push(&git.PushOptions{})
	if err != nil {
		t.Fatalf("Setup: Failed to push file to repo: %v", err)
	}
	t.Log("Setup for ResourceProcessor test completed successfully.")

	// 3. Test the ResourceProcessor endpoint for /uri/git/
	fileURL := fmt.Sprintf("%s/uri/git/%s/%s/%s", apiURL, orgName, repoName, fileName)
	t.Logf("Requesting file from URL: %s", fileURL)
	resp, err = http.Get(fileURL)
	if err != nil {
		t.Fatalf("Failed to make GET request to ResourceProcessor: %v", err)
	}
	defer resp.Body.Close()

	// 4. Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code 200 OK, but got %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != fileContent {
		t.Fatalf("Response body mismatch. Expected '%s', got '%s'", fileContent, string(body))
	}

	expectedContentType := "text/plain; charset=utf-8"
	if contentType := resp.Header.Get("Content-Type"); contentType != expectedContentType {
		t.Fatalf("Content-Type mismatch. Expected '%s', got '%s'", expectedContentType, contentType)
	}

	t.Log("SUCCESS: ResourceProcessor test for /uri/git/ passed.")
}

func TestDataResourceProcessor(t *testing.T) {
	t.Log("--- Running TestDataResourceProcessor for /uri/dat/ ---")
	// 1. Setup: Create a temporary directory for test data and start the server
	dataDir, err := ioutil.TempDir("", "potstack_test_data_drp_*")
	if err != nil {
		t.Fatalf("Failed to create temp data dir: %v", err)
	}
	defer os.RemoveAll(dataDir)
	t.Logf("Using temp data directory for DataResourceProcessor test: %s", dataDir)

	config.RepoRoot = dataDir
	serverAddr := setupTestServer(t)
	apiURL := "http://" + serverAddr

	orgName := "test-org-drp"
	repoName := "test-repo-drp"
	staticFileName := "static.txt"
	staticFileContent := "this is static content"

	// 2. Create user and repo (we don't need to push anything for this test)
	createUserURL := fmt.Sprintf("%s/api/v1/admin/users", apiURL)
	userReqBody, _ := json.Marshal(map[string]string{"username": orgName, "password": "password"})
	resp, err := http.Post(createUserURL, "application/json", bytes.NewBuffer(userReqBody))
	if err != nil || (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK) {
		t.Fatalf("Setup: Failed to create user for test. Status: %d, Err: %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	createRepoURL := fmt.Sprintf("%s/api/v1/admin/users/%s/repos", apiURL, orgName)
	repoReqBody, _ := json.Marshal(map[string]string{"name": repoName})
	resp, err = http.Post(createRepoURL, "application/json", bytes.NewBuffer(repoReqBody))
	if err != nil || (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK) {
		t.Fatalf("Setup: Failed to create repo for test. Status: %d, Err: %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	// 3. Manually create the 'data' directory and a static file within the bare repo
	repoGitDir := filepath.Join(dataDir, orgName, repoName+".git")
	dataDirInRepo := filepath.Join(repoGitDir, "data")
	if err := os.Mkdir(dataDirInRepo, 0755); err != nil {
		t.Fatalf("Setup: Failed to create 'data' directory in repo: %v", err)
	}
	staticFilePath := filepath.Join(dataDirInRepo, staticFileName)
	if err := ioutil.WriteFile(staticFilePath, []byte(staticFileContent), 0644); err != nil {
		t.Fatalf("Setup: Failed to write static file: %v", err)
	}
	t.Log("Setup for DataResourceProcessor test completed successfully.")

	// 4. Test the /uri/dat/ endpoint for a valid file
	fileURL := fmt.Sprintf("%s/uri/dat/%s/%s/%s", apiURL, orgName, repoName, staticFileName)
	t.Logf("Requesting static file from URL: %s", fileURL)
	resp, err = http.Get(fileURL)
	if err != nil {
		t.Fatalf("Failed to make GET request to /uri/dat/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code 200 OK, but got %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	if string(body) != staticFileContent {
		t.Fatalf("Response body mismatch. Expected '%s', got '%s'", staticFileContent, string(body))
	}
	t.Log("SUCCESS: /uri/dat/ test for valid file passed.")

	// 5. Test for path traversal attack
	attackURL := fmt.Sprintf("%s/uri/dat/%s/%s/../description", apiURL, orgName, repoName)
	t.Logf("Requesting malicious URL to test path traversal: %s", attackURL)
	resp, err = http.Get(attackURL)
	if err != nil {
		t.Fatalf("Failed to make GET request for path traversal test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("Path traversal test failed. Expected status code 403 Forbidden, but got %d", resp.StatusCode)
	}
	t.Log("SUCCESS: Path traversal attack was correctly blocked with a 403 Forbidden status.")
}

func TestCDNProcessor(t *testing.T) {
	t.Log("--- Running TestCDNProcessor for /cdn/ ---")
	// 1. Setup: Create a temporary directory for test data and start the server
	dataDir, err := ioutil.TempDir("", "potstack_test_data_cdn_*")
	if err != nil {
		t.Fatalf("Failed to create temp data dir: %v", err)
	}
	defer os.RemoveAll(dataDir)
	t.Logf("Using temp data directory for CDNProcessor test: %s", dataDir)

	config.RepoRoot = dataDir
	serverAddr := setupTestServer(t)
	apiURL := "http://" + serverAddr

	orgName := "biz.cdn"
	repoName := "test-cdn-repo"
	fileName := "style.css"
	fileContent := "body { color: blue; }"

	// 2. Create 'biz.cdn' user and a repo under it, then push the file.
	// Create user
	createUserURL := fmt.Sprintf("%s/api/v1/admin/users", apiURL)
	userReqBody, _ := json.Marshal(map[string]string{"username": orgName, "password": "password"})
	resp, err := http.Post(createUserURL, "application/json", bytes.NewBuffer(userReqBody))
	if err != nil || (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK) {
		t.Fatalf("Setup: Failed to create user 'biz.cdn' for test. Status: %d, Err: %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	// Create repo
	createRepoURL := fmt.Sprintf("%s/api/v1/admin/users/%s/repos", apiURL, orgName)
	repoReqBody, _ := json.Marshal(map[string]string{"name": repoName})
	resp, err = http.Post(createRepoURL, "application/json", bytes.NewBuffer(repoReqBody))
	if err != nil || (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK) {
		t.Fatalf("Setup: Failed to create CDN repo for test. Status: %d, Err: %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	// Clone, add file, commit, and push
	repoURL := fmt.Sprintf("%s/%s/%s.git", apiURL, orgName, repoName)
	cloneDir, err := ioutil.TempDir("", "clone_cdn_*")
	if err != nil {
		t.Fatalf("Setup: Failed to create temp dir for clone: %v", err)
	}
	defer os.RemoveAll(cloneDir)

	r, err := git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
	})
	if err != nil {
		t.Fatalf("Setup: Failed to clone CDN repo: %v", err)
	}

	filePath := filepath.Join(cloneDir, fileName)
	ioutil.WriteFile(filePath, []byte(fileContent), 0644)
	w, _ := r.Worktree()
	w.Add(fileName)
	w.Commit("feat: add style.css", &git.CommitOptions{Author: &object.Signature{Name: "CDN Test", Email: "cdn@test.com", When: time.Now()}})
	err = r.Push(&git.PushOptions{})
	if err != nil {
		t.Fatalf("Setup: Failed to push file to CDN repo: %v", err)
	}
	t.Log("Setup for CDNProcessor test completed successfully.")

	// 3. Test the CDN endpoint
	fileURL := fmt.Sprintf("%s/cdn/%s/%s", apiURL, repoName, fileName)
	t.Logf("Requesting file from CDN URL: %s", fileURL)
	resp, err = http.Get(fileURL)
	if err != nil {
		t.Fatalf("Failed to make GET request to CDNProcessor: %v", err)
	}
	defer resp.Body.Close()

	// 4. Verify the response
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code 200 OK, but got %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != fileContent {
		t.Fatalf("Response body mismatch. Expected '%s', got '%s'", fileContent, string(body))
	}

	expectedContentType := "text/css; charset=utf-8"
	if contentType := resp.Header.Get("Content-Type"); contentType != expectedContentType {
		t.Fatalf("Content-Type mismatch. Expected '%s', got '%s'", expectedContentType, contentType)
	}

	t.Log("SUCCESS: CDNProcessor test passed.")
}
