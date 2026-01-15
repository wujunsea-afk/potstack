package keeper

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"potstack/internal/models"
	"potstack/internal/router"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
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
	fmt.Println("Keeper started. Monitoring sandboxes...")

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
			fmt.Println("Keeper stopped.")
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
		// Always update routes for installed sandboxes
		s.Router.UpdateStaticRoutes(sb.Org, sb.Name)

		run, err := s.loadRunConfig(sb.Org, sb.Name)
		if err != nil {
			// run.yml missing. Initialize runtime.
			fmt.Printf("Initializing sandbox %s/%s\n", sb.Org, sb.Name)
			if err := s.createRuntime(sb.Org, sb.Name); err != nil {
				fmt.Printf("Failed to create runtime: %v\n", err)
				continue
			}
			s.Start(sb.Org, sb.Name)
			continue
		}

		if run.TargetStatus == models.RunStatusRunning {
			s.mu.RLock()
			_, running := s.runningInstances[fmt.Sprintf("%s/%s", sb.Org, sb.Name)]
			s.mu.RUnlock()

			if !running {
				s.Start(sb.Org, sb.Name)
			} else {
				// If running, ensure EXE routes are up-to-date
				s.Router.UpdateExeRoutes(sb.Org, sb.Name)
			}
		} else {
			// If stopped, ensure we fallback to static only
			s.Router.UpdateStaticRoutes(sb.Org, sb.Name)
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
	_, err := git.PlainClone(programDir, false, &git.CloneOptions{
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

// SignalUpdate is called by Loader
func (s *SandboxManager) SignalUpdate(org, name string) {
	fmt.Printf("Received update signal for %s/%s\n", org, name)

	// Update Runtime code
	if err := s.createRuntime(org, name); err != nil {
		fmt.Printf("Failed to update runtime: %v\n", err)
		return
	}

	// Ensure routes updated
	s.Router.UpdateStaticRoutes(org, name)

	// Restart if running
	s.Stop(org, name)
	s.Start(org, name)
}

// Start launches the sandbox (exe type only)
func (s *SandboxManager) Start(org, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s/%s", org, name)

	// 1. Path Calculation
	bareRepoPath := filepath.Join(s.RepoRoot, org, fmt.Sprintf("%s.git", name))
	sandboxRoot := filepath.Join(bareRepoPath, "data", "faaspot")
	programDir := filepath.Join(sandboxRoot, "program")

	// 2. Parse pot.yml
	configFile := filepath.Join(programDir, "pot.yml")
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("config not found: %w", err)
	}

	var potCfg models.PotConfig
	if err := yaml.Unmarshal(data, &potCfg); err != nil {
		return fmt.Errorf("failed to parse pot.yml: %w", err)
	}

	// Only exe type needs to start a process
	if potCfg.Type != "exe" {
		// For static type, just update routes
		s.Router.UpdateStaticRoutes(org, name)
		return nil
	}

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
	if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
		return fmt.Errorf("pot.exe not found at %s", cmdPath)
	}

	jobCmd := NewJobCmd(cmdPath)
	jobCmd.Dir = programDir

	// Env
	env := os.Environ()
	env = append(env, fmt.Sprintf("SU_SERVER_ADDR=%s", addr))
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

	// 7. Register Router (EXE Routes)
	s.Router.UpdateExeRoutes(org, name)

	s.runningInstances[key] = &Instance{
		Org:  org,
		Name: name,
		Cmd:  jobCmd,
	}
	fmt.Printf("Started sandbox %s (port %d)\n", key, port)

	// Monitor death for restart
	go s.watchProcess(key, jobCmd)

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

	// Downgrade to Static only
	s.Router.UpdateStaticRoutes(org, name)

	fmt.Printf("Stopped sandbox %s\n", key)
	return nil
}

func (s *SandboxManager) watchProcess(key string, cmd *JobCmd) {
	state, err := cmd.Process.Wait()
	fmt.Printf("Sandbox %s exited: %v %v\n", key, state, err)

	s.mu.Lock()
	delete(s.runningInstances, key)
	s.mu.Unlock()

	// Check if we should restart
	parts := strings.Split(key, "/")
	if len(parts) >= 2 {
		org, name := parts[0], parts[1]
		rc, _ := s.loadRunConfig(org, name)
		if rc != nil && rc.TargetStatus == models.RunStatusRunning {
			fmt.Printf("Auto-restarting %s...\n", key)
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
