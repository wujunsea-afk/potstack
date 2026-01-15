package loader

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"potstack/internal/service"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"gopkg.in/yaml.v3"
)

// Config Loader 配置
type Config struct {
	PotStackURL   string       // PotStack 服务地址
	Token         string       // 认证令牌
	BasePackPath  string       // 基础包路径（zip 文件）
	TempDir       string       // 临时目录
	PublicKeyPath string       // 公钥文件路径（可选，如果为 "" 则从 zip 包中 potstack_release.pub 读取）
	HTTPClient    *http.Client // 自定义 HTTP 客户端（可选）
}

// Loader 预处理模块
type Loader struct {
	config      *Config
	client      *http.Client
	pubKey      ed25519.PublicKey // 验证 PPK 签名用
	userService service.IUserService
	repoService service.IRepoService
}

// InstallManifest install.yml 结构
type InstallManifest struct {
	Version  string   `yaml:"version"`
	Packages []string `yaml:"packages"` // ppk 文件名列表
}

// New 创建 Loader 实例
func New(cfg *Config, us service.IUserService, rs service.IRepoService) *Loader {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// 安装自定义协议处理器以支持 InsecureSkipVerify
	customGitClient := githttp.NewClient(httpClient)
	client.InstallProtocol("http", customGitClient)
	client.InstallProtocol("https", customGitClient)

	l := &Loader{
		config:      cfg,
		client:      httpClient,
		userService: us,
		repoService: rs,
	}

	// 尝试加载公钥 (如果配置了路径)
	if cfg.PublicKeyPath != "" {
		if key, err := loadPublicKey(cfg.PublicKeyPath); err != nil {
			log.Printf("Warning: failed to load public key from %s: %v", cfg.PublicKeyPath, err)
		} else {
			l.pubKey = key
			log.Println("Loaded public key from config")
		}
	}

	return l
}

// loadPublicKey 读取 PEM 格式的 ED25519 公钥
func loadPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "POTPACKER PUBLIC KEY" {
		return nil, fmt.Errorf("invalid public key format")
	}

	if len(block.Bytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size")
	}

	return ed25519.PublicKey(block.Bytes), nil
}

// Initialize 初始化系统
func (l *Loader) Initialize() error {
	log.Println("Starting Loader initialization...")

	// 1. 检查 PotStack 服务是否可用
	if err := l.waitForService(); err != nil {
		return fmt.Errorf("service not available: %w", err)
	}

	// 2. 创建系统用户
	if err := l.createSystemUser(); err != nil {
		return fmt.Errorf("failed to create system user: %w", err)
	}

	// 3. 创建系统仓库
	if err := l.createSystemRepos(); err != nil {
		return fmt.Errorf("failed to create system repos: %w", err)
	}

	// 4. 解压并推送组件
	if l.config.BasePackPath != "" {
		if err := l.deployComponents(); err != nil {
			return fmt.Errorf("failed to deploy components: %w", err)
		}
	}

	log.Println("Loader initialization completed!")
	return nil
}

// waitForService 等待 PotStack 服务可用
func (l *Loader) waitForService() error {
	log.Println("Waiting for PotStack service...")

	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := l.client.Get(l.config.PotStackURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Println("PotStack service is ready")
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("service not available after %d retries", maxRetries)
}

// createSystemUser 创建系统用户
func (l *Loader) createSystemUser() error {
	log.Println("Creating system user: potstack")

	_, err := l.userService.CreateUser(context.Background(), "potstack", "system@potstack.local", "")
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			log.Println("System user already exists")
			return nil
		}
		return err
	}

	log.Println("System user created")
	return nil
}

// createSystemRepos 创建系统仓库
func (l *Loader) createSystemRepos() error {
	repos := []string{"keeper", "loader", "repo"}

	for _, name := range repos {
		log.Printf("Creating system repo: potstack/%s", name)

		_, err := l.repoService.CreateRepo(context.Background(), "potstack", name)
		if err != nil {
			if errors.Is(err, service.ErrRepoAlreadyExists) {
				log.Printf("Repo potstack/%s already exists", name)
				continue
			}
			return err
		}

		log.Printf("Repo potstack/%s created", name)
	}

	return nil
}

// ... deployComponents and other methods remain ...

// ensureUserAndRepo 确保用户和仓库存在
func (l *Loader) ensureUserAndRepo(owner, repo string) {
	// 创建用户（忽略错误）
	l.userService.CreateUser(context.Background(), owner, owner+"@potstack.local", "")

	// 创建仓库（忽略错误）
	l.repoService.CreateRepo(context.Background(), owner, repo)
}

// deployComponents 解压并推送组件
func (l *Loader) deployComponents() error {
	log.Printf("Deploying components from: %s", l.config.BasePackPath)

	// 创建临时目录
	tempDir := l.config.TempDir
	if tempDir == "" {
		tempDir = filepath.Join(os.TempDir(), "potstack-loader")
	}
	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// 1. 解压 potstack-base.zip
	if err := l.unzip(l.config.BasePackPath, tempDir); err != nil {
		return fmt.Errorf("failed to unzip base pack: %w", err)
	}

	// 1.5 尝试从 base pack 加载公钥 (如果尚未加载)
	if len(l.pubKey) == 0 {
		// 尝试常见的文件名
		candidates := []string{"potstack_release.pub", "release.pub"}
		for _, name := range candidates {
			keyPath := filepath.Join(tempDir, name)
			if key, err := loadPublicKey(keyPath); err == nil {
				l.pubKey = key
				log.Printf("Loaded public key from base pack: %s", name)
				break
			}
		}
	}

	// 2. 读取 install.yml
	manifest, err := l.loadInstallManifest(filepath.Join(tempDir, "install.yml"))
	if err != nil {
		return fmt.Errorf("failed to load install.yml: %w", err)
	}

	log.Printf("Install manifest version: %s, packages: %d", manifest.Version, len(manifest.Packages))

	// 3. 处理每个 ppk 包
	for _, ppkFile := range manifest.Packages {
		ppkPath := filepath.Join(tempDir, ppkFile)
		if err := l.deployPPK(ppkPath); err != nil {
			log.Printf("Warning: failed to deploy %s: %v", ppkFile, err)
		}
	}

	return nil
}

// loadInstallManifest 加载 install.yml
func (l *Loader) loadInstallManifest(path string) (*InstallManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest InstallManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// deployPPK 解压并部署单个 ppk 包
func (l *Loader) deployPPK(ppkPath string) error {
	log.Printf("Deploying PPK: %s", ppkPath)

	f, err := os.Open(ppkPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// 1. 解析 Header
	header, err := ParsePpkHeader(f)
	if err != nil {
		return fmt.Errorf("invalid ppk header: %w", err)
	}

	// 2. 读取 7z 数据
	// log.Printf("Reading PPK content, len: %d", header.ContentLen)
	content := make([]byte, header.ContentLen)
	if _, err := io.ReadFull(f, content); err != nil {
		return fmt.Errorf("failed to read ppk content: %w", err)
	}

	// 3. 验证签名
	if len(l.pubKey) > 0 {
		if err := header.VerifySignature(content, l.pubKey); err != nil {
			return fmt.Errorf("signature verification failed: %w", err)
		}
		log.Println("Signature verified successfully")
	} else {
		log.Println("Warning: No public key loaded, skipping signature verification")
	}

	// 4. 解压 Zip 到临时目录
	ppkTempDir := ppkPath + "_extracted"
	os.RemoveAll(ppkTempDir)
	if err := os.MkdirAll(ppkTempDir, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(ppkTempDir)

	r, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return fmt.Errorf("failed to open zip reader: %w", err)
	}

	if err := extractZip(r, ppkTempDir); err != nil {
		return fmt.Errorf("failed to extract zip content: %w", err)
	}

	// 5. 遍历 owner 目录并推送
	ownerEntries, err := os.ReadDir(ppkTempDir)
	if err != nil {
		return err
	}

	for _, ownerEntry := range ownerEntries {
		if !ownerEntry.IsDir() {
			continue
		}
		owner := ownerEntry.Name()
		ownerPath := filepath.Join(ppkTempDir, owner)

		// 遍历 potname 目录
		potEntries, err := os.ReadDir(ownerPath)
		if err != nil {
			continue
		}

		for _, potEntry := range potEntries {
			if !potEntry.IsDir() {
				continue
			}
			potname := potEntry.Name()
			potPath := filepath.Join(ownerPath, potname)

			// 确保用户和仓库存在
			l.ensureUserAndRepo(owner, potname)

			// 推送到仓库
			if err := l.pushToRepo(owner, potname, potPath); err != nil {
				log.Printf("Warning: failed to push %s/%s: %v", owner, potname, err)
			}
		}
	}

	return nil
}

// extractZip 解压 Zip 数据
func extractZip(r *zip.Reader, dest string) error {
	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)

		// 安全检查
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// unzip 解压 zip 文件
func (l *Loader) unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)

		// 安全检查
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0755) // 使用固定权限，避免从 zip 继承的无效权限
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// pushToRepo 推送目录内容到本地裸仓库
func (l *Loader) pushToRepo(owner, repo, dir string) error {
	// 获取本地裸仓库路径
	bareRepoPath := l.repoService.GetRepoPath(owner, repo)
	log.Printf("Pushing %s to %s", dir, bareRepoPath)

	// 检查裸仓库是否存在
	if _, err := os.Stat(bareRepoPath); os.IsNotExist(err) {
		return fmt.Errorf("bare repo does not exist: %s", bareRepoPath)
	}

	// 1. 打开或初始化本地仓库
	r, err := git.PlainOpen(dir)
	if err != nil {
		log.Printf("Dir %s is not a git repo, initializing...", dir)
		r, err = git.PlainInit(dir, false)
		if err != nil {
			return fmt.Errorf("failed to init repo: %w", err)
		}

		// 默认 go-git 使用 master，强制切换到 main 以匹配服务端
		headRef := plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main")
		if err := r.Storer.SetReference(headRef); err != nil {
			return fmt.Errorf("failed to set HEAD to main: %w", err)
		}
	}

	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// 2. 添加所有文件
	if _, err := w.Add("."); err != nil {
		return fmt.Errorf("failed to add files: %w", err)
	}

	// 3. 提交
	hash, err := w.Commit("Initial commit by Loader", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "potstack-loader",
			Email: "loader@potstack.local",
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Printf("Commit result for %s/%s: %v", owner, repo, err)
	} else {
		log.Printf("Committed %s/%s: %s", owner, repo, hash.String())
	}

	// 4. 配置远程指向本地裸仓库（如果已存在则删除重建）
	_ = r.DeleteRemote("origin")
	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{bareRepoPath},
	})
	if err != nil {
		return fmt.Errorf("failed to create remote: %w", err)
	}
	log.Printf("Remote origin set to: %s", bareRepoPath)

	// 5. 推送到本地裸仓库
	err = r.Push(&git.PushOptions{
		RemoteName: "origin",
		Force:      true,
	})
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			log.Printf("Repo %s/%s already up to date", owner, repo)
			return nil
		}
		log.Printf("Push failed for %s/%s: %v", owner, repo, err)
		return fmt.Errorf("failed to push: %w", err)
	}

	log.Printf("Pushed %s/%s successfully", owner, repo)
	return nil
}
