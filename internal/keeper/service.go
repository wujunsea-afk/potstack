package keeper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"potstack/config"
	"potstack/internal/git"
	"potstack/internal/models"
	"potstack/internal/router"

	gitlib "github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"
)

// PotProvider defines interface to get installed pots
type PotProvider interface {
	GetInstalledPots() []PotURI
}

// PotURI represents a unique pot identifier
type PotURI struct {
	Org  string
	Name string
}

type SandboxManager struct {
	RepoRoot    string
	PotProvider PotProvider
	Router      *router.Router

	// Key: org/repo
	runningInstances map[string]*Instance
	mu               sync.RWMutex
	stopChan         chan struct{}
}

func NewManager(repoRoot string, r *router.Router) *SandboxManager {
	return &SandboxManager{
		RepoRoot:         repoRoot,
		Router:           r,
		runningInstances: make(map[string]*Instance),
		stopChan:         make(chan struct{}),
	}
}

func (s *SandboxManager) SetPotProvider(p PotProvider) {
	s.PotProvider = p
}

// StartKeeper is the main loop
func (s *SandboxManager) StartKeeper() {
	log.Println("Keeper started. Monitoring sandboxes...")

	// Initial Scan
	s.reconcile()

	// Monitor Loop
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.monitor()
		case <-s.stopChan:
			log.Println("Keeper stopped.")
			return
		}
	}
}

// reconcile ensures all sandboxes are in desired state
func (s *SandboxManager) reconcile() {
	if s.PotProvider == nil {
		return
	}

	list := s.PotProvider.GetInstalledPots()
	for _, sb := range list {
		// 1. 从 Git 读取 pot.yml
		var potCfg models.PotConfig
		if err := git.ReadPotYml(s.RepoRoot, sb.Org, sb.Name, &potCfg); err != nil {
			continue // 没有 pot.yml，跳过
		}

		// 2. 根据类型处理
		if potCfg.Type == "static" {
			// Static 类型：直接刷新路由即可
			s.refreshRoute(sb.Org, sb.Name)
			continue
		}

		if potCfg.Type == "exe" {
			// Exe 类型：需要管理进程
			run, err := s.loadRunConfig(sb.Org, sb.Name)
			if err != nil {
				// 初始化运行时
				log.Printf("Initializing sandbox %s/%s", sb.Org, sb.Name)
				if err := s.createRuntime(sb.Org, sb.Name); err != nil {
					log.Printf("Failed to create runtime: %v", err)
					continue
				}
				if err := s.Start(sb.Org, sb.Name); err != nil {
					log.Printf("Failed to start sandbox %s/%s: %v", sb.Org, sb.Name, err)
				}
				continue
			}

			// 根据 TargetStatus 处理
			if run.TargetStatus == models.RunStatusRunning {
				s.mu.RLock()
				_, running := s.runningInstances[fmt.Sprintf("%s/%s", sb.Org, sb.Name)]
				s.mu.RUnlock()

				if !running {
					if err := s.Start(sb.Org, sb.Name); err != nil {
						log.Printf("Failed to start sandbox %s/%s: %v", sb.Org, sb.Name, err)
					}
				} else {
					// 已经在运行，确保路由是最新的
					s.refreshRoute(sb.Org, sb.Name)
				}
			} else {
				// TargetStatus 是 stopped，确保进程已停止
				s.mu.RLock()
				_, running := s.runningInstances[fmt.Sprintf("%s/%s", sb.Org, sb.Name)]
				s.mu.RUnlock()

				if running {
					s.Stop(sb.Org, sb.Name) // 内部会调用 refreshRoute
				}
			}
		}
	}
}

// createRuntime prepares the sandbox environment (Clones from bare repo)
func (s *SandboxManager) createRuntime(org, name string) error {
	bareRepoPath := filepath.Join(s.RepoRoot, org, fmt.Sprintf("%s.git", name))

	sandboxRoot := filepath.Join(bareRepoPath, "data", "faaspot")
	programDir := filepath.Join(sandboxRoot, "program")
	dataDir := filepath.Join(sandboxRoot, "data")
	logDir := filepath.Join(sandboxRoot, "log")

	// Verify bare repo exists
	if _, err := os.Stat(bareRepoPath); os.IsNotExist(err) {
		return fmt.Errorf("git repo does not exist at %s", bareRepoPath)
	}

	// Create directories
	dirs := []string{dataDir, logDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create dir %s: %w", d, err)
		}
	}

	// Clean program dir
	os.RemoveAll(programDir)

	// Clone to programDir
	_, err := gitlib.PlainClone(programDir, false, &gitlib.CloneOptions{
		URL: bareRepoPath,
	})
	if err != nil {
		return fmt.Errorf("failed to clone code to sandbox: %w", err)
	}

	return nil
}

// GetSandboxConfig reads pot.yml from an installed sandbox
func (s *SandboxManager) GetSandboxConfig(org, name string) (*models.PotConfig, error) {
	bareRepoPath := filepath.Join(s.RepoRoot, org, fmt.Sprintf("%s.git", name))
	configFile := filepath.Join(bareRepoPath, "data", "faaspot", "program", "pot.yml")

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	var pct models.PotConfig
	if err := yaml.Unmarshal(data, &pct); err != nil {
		return nil, err
	}
	return &pct, nil
}

// monitor checks process health
func (s *SandboxManager) monitor() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, inst := range s.runningInstances {
		if inst.Cmd != nil && inst.Cmd.Process != nil {
			// Monitoring logic is handled by watchProcess mostly.
		}
	}
}

// refreshRoute 调用 Router 刷新接口更新路由
func (s *SandboxManager) refreshRoute(org, name string) {
	url := fmt.Sprintf("http://localhost:%s/pot/potstack/router/refresh", config.InternalPort)

	payload := map[string]string{"org": org, "name": name}
	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to refresh route for %s/%s: %v", org, name, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Refresh route failed for %s/%s: status %d, body: %s", org, name, resp.StatusCode, body)
	} else {
		log.Printf("Route refreshed for %s/%s", org, name)
	}
}

// SignalUpdate is called by Loader
func (s *SandboxManager) SignalUpdate(org, name string) {
	log.Printf("Received update signal for %s/%s", org, name)

	// Update Runtime code
	if err := s.createRuntime(org, name); err != nil {
		log.Printf("Failed to update runtime: %v", err)
		return
	}

	// Restart if running
	s.Stop(org, name)
	s.Start(org, name)
}

// Start launches the sandbox (exe type only)
func (s *SandboxManager) Start(org, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s/%s", org, name)

	// 1. 从 Git 读取 pot.yml 判断类型
	var potCfg models.PotConfig
	if err := git.ReadPotYml(s.RepoRoot, org, name, &potCfg); err != nil {
		return fmt.Errorf("pot.yml not found: %w", err)
	}

	// Only exe type needs to start a process
	if potCfg.Type != "exe" {
		return fmt.Errorf("not an exe type sandbox")
	}

	// 2. Path Calculation
	bareRepoPath := filepath.Join(s.RepoRoot, org, fmt.Sprintf("%s.git", name))
	sandboxRoot := filepath.Join(bareRepoPath, "data", "faaspot")
	programDir := filepath.Join(sandboxRoot, "program")

	// 3. Prepare Run Config
	rc := models.RunConfig{
		TargetStatus: models.RunStatusRunning,
	}
	rc.Runtime.StartTime = time.Now().Format(time.RFC3339)

	// 4. Get port
	var port int
	var addr string

	// Check env for SU_SERVER_ADDR
	customAddr := ""
	for _, e := range potCfg.Env {
		if e.Name == "SU_SERVER_ADDR" {
			customAddr = e.Value
			break
		}
	}

	if customAddr != "" {
		addr = customAddr
		_, portStr, err := net.SplitHostPort(addr)
		if err == nil {
			fmt.Sscanf(portStr, "%d", &port)
		}
	} else {
		// Random Port
		p, err := GetFreePort()
		if err != nil {
			return err
		}
		port = p
		addr = fmt.Sprintf("127.0.0.1:%d", port)
	}

	rc.Runtime.Port = port

	// 5. Launch pot.exe
	cmdPath := filepath.Join(programDir, "pot.exe")
	// 转换为绝对路径
	absCmdPath, err := filepath.Abs(cmdPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmdPath = absCmdPath

	if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
		return fmt.Errorf("pot.exe not found at %s", cmdPath)
	}

	jobCmd := NewJobCmd(cmdPath)
	jobCmd.Dir = programDir

	// Env
	env := os.Environ()
	// 内置环境变量
	dataPath := filepath.Join(sandboxRoot, "data")
	env = append(env, fmt.Sprintf("DATA_PATH=%s", dataPath))
	env = append(env, fmt.Sprintf("PROGRAM_PATH=%s", programDir))
	env = append(env, fmt.Sprintf("LOG_PATH=%s", filepath.Join(sandboxRoot, "log")))
	env = append(env, fmt.Sprintf("POTSTACK_BASE_URL=http://localhost:%s", config.InternalPort))
	env = append(env, fmt.Sprintf("SU_SERVER_ADDR=%s", addr))
	// 用户自定义环境变量
	for _, e := range potCfg.Env {
		env = append(env, fmt.Sprintf("%s=%s", e.Name, e.Value))
	}
	jobCmd.Env = env

	if err := jobCmd.Start(); err != nil {
		return fmt.Errorf("failed to start pot.exe: %w", err)
	}

	rc.Runtime.Pid = jobCmd.Process.Pid

	// 6. Save Run Config
	s.saveRunConfig(org, name, &rc)



	s.runningInstances[key] = &Instance{
		Org:  org,
		Name: name,
		Cmd:  jobCmd,
	}
	log.Printf("Started sandbox %s (port %d)", key, port)

	// Monitor death for restart
	go s.watchProcess(key, jobCmd)

	// 解锁后刷新路由
	s.mu.Unlock()
	s.refreshRoute(org, name)
	s.mu.Lock() // 重新加锁以配合 defer Unlock

	return nil
}

func (s *SandboxManager) Stop(org, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s/%s", org, name)

	// Kill Process
	if inst, ok := s.runningInstances[key]; ok {
		if inst.Cmd != nil && inst.Cmd.Process != nil {
			inst.Cmd.Process.Kill()
		}
		delete(s.runningInstances, key)
	}

	// Update Status
	rc, _ := s.loadRunConfig(org, name)
	if rc == nil {
		rc = &models.RunConfig{}
	}
	rc.TargetStatus = models.RunStatusStopped
	s.saveRunConfig(org, name, rc)



	// 解锁后刷新路由
	s.mu.Unlock()
	s.refreshRoute(org, name)
	s.mu.Lock() // 重新加锁以配合 defer Unlock

	log.Printf("Stopped sandbox %s", key)
	return nil
}

func (s *SandboxManager) watchProcess(key string, cmd *JobCmd) {
	state, err := cmd.Process.Wait()
	log.Printf("Sandbox %s exited: %v %v", key, state, err)

	s.mu.Lock()
	delete(s.runningInstances, key)
	s.mu.Unlock()

	// Check if we should restart
	parts := strings.Split(key, "/")
	if len(parts) >= 2 {
		org, name := parts[0], parts[1]
		rc, _ := s.loadRunConfig(org, name)
		if rc != nil && rc.TargetStatus == models.RunStatusRunning {
			log.Printf("Auto-restarting %s...", key)
			time.Sleep(1 * time.Second) // backoff
			s.Start(org, name)
		}
	}
}

func (s *SandboxManager) loadRunConfig(org, name string) (*models.RunConfig, error) {
	runFile := filepath.Join(s.RepoRoot, org, fmt.Sprintf("%s.git", name), "data", "faaspot", "run.yml")
	data, err := os.ReadFile(runFile)
	if err != nil {
		return nil, err
	}

	var rc models.RunConfig
	if err := yaml.Unmarshal(data, &rc); err != nil {
		return nil, err
	}
	return &rc, nil
}

func (s *SandboxManager) saveRunConfig(org, name string, rc *models.RunConfig) error {
	runFile := filepath.Join(s.RepoRoot, org, fmt.Sprintf("%s.git", name), "data", "faaspot", "run.yml")
	data, err := yaml.Marshal(rc)
	if err != nil {
		return err
	}
	return os.WriteFile(runFile, data, 0644)
}

func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
