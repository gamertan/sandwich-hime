// SPDX-License-Identifier: AGPL-3.0-only
//go:build !windows

package devserver

import (
	"errors"
	"os/exec"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func attachProcessTree(_ *exec.Cmd) (uintptr, error) { return 0, nil }

func terminateProcess(command *exec.Cmd, _ uintptr) error {
	if command.Process == nil {
		return nil
	}
	if err := syscall.Kill(-command.Process.Pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return command.Process.Signal(syscall.SIGTERM)
	}
	return nil
}

func killProcess(command *exec.Cmd, _ uintptr) error {
	if command.Process == nil {
		return nil
	}
	if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return command.Process.Kill()
	}
	return nil
}

func cleanupProcess(command *exec.Cmd, processTree uintptr) error {
	return killProcess(command, processTree)
}
