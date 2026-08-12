// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"context"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestManagedProcessStopsAndWaits(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=TestManagedProcessHelper$")
	command.Env = append(os.Environ(), "HIMESAN_PROCESS_HELPER=1")
	candidate, err := startManagedProcess(command, "127.0.0.1:1", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := candidate.stop(ctx); err != nil {
		t.Fatalf("stop() error = %v", err)
	}
	if !candidate.hasExited() {
		t.Fatal("candidate process was not reaped")
	}
}

func TestManagedProcessStopsDescendantTree(t *testing.T) {
	if testing.Short() {
		t.Skip("helper-process integration test")
	}
	directory := t.TempDir()
	gatePath := filepath.Join(directory, "start-child")
	readyPath := filepath.Join(directory, "child-address")
	command := exec.Command(os.Args[0], "-test.run=TestManagedProcessHelper$")
	command.Env = append(os.Environ(),
		"HIMESAN_PROCESS_HELPER=tree-parent",
		"HIMESAN_PROCESS_GATE="+gatePath,
		"HIMESAN_PROCESS_READY="+readyPath,
	)
	candidate, err := startManagedProcess(command, "127.0.0.1:1", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gatePath, []byte("start"), 0o600); err != nil {
		t.Fatal(err)
	}
	address := waitForChildAddress(t, readyPath)
	waitForChildListener(t, address)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := candidate.stop(ctx); err != nil {
		t.Fatalf("stop() error = %v", err)
	}
	if !candidate.hasExited() {
		t.Fatal("candidate root process was not reaped")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		connection, dialErr := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if dialErr != nil {
			break
		}
		_ = connection.Close()
		if time.Now().After(deadline) {
			t.Fatalf("managed descendant still accepts connections at %s", address)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTaskkillArguments(t *testing.T) {
	t.Parallel()
	if got, want := taskkillArguments(42, false), []string{"/PID", "42", "/T"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("taskkillArguments(graceful) = %q, want %q", got, want)
	}
	if got, want := taskkillArguments(42, true), []string{"/PID", "42", "/T", "/F"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("taskkillArguments(force) = %q, want %q", got, want)
	}
}

func TestManagedProcessHelper(t *testing.T) {
	switch os.Getenv("HIMESAN_PROCESS_HELPER") {
	case "":
		return
	case "tree-parent":
		runTreeParentHelper()
	case "tree-child":
		runTreeChildHelper()
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals)
	<-signals
	os.Exit(0)
}

func runTreeParentHelper() {
	gatePath := os.Getenv("HIMESAN_PROCESS_GATE")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(gatePath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			os.Exit(2)
		}
		time.Sleep(10 * time.Millisecond)
	}
	child := exec.Command(os.Args[0], "-test.run=TestManagedProcessHelper$")
	child.Env = replaceEnvironment(os.Environ(), "HIMESAN_PROCESS_HELPER", "tree-child")
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
}

func runTreeChildHelper() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	defer listener.Close()
	if err := os.WriteFile(os.Getenv("HIMESAN_PROCESS_READY"), []byte(listener.Addr().String()), 0o600); err != nil {
		os.Exit(2)
	}
	for {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			os.Exit(0)
		}
		_ = connection.Close()
	}
}

func waitForChildAddress(t *testing.T, readyPath string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		contents, err := os.ReadFile(readyPath)
		if err == nil && strings.TrimSpace(string(contents)) != "" {
			return strings.TrimSpace(string(contents))
		}
		if time.Now().After(deadline) {
			t.Fatalf("managed descendant did not report its address: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForChildListener(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for {
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		lastErr = err
		if time.Now().After(deadline) {
			t.Fatalf("managed descendant did not accept a connection at %s: %v", address, lastErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
