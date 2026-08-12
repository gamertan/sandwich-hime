// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package devserver

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

const (
	processSetQuota                   = 0x0100
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x00002000
)

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInfo struct {
	BasicLimitInformation jobObjectBasicLimitInfo
	IOInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	assignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	createJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	generateConsoleCtrlEvent = kernel32.NewProc("GenerateConsoleCtrlEvent")
	setInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	terminateJobObject       = kernel32.NewProc("TerminateJobObject")
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// attachProcessTree places the candidate in a Windows Job Object. Job
// membership is inherited by descendants, so they remain terminable even when
// the root process exits before cleanup reaches it.
func attachProcessTree(command *exec.Cmd) (uintptr, error) {
	if command.Process == nil {
		return 0, errors.New("candidate process is unavailable")
	}
	job, _, createErr := createJobObjectW.Call(0, 0)
	if job == 0 {
		return 0, fmt.Errorf("CreateJobObjectW: %w", createErr)
	}
	limits := jobObjectExtendedLimitInfo{}
	limits.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	configured, _, configureErr := setInformationJobObject.Call(
		job,
		jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		unsafe.Sizeof(limits),
	)
	if configured == 0 {
		_ = syscall.CloseHandle(syscall.Handle(job))
		return 0, fmt.Errorf("SetInformationJobObject: %w", configureErr)
	}
	process, err := syscall.OpenProcess(processSetQuota|syscall.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err != nil {
		_ = syscall.CloseHandle(syscall.Handle(job))
		return 0, fmt.Errorf("open candidate for Job Object assignment: %w", err)
	}
	defer syscall.CloseHandle(process)
	assigned, _, assignErr := assignProcessToJobObject.Call(job, uintptr(process))
	if assigned == 0 {
		_ = syscall.CloseHandle(syscall.Handle(job))
		return 0, fmt.Errorf("AssignProcessToJobObject: %w", assignErr)
	}
	return job, nil
}

func terminateProcess(command *exec.Cmd, _ uintptr) error {
	if command.Process == nil {
		return nil
	}
	result, _, callErr := generateConsoleCtrlEvent.Call(syscall.CTRL_BREAK_EVENT, uintptr(command.Process.Pid))
	if result == 0 {
		return fmt.Errorf("GenerateConsoleCtrlEvent: %w", callErr)
	}
	return nil
}

func killProcess(command *exec.Cmd, processTree uintptr) error {
	if processTree != 0 {
		result, _, callErr := terminateJobObject.Call(processTree, 1)
		if result != 0 {
			return nil
		}
		return fmt.Errorf("TerminateJobObject: %w", callErr)
	}
	if command.Process == nil {
		return nil
	}
	if err := runTaskkill(command.Process.Pid, true); err != nil {
		return errors.Join(err, command.Process.Kill())
	}
	return nil
}

func cleanupProcess(command *exec.Cmd, processTree uintptr) error {
	if processTree == 0 {
		// A successfully returned Windows candidate always owns a Job Object.
		// Zero therefore means cleanup already ran; do not target a potentially
		// recycled process ID.
		return nil
	}
	terminateErr := killProcess(command, processTree)
	closeErr := syscall.CloseHandle(syscall.Handle(processTree))
	return errors.Join(terminateErr, closeErr)
}

func runTaskkill(pid int, force bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "taskkill", taskkillArguments(pid, force)...).Run()
}
