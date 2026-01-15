package resource

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"potstack/config"

	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// serveRepoFile is a helper function that opens a git repository, finds a file, and serves it.
func serveRepoFile(c *gin.Context, repoPath, filePathInRepo string) {
	// 1. Open the bare repository
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			log.Printf("Repository not found at path: %s", repoPath)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		log.Printf("Error opening repository at %s: %v", repoPath, err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// 2. Get the HEAD reference
	headRef, err := r.Head()
	if err != nil {
		log.Printf("Error getting HEAD for repo %s: %v", repoPath, err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// 3. Get the commit object from HEAD
	commit, err := r.CommitObject(headRef.Hash())
	if err != nil {
		log.Printf("Error getting commit object for repo %s: %v", repoPath, err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// 4. Get the tree from the commit
	tree, err := commit.Tree()
	if err != nil {
		log.Printf("Error getting tree from commit for repo %s: %v", repoPath, err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// 5. Find the file in the tree
	file, err := tree.File(filePathInRepo)
	if err != nil {
		if err == object.ErrFileNotFound {
			log.Printf("File '%s' not found in repo '%s'", filePathInRepo, repoPath)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		log.Printf("Error finding file '%s' in repo '%s': %v", filePathInRepo, repoPath, err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// 6. Get the file's blob reader
	reader, err := file.Reader()
	if err != nil {
		log.Printf("Error getting reader for file '%s' in repo '%s': %v", filePathInRepo, repoPath, err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	// 7. Serve the file content
	// Set content type based on file extension
	contentType := mime.TypeByExtension(filepath.Ext(file.Name))
	if contentType == "" {
		contentType = "application/octet-stream" // Default content type
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", file.Size))
	c.Status(http.StatusOK)
	io.Copy(c.Writer, reader)
}

// ResourceProcessor handles /uri requests by serving files from a specified git repository
// or a direct data directory within it, based on the path prefix.
func ResourceProcessor() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := strings.TrimPrefix(c.Param("path"), "/")

		if strings.HasPrefix(path, "git/") {
			// Handles /uri/git/<owner>/<repo>/<file-path>
			// Serves the file from the git history.
			gitPath := strings.TrimPrefix(path, "git/")
			parts := strings.SplitN(gitPath, "/", 3)

			if len(parts) < 3 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path format for /uri/git/, expected /git/<owner>/<repo>/<file-path>"})
				return
			}

			owner := parts[0]
			repoName := parts[1]
			filePathInRepo := parts[2]
			repoPath := filepath.Join(config.RepoDir, owner, repoName+".git")

			serveRepoFile(c, repoPath, filePathInRepo)

		} else if strings.HasPrefix(path, "dat/") {
			// Handles /uri/dat/<owner>/<repo>/<file-path>
			// Serves the file directly from the repository's 'data' subdirectory.
			datPath := strings.TrimPrefix(path, "dat/")
			parts := strings.SplitN(datPath, "/", 3)

			if len(parts) < 3 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path format for /uri/dat/, expected /dat/<owner>/<repo>/<file-path>"})
				return
			}

			owner := parts[0]
			repoName := parts[1]
			filePathInDataDir := parts[2]

			// Base path of the 'data' directory inside the bare repo.
			dataRoot := filepath.Join(config.RepoDir, owner, repoName+".git", "data")

			// Security: Join and clean the path to prevent path traversal attacks.
			fullPath := filepath.Join(dataRoot, filePathInDataDir)
			cleanedPath := filepath.Clean(fullPath)

			// Security check: Ensure the cleaned path is still within the intended dataRoot.
			if !strings.HasPrefix(cleanedPath, dataRoot) {
				log.Printf("Path traversal attempt blocked. Original path: %s, Cleaned path: %s", fullPath, cleanedPath)
				c.AbortWithStatus(http.StatusForbidden)
				return
			}

			// Serve the static file. http.ServeFile handles existence checks.
			http.ServeFile(c.Writer, c.Request, cleanedPath)

		} else {
			// Path prefix is invalid.
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path prefix, must start with /git/ or /dat/"})
		}
	}
}

// CDNProcessor handles /cdn requests mapping to biz.cdn repo.
func CDNProcessor() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Example: /cdn/myrepo/main.js
		// c.Param("path") will be "/myrepo/main.js"
		path := strings.TrimPrefix(c.Param("path"), "/")
		parts := strings.SplitN(path, "/", 2)

		if len(parts) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path format, expected /<repo>/<file-path>"})
			return
		}

		owner := "biz.cdn"
		repoName := parts[0]
		filePathInRepo := parts[1]
		repoPath := filepath.Join(config.RepoDir, owner, repoName+".git")

		serveRepoFile(c, repoPath, filePathInRepo)
	}
}

// WebProcessor handles /web redirection/proxying to sandbox
func WebProcessor() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Placeholder for sandbox web path mapping
		c.JSON(http.StatusOK, gin.H{"message": "Web path mapping not yet implemented"})
	}
}

// ATTProcessor handles ATT operations
func ATTProcessor() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Placeholder for ATT operation handler
		c.JSON(http.StatusOK, gin.H{"message": "ATT operation not yet implemented"})
	}
}

// NewStaticHandler 返回一个 http.Handler，从指定仓库的 root 目录下服务静态文件
// 直接从 Git 仓库的 HEAD commit 读取文件
func NewStaticHandler(repoRoot, org, name, root string) http.Handler {
	repoPath := filepath.Join(repoRoot, org, fmt.Sprintf("%s.git", name))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 获取请求的文件路径，拼接 root 前缀
		reqPath := strings.TrimPrefix(r.URL.Path, "/")
		filePathInRepo := filepath.Join(root, reqPath)

		// 使用 go-git 从仓库读取文件
		serveRepoFileHTTP(w, r, repoPath, filePathInRepo)
	})
}

// serveRepoFileHTTP 是 serveRepoFile 的 http.Handler 版本
func serveRepoFileHTTP(w http.ResponseWriter, r *http.Request, repoPath, filePathInRepo string) {
	// 1. Open the bare repository
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 2. Get HEAD reference
	headRef, err := repo.Head()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 3. Get commit
	commit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 4. Get tree
	tree, err := commit.Tree()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 5. Find file
	file, err := tree.File(filePathInRepo)
	if err != nil {
		if err == object.ErrFileNotFound {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 6. Get reader
	reader, err := file.Reader()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	// 7. Serve content
	contentType := mime.TypeByExtension(filepath.Ext(file.Name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))
	io.Copy(w, reader)
}

