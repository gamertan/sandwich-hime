// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSupervisorBuildsSwapsAndCleansUp(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test builds temporary Go applications")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/himesan-dev-test\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "main.go")
	writeTestApplication(t, mainPath, "version one", true)

	cfg := DefaultConfig()
	cfg.ProxyAddress = "127.0.0.1:0"
	cfg.HealthPath = "/healthz"
	var generations atomic.Int32
	events := make(chan Event, 32)
	supervisor, err := New(Options{
		RootDir: root,
		Config:  cfg,
		Generate: func(context.Context) error {
			generations.Add(1)
			return nil
		},
		OnEvent:         func(event Event) { events <- event },
		CacheDir:        filepath.Join(t.TempDir(), "cache"),
		PollInterval:    25 * time.Millisecond,
		Debounce:        25 * time.Millisecond,
		BuildTimeout:    30 * time.Second,
		StartupTimeout:  750 * time.Millisecond,
		ShutdownTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() { runResult <- supervisor.Run(ctx) }()

	proxyAddress := waitForProxyAddress(t, supervisor)
	waitForBody(t, "http://"+proxyAddress+"/", "version one")
	firstUpstream := supervisor.proxy.target.Load().Host

	if err := os.WriteFile(mainPath, []byte("package main\nfunc"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitForPhase(t, events, "build")
	waitForBody(t, "http://"+proxyAddress+"/", "version one")

	writeTestApplication(t, mainPath, "unhealthy candidate", false)
	waitForPhase(t, events, "startup")
	waitForBody(t, "http://"+proxyAddress+"/", "version one")

	writeTestApplication(t, mainPath, "version two", true)
	waitForBody(t, "http://"+proxyAddress+"/", "version two")
	if generations.Load() < 4 {
		t.Fatalf("Generate hook ran %d times, want at least 4", generations.Load())
	}
	secondUpstream := supervisor.proxy.target.Load().Host
	if firstUpstream == secondUpstream {
		t.Fatalf("healthy candidate was not swapped: %s", firstUpstream)
	}
	// The proxy target changes before graceful shutdown of the replaced child
	// completes. Wait for that bounded cleanup instead of racing the supervisor
	// immediately after the first response from the new target.
	waitForConnectionRefused(t, firstUpstream, 3*time.Second)

	cancel()
	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
	waitForConnectionRefused(t, secondUpstream, 3*time.Second)
}

func TestSupervisorClearsTargetWhenCurrentApplicationExits(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test builds a temporary Go application")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/himesan-dev-exit-test\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "main.go")
	writeExitingTestApplication(t, mainPath, "short lived", 1500*time.Millisecond)

	cfg := DefaultConfig()
	cfg.ProxyAddress = "127.0.0.1:0"
	cfg.HealthPath = "/healthz"
	events := make(chan Event, 16)
	supervisor, err := New(Options{
		RootDir:         root,
		Config:          cfg,
		Generate:        func(context.Context) error { return nil },
		OnEvent:         func(event Event) { events <- event },
		CacheDir:        filepath.Join(t.TempDir(), "cache"),
		PollInterval:    30 * time.Second,
		Debounce:        25 * time.Millisecond,
		BuildTimeout:    30 * time.Second,
		StartupTimeout:  time.Second,
		ShutdownTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() { runResult <- supervisor.Run(ctx) }()
	t.Cleanup(cancel)

	proxyAddress := waitForProxyAddress(t, supervisor)
	waitForBody(t, "http://"+proxyAddress+"/", "short lived")
	waitForPhase(t, events, "run")
	if target := supervisor.proxy.target.Load(); target != nil {
		t.Fatalf("proxy retained exited upstream %v", target)
	}

	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Second}
	response, err := client.Get("http://" + proxyAddress + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), "waiting for a healthy application") {
		t.Fatalf("dead upstream response = %d %q", response.StatusCode, body)
	}

	cancel()
	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
}

func TestGenerationFailureDoesNotMoveProxyTarget(t *testing.T) {
	t.Parallel()
	upstream := http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "last good")
	})}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = upstream.Close() })
	go func() { _ = upstream.Serve(listener) }()

	wantError := errors.New("templates are invalid")
	events := make(chan Event, 1)
	supervisor, err := New(Options{
		RootDir: t.TempDir(),
		Config:  DefaultConfig(),
		Generate: func(context.Context) error {
			return wantError
		},
		OnEvent:  func(event Event) { events <- event },
		CacheDir: filepath.Join(t.TempDir(), "cache"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := supervisor.proxy.setTarget(listener.Addr().String()); err != nil {
		t.Fatal(err)
	}
	wantTarget := supervisor.proxy.target.Load().String()
	if candidate := supervisor.buildHealthyCandidate(context.Background()); candidate != nil {
		t.Fatal("generation failure unexpectedly produced a candidate")
	}
	if got := supervisor.proxy.target.Load().String(); got != wantTarget {
		t.Fatalf("proxy target changed from %q to %q", wantTarget, got)
	}
	select {
	case event := <-events:
		if event.Type != "diagnostic" || event.Phase != "generate" || !strings.Contains(event.Message, wantError.Error()) {
			t.Fatalf("generation event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("generation diagnostic was not emitted")
	}
}

func writeTestApplication(t *testing.T, path, message string, healthy bool) {
	t.Helper()
	healthStatus := "http.StatusNoContent"
	if !healthy {
		healthStatus = "http.StatusServiceUnavailable"
	}
	contents := fmt.Sprintf(`package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(%s) })
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<!doctype html><html><body>%s</body></html>")
	})
	if err := http.ListenAndServe(os.Getenv("HIMESAN_LISTEN_ADDR"), mux); err != nil {
		panic(err)
	}
}
`, healthStatus, message)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeExitingTestApplication(t *testing.T, path, message string, lifetime time.Duration) {
	t.Helper()
	contents := fmt.Sprintf(`package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	go func() {
		time.Sleep(%d * time.Millisecond)
		os.Exit(0)
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<!doctype html><html><body>%s</body></html>")
	})
	if err := http.ListenAndServe(os.Getenv("HIMESAN_LISTEN_ADDR"), mux); err != nil {
		panic(err)
	}
}
`, lifetime.Milliseconds(), message)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitForPhase(t *testing.T, events <-chan Event, phase string) {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-events:
			if event.Type == "diagnostic" && event.Phase == phase {
				return
			}
		case <-timer.C:
			t.Fatalf("did not receive %s diagnostic", phase)
		}
	}
}

func waitForProxyAddress(t *testing.T, supervisor *Supervisor) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if address := supervisor.ProxyAddress(); address != "" {
			return address
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("development proxy did not start")
	return ""
}

func waitForBody(t *testing.T, url, want string) {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		response, err := client.Get(url)
		if err == nil {
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if readErr == nil {
				last = string(body)
				if response.StatusCode == http.StatusOK && strings.Contains(last, want) && strings.Contains(last, string(reloadClientTag)) {
					return
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("proxy never served %q; last body = %q", want, last)
}

func waitForConnectionRefused(t *testing.T, address string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err != nil {
			return
		}
		_ = connection.Close()
		if time.Now().After(deadline) {
			t.Fatalf("replaced child still accepts connections at %s after %s", address, timeout)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestLimitedDiagnosticBuffer(t *testing.T) {
	t.Parallel()
	var buffer limitedDiagnosticBuffer
	contents := strings.Repeat("x", maxDiagnosticOutput+100)
	written, err := buffer.Write([]byte(contents))
	if err != nil || written != len(contents) {
		t.Fatalf("Write() = %d, %v", written, err)
	}
	if buffer.Buffer.Len() != maxDiagnosticOutput {
		t.Fatalf("stored bytes = %d, want %d", buffer.Buffer.Len(), maxDiagnosticOutput)
	}
	if !strings.HasSuffix(buffer.String(), "diagnostic output truncated") {
		t.Fatalf("String() did not report truncation: %q", buffer.String())
	}
}

func TestReplaceEnvironment(t *testing.T) {
	t.Parallel()
	got := replaceEnvironment([]string{"A=one", "HIMESAN_LISTEN_ADDR=old", "B=two"}, "HIMESAN_LISTEN_ADDR", "127.0.0.1:1")
	want := []string{"A=one", "B=two", "HIMESAN_LISTEN_ADDR=127.0.0.1:1"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("replaceEnvironment() = %q, want %q", got, want)
	}
}

func TestHealthCheckDoesNotFollowRedirectOffLoopback(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "http://example.invalid/escaped")
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()

	cfg := DefaultConfig()
	supervisor, err := New(Options{
		RootDir:  t.TempDir(),
		Config:   cfg,
		Generate: func(context.Context) error { return nil },
		CacheDir: filepath.Join(t.TempDir(), "cache"),
	})
	if err != nil {
		t.Fatal(err)
	}
	candidate := &candidateProcess{
		address: strings.TrimPrefix(upstream.URL, "http://"),
		exited:  make(chan struct{}),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := supervisor.waitUntilHealthy(ctx, candidate); err != nil {
		t.Fatalf("loopback redirect status should be observed without following it: %v", err)
	}
}
