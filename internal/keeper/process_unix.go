//go:build !windows

package keeper

import (
	"os/exec"
	"syscall"
)

// JobCmd wraps exec.Cmd to ensure it runs with parent-death signal
type JobCmd struct {
	*exec.Cmd
}

func NewJobCmd(name string, arg ...string) *JobCmd {
	return &JobCmd{
		Cmd: exec.Command(name, arg...),
	}
}

func (j *JobCmd) Start() error {
	if j.Cmd.SysProcAttr == nil {
		j.Cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	
	// Linux specific: Ensure child receives SIGKILL when parent dies
	// This mimics Windows Job Object "KILL_ON_JOB_CLOSE" behavior
	j.Cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	
	// Create new Process Group (useful if we want to kill group manually later)
	j.Cmd.SysProcAttr.Setpgid = true

	return j.Cmd.Start()
}
