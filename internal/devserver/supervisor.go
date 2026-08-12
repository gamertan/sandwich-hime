// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const maxDiagnosticOutput = 64 << 10

// GenerateFunc regenerates affected .sando.go files. The supervisor does not
// import the compiler: the CLI supplies this hook.
type GenerateFunc func(context.Context) error

// Options configures a Supervisor. Durations and output writers have safe
// defaults when omitted.
type Options struct {
	RootDir        string
	Config         Config
	Generate       GenerateFunc
	MapDiagnostics func(error) []Diagnostic
	OnEvent        func(Event)

	Output      io.Writer
	ErrorOutput io.Writer
	GoCommand   string
	CacheDir    string

	PollInterval    time.Duration
	Debounce        time.Duration
	BuildTimeout    time.Duration
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
	HTTPClient      *http.Client
}

// Supervisor owns the local proxy, watcher, build candidates, and current
// healthy application child.
type Supervisor struct {
	options  Options
	rootDir  string
	cacheDir string

	hub   *eventHub
	proxy *developmentProxy

	running   atomic.Bool
	addressMu sync.RWMutex
	address   string
}

// New validates and normalizes a local development supervisor without opening
// listeners or starting processes.
func New(options Options) (*Supervisor, error) {
	if err := options.Config.Validate(); err != nil {
		return nil, fmt.Errorf("development config: %w", err)
	}
	if options.Generate == nil {
		return nil, errors.New("development generate hook is required")
	}
	options.Config.SourceRoots = append([]string(nil), options.Config.SourceRoots...)
	options.Config.AppArgs = append([]string(nil), options.Config.AppArgs...)
	options.Config.AdditionalWatchRoots = append([]string(nil), options.Config.AdditionalWatchRoots...)
	rootDir := options.RootDir
	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get project directory: %w", err)
		}
	}
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project directory: %w", err)
	}
	info, err := os.Stat(rootDir)
	if err != nil {
		return nil, fmt.Errorf("inspect project directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project root %q is not a directory", rootDir)
	}

	applyOptionDefaults(&options)
	cacheDir := options.CacheDir
	if cacheDir == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("locate user cache directory: %w", err)
		}
		key := sha256.Sum256([]byte(rootDir + "\x00" + options.Config.GoPackage))
		cacheDir = filepath.Join(userCache, "himesan", "dev", hex.EncodeToString(key[:8]))
	} else if !filepath.IsAbs(cacheDir) {
		cacheDir = filepath.Join(rootDir, cacheDir)
	}

	hub := newEventHub()
	return &Supervisor{
		options:  options,
		rootDir:  rootDir,
		cacheDir: filepath.Clean(cacheDir),
		hub:      hub,
		proxy:    newDevelopmentProxy(hub),
	}, nil
}

func applyOptionDefaults(options *Options) {
	if options.Output == nil {
		options.Output = io.Discard
	}
	if options.ErrorOutput == nil {
		options.ErrorOutput = io.Discard
	}
	if options.GoCommand == "" {
		options.GoCommand = "go"
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 250 * time.Millisecond
	}
	if options.Debounce <= 0 {
		options.Debounce = 150 * time.Millisecond
	}
	if options.BuildTimeout <= 0 {
		options.BuildTimeout = 2 * time.Minute
	}
	if options.StartupTimeout <= 0 {
		options.StartupTimeout = 10 * time.Second
	}
	if options.ShutdownTimeout <= 0 {
		options.ShutdownTimeout = 5 * time.Second
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{
			Transport: &http.Transport{Proxy: nil},
			Timeout:   time.Second,
		}
	} else {
		copy := *options.HTTPClient
		options.HTTPClient = &copy
	}
	// Health redirects are status results, not permission to leave loopback.
	options.HTTPClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
}

// ProxyAddress reports the bound stable proxy address after Run has opened its
// listener. It is useful when Config.ProxyAddress requests port zero in tests.
func (s *Supervisor) ProxyAddress() string {
	s.addressMu.RLock()
	defer s.addressMu.RUnlock()
	return s.address
}

// Run serves until ctx is canceled or the stable proxy fails. A generation,
// build, startup, or health-check failure is reported as a diagnostic event and
// leaves the last healthy child serving.
func (s *Supervisor) Run(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return errors.New("development supervisor may only be run once")
	}
	if err := os.MkdirAll(s.cacheDir, 0o700); err != nil {
		return fmt.Errorf("create development cache: %w", err)
	}
	if err := os.Chmod(s.cacheDir, 0o700); err != nil {
		return fmt.Errorf("secure development cache: %w", err)
	}
	listener, err := net.Listen("tcp", s.options.Config.ProxyAddress)
	if err != nil {
		return fmt.Errorf("listen on development proxy: %w", err)
	}
	if err := s.proxy.setAuthority(listener.Addr().String()); err != nil {
		_ = listener.Close()
		return err
	}
	s.addressMu.Lock()
	s.address = listener.Addr().String()
	s.addressMu.Unlock()

	server := &http.Server{
		Handler:           s.proxy,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       75 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverErrors <- err
	}()

	var current *candidateProcess
	defer func() {
		s.hub.close()
		s.proxy.closeIdleConnections()
		serverCtx, cancelServer := context.WithTimeout(context.Background(), s.options.ShutdownTimeout)
		_ = server.Shutdown(serverCtx)
		cancelServer()
		if current != nil {
			processCtx, cancelProcess := context.WithTimeout(context.Background(), s.options.ShutdownTimeout)
			_ = current.stop(processCtx)
			cancelProcess()
		}
	}()

	s.emit(Event{Type: "ready", Phase: "proxy", Message: "http://" + listener.Addr().String()})
	if candidate := s.buildHealthyCandidate(ctx); candidate != nil {
		current = s.activateCandidate(candidate, current)
	}

	roots := makeWatchRoots(s.rootDir, s.options.Config)
	snapshot, snapshotErr := takeSnapshot(roots)
	lastWatchError := ""
	if snapshotErr != nil {
		lastWatchError = snapshotErr.Error()
		s.report("watch", snapshotErr)
	}
	ticker := time.NewTicker(s.options.PollInterval)
	defer ticker.Stop()
	var pending bool
	var changedAt time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-serverErrors:
			if err != nil {
				return fmt.Errorf("development proxy: %w", err)
			}
			return nil
		case now := <-ticker.C:
			next, watchErr := takeSnapshot(roots)
			watchError := ""
			if watchErr != nil {
				watchError = watchErr.Error()
			}
			if watchError != "" && watchError != lastWatchError {
				s.report("watch", watchErr)
			}
			lastWatchError = watchError
			if !snapshotsEqual(snapshot, next) {
				snapshot = next
				pending = true
				changedAt = now
			}
			if current != nil && current.hasExited() {
				exitErr := current.result()
				if exitErr == nil {
					exitErr = errors.New("application exited")
				} else {
					exitErr = fmt.Errorf("application exited: %w", exitErr)
				}
				s.report("run", exitErr)
				_ = current.cleanupProcessTree()
				_ = os.Remove(current.binaryPath)
				current = nil
			}
			if pending && now.Sub(changedAt) >= s.options.Debounce {
				pending = false
				if candidate := s.buildHealthyCandidate(ctx); candidate != nil {
					current = s.activateCandidate(candidate, current)
				}
			}
		}
	}
}

func (s *Supervisor) buildHealthyCandidate(ctx context.Context) *candidateProcess {
	if err := s.options.Generate(ctx); err != nil {
		s.report("generate", err)
		return nil
	}
	binaryPath, err := s.build(ctx)
	if err != nil {
		s.report("build", err)
		return nil
	}
	candidate, err := s.startAndCheck(ctx, binaryPath)
	if err != nil {
		_ = os.Remove(binaryPath)
		s.report("startup", err)
		return nil
	}
	return candidate
}

func (s *Supervisor) build(ctx context.Context) (string, error) {
	buildCtx, cancel := context.WithTimeout(ctx, s.options.BuildTimeout)
	defer cancel()
	template := "candidate-*"
	if runtime.GOOS == "windows" {
		template += ".exe"
	}
	placeholder, err := os.CreateTemp(s.cacheDir, template)
	if err != nil {
		return "", fmt.Errorf("reserve candidate binary: %w", err)
	}
	binaryPath := placeholder.Name()
	if err := placeholder.Close(); err != nil {
		_ = os.Remove(binaryPath)
		return "", fmt.Errorf("close candidate placeholder: %w", err)
	}
	if err := os.Remove(binaryPath); err != nil {
		return "", fmt.Errorf("prepare candidate binary: %w", err)
	}
	command := exec.CommandContext(buildCtx, s.options.GoCommand, "build", "-o", binaryPath, "--", s.options.Config.GoPackage)
	command.Dir = s.rootDir
	var diagnostics limitedDiagnosticBuffer
	command.Stdout = io.MultiWriter(s.options.Output, &diagnostics)
	command.Stderr = io.MultiWriter(s.options.ErrorOutput, &diagnostics)
	if err := command.Run(); err != nil {
		_ = os.Remove(binaryPath)
		message := diagnostics.String()
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("go build failed: %s", message)
	}
	return binaryPath, nil
}

func (s *Supervisor) startAndCheck(ctx context.Context, binaryPath string) (*candidateProcess, error) {
	address, err := unusedLoopbackAddress()
	if err != nil {
		return nil, err
	}
	command := exec.Command(binaryPath, s.options.Config.AppArgs...)
	command.Dir = s.rootDir
	command.Env = replaceEnvironment(os.Environ(), s.options.Config.ListenAddressEnv, address)
	command.Stdout = s.options.Output
	command.Stderr = s.options.ErrorOutput
	candidate, err := startManagedProcess(command, address, binaryPath)
	if err != nil {
		return nil, fmt.Errorf("start candidate: %w", err)
	}
	startupCtx, cancel := context.WithTimeout(ctx, s.options.StartupTimeout)
	defer cancel()
	if err := s.waitUntilHealthy(startupCtx, candidate); err != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), s.options.ShutdownTimeout)
		defer stopCancel()
		_ = candidate.stop(stopCtx)
		return nil, err
	}
	return candidate, nil
}

func unusedLoopbackAddress() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("reserve candidate address: %w", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return "", fmt.Errorf("release candidate address: %w", err)
	}
	return address, nil
}

func (s *Supervisor) waitUntilHealthy(ctx context.Context, candidate *candidateProcess) error {
	url := "http://" + candidate.address + s.options.Config.HealthPath
	ticker := time.NewTicker(75 * time.Millisecond)
	defer ticker.Stop()
	var lastError error
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("create health request: %w", err)
		}
		response, err := s.options.HTTPClient.Do(request)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
			_ = response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 400 {
				return nil
			}
			lastError = fmt.Errorf("health endpoint returned %s", response.Status)
		} else {
			lastError = err
		}
		select {
		case <-candidate.exited:
			exitErr := candidate.result()
			if exitErr == nil {
				return errors.New("candidate exited before becoming healthy")
			}
			return fmt.Errorf("candidate exited before becoming healthy: %w", exitErr)
		case <-ctx.Done():
			if lastError == nil {
				lastError = ctx.Err()
			}
			return fmt.Errorf("candidate did not become healthy: %w", lastError)
		case <-ticker.C:
		}
	}
}

func (s *Supervisor) activateCandidate(candidate, previous *candidateProcess) *candidateProcess {
	if err := s.proxy.setTarget(candidate.address); err != nil {
		s.report("proxy", err)
		stopCtx, cancel := context.WithTimeout(context.Background(), s.options.ShutdownTimeout)
		defer cancel()
		_ = candidate.stop(stopCtx)
		return previous
	}
	s.emit(Event{Type: "reload", Phase: "serve", Message: "healthy application activated"})
	if previous != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), s.options.ShutdownTimeout)
		_ = previous.stop(stopCtx)
		cancel()
	}
	return candidate
}

func (s *Supervisor) emit(event Event) {
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	s.hub.publish(event)
	if s.options.OnEvent != nil {
		s.options.OnEvent(event)
	}
}

func (s *Supervisor) report(phase string, err error) {
	if err == nil {
		return
	}
	event := Event{Type: "diagnostic", Phase: phase, Message: truncateDiagnostic(err.Error())}
	if s.options.MapDiagnostics != nil {
		event.Diagnostics = s.options.MapDiagnostics(err)
	}
	s.emit(event)
}

func replaceEnvironment(environment []string, name, value string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		itemName, _, ok := strings.Cut(item, "=")
		matches := ok && itemName == name
		if runtime.GOOS == "windows" {
			matches = ok && strings.EqualFold(itemName, name)
		}
		if matches {
			continue
		}
		result = append(result, item)
	}
	return append(result, name+"="+value)
}

func truncateDiagnostic(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= maxDiagnosticOutput {
		return message
	}
	return strings.ToValidUTF8(message[:maxDiagnosticOutput], "�") + "\n… diagnostic output truncated"
}

type limitedDiagnosticBuffer struct {
	bytes.Buffer
	truncated bool
}

func (b *limitedDiagnosticBuffer) Write(contents []byte) (int, error) {
	originalLength := len(contents)
	remaining := maxDiagnosticOutput - b.Buffer.Len()
	writtenLength := 0
	if remaining > 0 {
		if len(contents) > remaining {
			contents = contents[:remaining]
		}
		writtenLength, _ = b.Buffer.Write(contents)
	}
	if originalLength > writtenLength {
		b.truncated = true
	}
	return originalLength, nil
}

func (b *limitedDiagnosticBuffer) String() string {
	message := strings.TrimSpace(strings.ToValidUTF8(b.Buffer.String(), "�"))
	if b.truncated {
		message += "\n… diagnostic output truncated"
	}
	return message
}
