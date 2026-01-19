package loader

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"potstack/internal/docker"
	"potstack/internal/models"
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
	DataDir       string       // 数据目录 (用于查找公钥等)
	PublicKeyPath string       // 公钥文件路径（可选，优先级最高）
	HTTPClient    *http.Client // 自定义 HTTP 客户端（可选）
}

// Loader 预处理模块
type Loader struct {
	config      *Config
	client      *http.Client
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

	return l
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

	// 2. 读取 zip 数据
	// log.Printf("Reading PPK content, len: %d", header.ContentLen)
	content := make([]byte, header.ContentLen)
	if _, err := io.ReadFull(f, content); err != nil {
		return fmt.Errorf("failed to read ppk content: %w", err)
	}

	// 3. 验证签名与身份锁定 (TOFU + Pinning)
	// 解析出 Owner (这里先简单假设 owner 为 path 的第一级目录，实际应该解压后看)
	// 由于我们还未解压，无法确切知道 owner。但 PotPacker 打包规范通常是根目录下即为 owner 目录。
	// 不过，为了安全，我们最好先验证签名再解压。
	// 但是验证签名需要公钥。公钥在 Header 里。
	// 我们先用 Header 里的公钥验证签名（确保自洽）。
	if err := header.VerifySignature(content, ed25519.PublicKey(header.PublicKey[:])); err != nil {
		return fmt.Errorf("signature verification failed (self-check): %w", err)
	}

	// 临时解压以获取 Owner
	ppkTempDir := ppkPath + "_extracted"
	os.RemoveAll(ppkTempDir)
	defer os.RemoveAll(ppkTempDir)

	// Create reader for zip
	r, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return fmt.Errorf("failed to open zip reader: %w", err)
	}
	// Extract to temp to find owner
	if err := extractZip(r, ppkTempDir); err != nil {
		return fmt.Errorf("failed to extract zip content: %w", err)
	}

	// 扫描 Owner
	ownerEntries, err := os.ReadDir(ppkTempDir)
	if err != nil {
		return err
	}

	for _, ownerEntry := range ownerEntries {
		if !ownerEntry.IsDir() {
			continue
		}
		owner := ownerEntry.Name()

		// A. 获取或创建用户
		user, err := l.userService.GetUser(context.Background(), owner)
		if errors.Is(err, service.ErrUserNotFound) {
			// 自动创建用户 (TOFU User)
			user, err = l.userService.CreateUser(context.Background(), owner, owner+"@potstack.local", "")
			if err != nil {
				if errors.Is(err, service.ErrUserAlreadyExists) {
					user, err = l.userService.GetUser(context.Background(), owner)
				} else {
					return fmt.Errorf("failed to create user %s: %w", owner, err)
				}
			}
		}

		if err != nil {
			return fmt.Errorf("failed to get user %s: %w", owner, err)
		}

		// 公钥锁定校验
		headerPubKeyStr := fmt.Sprintf("%x", header.PublicKey) // 转为 hex 存储

		if user.PublicKey == "" {
			// TOFU: 首次信任，或者是老用户迁移
			log.Printf("TOFU: Trusting public key for owner %s", owner)
			if err := l.userService.SetUserPublicKey(context.Background(), owner, headerPubKeyStr); err != nil {
				return fmt.Errorf("failed to set public key for %s: %w", owner, err)
			}
		} else {
			// Pinning: 校验
			if user.PublicKey != headerPubKeyStr {
				return fmt.Errorf("SECURITY ERROR: Public key mismatch for owner %s! Expected: %s, Got: %s",
					owner, user.PublicKey, headerPubKeyStr)
			}
			log.Printf("Key pinning verified for owner %s", owner)
		}
	}

	// 4. 部署组件 (Deploy)

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

			// 检查并拉取 Docker 镜像（在推送代码前）
			potYmlPath := filepath.Join(potPath, "pot.yml")
			if potYmlData, err := os.ReadFile(potYmlPath); err == nil {
				var potCfg models.PotConfig
				if yaml.Unmarshal(potYmlData, &potCfg) == nil && potCfg.Docker != "" {
					localTag := fmt.Sprintf("potstack/%s/%s:latest", owner, potname)
					if !docker.ImageExists(localTag) {
						log.Printf("Pulling docker image: %s -> %s", potCfg.Docker, localTag)
						if err := docker.PullAndTag(potCfg.Docker, localTag); err != nil {
							return fmt.Errorf("failed to pull docker image for %s/%s: %w", owner, potname, err)
						}
					}
				}
			}

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
