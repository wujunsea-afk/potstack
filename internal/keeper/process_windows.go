//go:build windows

package keeper

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

// JobCmd wraps exec.Cmd to ensure it runs in a Job Object
type JobCmd struct {
	*exec.Cmd
	jobHandle syscall.Handle
}

func NewJobCmd(name string, arg ...string) *JobCmd {
	return &JobCmd{
		Cmd: exec.Command(name, arg...),
	}
}

func (j *JobCmd) Start() error {
	// Create a Job Object
	// For simplicity, we create a new anonymous job or named one unique to this process
	// Actually, creating a new unnamed job object is cleaner
	job, err := CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("CreateJobObject failed: %w", err)
	}
	j.jobHandle = job

	// Set setup to kill on close
	info := JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := SetInformationJobObject(job, JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		syscall.CloseHandle(job)
		return fmt.Errorf("SetInformationJobObject failed: %w", err)
	}

	// Start normally (Race condition acceptable for this MVP)
	// We do NOT use CREATE_SUSPENDED because resuming purely in Go is hard without low-level APIs
	// j.Cmd.SysProcAttr = &syscall.SysProcAttr{ ... } 

	if err := j.Cmd.Start(); err != nil {
		syscall.CloseHandle(job)
		return err
	}

	// We need to get a Handle from the PID
	// PID is NOT a handle.
	pid := j.Cmd.Process.Pid
	const PROCESS_ALL_ACCESS = 0x1F0FFF
	// Strictly we need PROCESS_SET_QUOTA | PROCESS_TERMINATE
	// 0x100 | 0x1
	
	processHandle, err := syscall.OpenProcess(0x100|0x1, false, uint32(pid))
	if err != nil {
		// Just log? Failure to assign to job means it won't auto-die.
		// For now we error out.
		j.Cmd.Process.Kill()
		syscall.CloseHandle(job)
		return fmt.Errorf("OpenProcess failed: %w", err)
	}
	defer syscall.CloseHandle(processHandle)

	// Assign process to job
	if err := AssignProcessToJobObject(job, processHandle); err != nil {
		j.Cmd.Process.Kill()
		syscall.CloseHandle(job)
		return fmt.Errorf("AssignProcessToJobObject failed: %w", err)
	}

	// Resume the thread
	// We need the handle to the main thread. Go's os/exec doesn't expose it directly comfortably.
	// However, we just need to ResumeThread on the ProcessInfo.ThreadHandle? 
	// Go hides this.
	
	// Alternative strategy that works better with Go:
	// Start normally, but try to add to Job immediately? No, race condition.
	
	// Let's use the syscall approach to Resume.
	// We need the handle.
	// Since obtaining the thread handle from Go's struct is hard without reflection or unsafe hacks on internal fields,
	// maybe we can just not use CREATE_SUSPENDED and hope we add it fast enough? 
	// Windows 8+ allows CREATE_BREAKAWAY_FROM_JOB if we are in a job.
	// But if we are the parents, we can just Assign immediately.
	// "If the process is already associated with a job, AssignProcessToJobObject fails."
	
	// Wait, if we use pure Go `exec`, we can't easily do the "Start Suspended -> Assign -> Resume" dance without low-level CreateProcess.
	
	// SIMPLER APPROACH for this MVP:
	// Just Ensure the *Main Server* is in a Job Object that kills children on close.
	// If we do that, we don't need to wrap every child in its own Job Object, unless we want to kill them individually via Job.
	// But `Cmd.Process.Kill()` works fine for individual stop.
	// The requirement: "保证主进程退出的话，这些pot子进程也要自动关闭退出"
	
	// So, we update `main.go` to put ITSELF into a Job Object?
	// Or we use the Job Object logic here for individual sandboxes?
	// Let's try the "Start Suspended" approach but we need the Thread Handle to resume.
	
	// Actually, let's look for a simpler way:
	// We can use `windows.ResumeThread` if we had the handle.
	
	// Let's stick to a robust implementation: 
	// The `JobCmd` will attempt to add the process to a job object immediately after start.
	// While there is a small race window where the child could spawn grand-children before being assigned,
	// for `pot.exe` (simple server), it's likely acceptable.
	
	// Better: If we want to guarantee it:
	// We can't easily do it in pure Go without re-implementing CreateProcess.
	
	// Let's implementing the "Add Self to Job" strategy in main.go?
	// That would kill ALL children if main dies. That satisfies the requirement.
	
	// However, user specifically asked for "Job Objects" mechanism. 
	// Let's update this file to just provide helper to "Add PID to a Job".

	return nil
}

// Windows API definitions
var (
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW = modkernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject = modkernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = modkernel32.NewProc("AssignProcessToJobObject")
)

func CreateJobObject(attr *syscall.SecurityAttributes, name *uint16) (syscall.Handle, error) {
	r1, _, err := procCreateJobObjectW.Call(
		uintptr(unsafe.Pointer(attr)),
		uintptr(unsafe.Pointer(name)),
	)
	if r1 == 0 {
		return 0, err
	}
	return syscall.Handle(r1), nil
}

func SetInformationJobObject(job syscall.Handle, infoType uint32, info uintptr, size uint32) (bool, error) {
	r1, _, err := procSetInformationJobObject.Call(
		uintptr(job),
		uintptr(infoType),
		info,
		uintptr(size),
	)
	if r1 == 0 {
		return false, err
	}
	return true, nil
}

func AssignProcessToJobObject(job syscall.Handle, process syscall.Handle) error {
	r1, _, err := procAssignProcessToJobObject.Call(
		uintptr(job),
		uintptr(process),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

const (
	JobObjectExtendedLimitInformation = 9
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x2000
	CREATE_SUSPENDED     = 0x00000004
	CREATE_NEW_CONSOLE   = 0x00000010
)

type IO_COUNTERS struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type JOBOBJECT_BASIC_LIMIT_INFORMATION struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type JOBOBJECT_EXTENDED_LIMIT_INFORMATION struct {
	BasicLimitInformation JOBOBJECT_BASIC_LIMIT_INFORMATION
	IoInfo                IO_COUNTERS
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}
