package models

// PotConfig represents the structure of pot.yml
type PotConfig struct {
	Title   string   `yaml:"title"`
	Version string   `yaml:"version"`
	Owner   string   `yaml:"owner"`
	PotName string   `yaml:"potname"`
	Type    string   `yaml:"type"`           // "exe" or "static"
	Root    string   `yaml:"root,omitempty"` // static 类型专用
	Env     []EnvVar `yaml:"env,omitempty"`  // exe 类型专用
}

// EnvVar definition
type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
	Tips  string `yaml:"tips,omitempty"`
}

// RunStatus defines the desired state of a sandbox
type RunStatus string

const (
	RunStatusRunning RunStatus = "running"
	RunStatusStopped RunStatus = "stopped"
)

// RunConfig represents the runtime state in run.yml
type RunConfig struct {
	TargetStatus RunStatus `yaml:"target_status"`
	Runtime      struct {
		Pid       int    `yaml:"pid"`
		Port      int    `yaml:"port"`
		StartTime string `yaml:"start_time"`
	} `yaml:"runtime"`
}
