// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type candidateProcess struct {
	command     *exec.Cmd
	address     string
	binaryPath  string
	processTree uintptr
	exited      chan struct{}

	mu      sync.Mutex
	waitErr error
}

func taskkillArguments(pid int, force bool) []string {
	arguments := []string{"/PID", strconv.Itoa(pid), "/T"}
	if force {
		arguments = append(arguments, "/F")
	}
	return arguments
}

func startManagedProcess(command *exec.Cmd, address, binaryPath string) (*candidateProcess, error) {
	configureProcess(command)
	if err := command.Start(); err != nil {
		return nil, err
	}
	processTree, err := attachProcessTree(command)
	if err != nil {
		// Never return an unmanaged child. In particular, a Windows candidate
		// must be attached to its Job Object before it can be considered usable.
		_ = killProcess(command, 0)
		_ = command.Wait()
		return nil, errors.New("attach managed process tree: " + err.Error())
	}
	candidate := &candidateProcess{
		command:     command,
		address:     address,
		binaryPath:  binaryPath,
		processTree: processTree,
		exited:      make(chan struct{}),
	}
	go func() {
		err := command.Wait()
		candidate.mu.Lock()
		candidate.waitErr = err
		candidate.mu.Unlock()
		close(candidate.exited)
	}()
	return candidate, nil
}

func (c *candidateProcess) result() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.waitErr
}

func (c *candidateProcess) hasExited() bool {
	select {
	case <-c.exited:
		return true
	default:
		return false
	}
}

func (c *candidateProcess) cleanupProcessTree() error {
	c.mu.Lock()
	processTree := c.processTree
	c.processTree = 0
	c.mu.Unlock()
	return cleanupProcess(c.command, processTree)
}

func (c *candidateProcess) stop(ctx context.Context) error {
	defer func() {
		if c.binaryPath != "" {
			_ = os.Remove(c.binaryPath)
		}
	}()
	if c.hasExited() {
		return errors.Join(acceptableStopError(c.result()), c.cleanupProcessTree())
	}
	if err := terminateProcess(c.command, c.processTree); err != nil {
		// A graceful signal is best effort. Failure to deliver it immediately
		// escalates to the platform's process-tree termination primitive.
		_ = killProcess(c.command, c.processTree)
	}
	select {
	case <-c.exited:
		return errors.Join(acceptableStopError(c.result()), c.cleanupProcessTree())
	case <-ctx.Done():
		killErr := killProcess(c.command, c.processTree)
		select {
		case <-c.exited:
			return errors.Join(ctx.Err(), killErr, acceptableStopError(c.result()), c.cleanupProcessTree())
		case <-time.After(2 * time.Second):
			// Closing a Windows Job Object configured with
			// KILL_ON_JOB_CLOSE is the final bounded fallback. On Unix this
			// repeats the process-group kill without retaining resources.
			return errors.Join(ctx.Err(), killErr, c.cleanupProcessTree())
		}
	}
}

func acceptableStopError(err error) error {
	if err == nil {
		return nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return nil
	}
	return err
}
