// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"gamertan.com/sandwich-hime/internal/compiler"
	"gamertan.com/sandwich-hime/internal/devserver"
	"gamertan.com/sandwich-hime/internal/lsp"
	"gamertan.com/sandwich-hime/internal/version"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}

	switch args[0] {
	case "generate", "gen":
		return runCompilerCommand(ctx, "generate", args[1:], stdout, stderr, compiler.Generate)
	case "check", "bless":
		return runCompilerCommand(ctx, args[0], args[1:], stdout, stderr, compiler.Check)
	case "dev":
		return runDev(ctx, args[1:], stdout, stderr)
	case "lsp":
		return runLSP(ctx, args[1:], os.Stdin, stdout, stderr)
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printHelp(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "himesan: unknown command %q\n\n", args[0])
		printHelp(stderr)
		return 2
	}
}

type compilerOperation func(context.Context, []string) (compiler.Result, error)

func runCompilerCommand(ctx context.Context, command string, args []string, stdout, stderr io.Writer, operation compilerOperation) int {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "emit one machine-readable JSON result")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	result, operationErr := operation(ctx, flags.Args())
	if *jsonOutput {
		payload := struct {
			Command string          `json:"command"`
			OK      bool            `json:"ok"`
			Result  compiler.Result `json:"result"`
		}{Command: command, OK: operationErr == nil, Result: result}
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(payload); err != nil {
			fmt.Fprintf(stderr, "himesan: encode JSON result: %v\n", err)
			return 2
		}
	} else {
		printDiagnostics(stderr, result.Diagnostics)
		if operationErr == nil {
			switch command {
			case "generate":
				fmt.Fprintf(stdout, "generated %d, unchanged %d (%d .sando files)\n", result.Changed, result.Unchanged, result.Discovered)
			case "bless":
				fmt.Fprintf(stdout, "blessed: generated output is current and valid (%d files checked, no writes)\n", result.Discovered)
			default:
				fmt.Fprintf(stdout, "checked %d .sando files: %d current\n", result.Discovered, result.Unchanged)
			}
		}
	}
	if operationErr != nil {
		return 1
	}
	return 0
}

func printDiagnostics(output io.Writer, diagnostics []compiler.Diagnostic) {
	for _, item := range diagnostics {
		fmt.Fprintf(output, "%s: %s\n", item.Severity, item.Error())
	}
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "emit machine-readable version information")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "himesan version does not accept positional arguments")
		return 2
	}
	information := struct {
		Compiler   string   `json:"compiler"`
		RuntimeABI string   `json:"runtime_abi"`
		Go         string   `json:"go"`
		Features   []string `json:"features"`
	}{Compiler: version.Compiler, RuntimeABI: version.RuntimeABI, Go: runtime.Version(), Features: []string{"lsp-stdio"}}
	if *jsonOutput {
		if err := json.NewEncoder(stdout).Encode(information); err != nil {
			fmt.Fprintf(stderr, "himesan: encode version: %v\n", err)
			return 2
		}
		return 0
	}
	fmt.Fprintf(stdout, "himesan %s (runtime ABI %s, %s)\n", information.Compiler, information.RuntimeABI, information.Go)
	return 0
}

func runLSP(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("lsp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	stdio := flags.Bool("stdio", false, "serve Language Server Protocol JSON-RPC over stdin/stdout")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if !*stdio || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "himesan lsp requires exactly --stdio")
		return 2
	}
	if err := lsp.Run(ctx, lsp.Options{Input: stdin, Output: stdout, LogOutput: stderr}); err != nil {
		fmt.Fprintf(stderr, "himesan lsp: %v\n", err)
		return 1
	}
	return 0
}

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }
func (values *stringList) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value must not be empty")
	}
	*values = append(*values, value)
	return nil
}

func runDev(parent context.Context, args []string, stdout, stderr io.Writer) int {
	commandArgs, appArgs := splitAppArgs(args)
	flags := flag.NewFlagSet("dev", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "himesan.json path (defaults to ./himesan.json when present)")
	proxyAddress := flags.String("proxy", "", "stable loopback proxy address")
	listenEnvironment := flags.String("listen-env", "", "environment variable used to pass the random upstream address")
	healthPath := flags.String("health", "", "candidate health-check path")
	var sourceRoots stringList
	var watchRoots stringList
	flags.Var(&sourceRoots, "source", "source root to generate and watch (repeatable)")
	flags.Var(&watchRoots, "watch", "additional asset root to watch (repeatable)")
	if err := flags.Parse(commandArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderr, "himesan dev accepts at most one Go package before --")
		return 2
	}

	rootDir, config, err := loadDevelopmentConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "himesan dev: %v\n", err)
		return 1
	}
	if flags.NArg() == 1 {
		config.GoPackage = flags.Arg(0)
	}
	if len(sourceRoots) != 0 {
		config.SourceRoots = append([]string(nil), sourceRoots...)
	}
	if len(watchRoots) != 0 {
		config.AdditionalWatchRoots = append([]string(nil), watchRoots...)
	}
	if *proxyAddress != "" {
		config.ProxyAddress = *proxyAddress
	}
	if *listenEnvironment != "" {
		config.ListenAddressEnv = *listenEnvironment
	}
	if *healthPath != "" {
		config.HealthPath = *healthPath
	}
	if appArgs != nil {
		config.AppArgs = append([]string(nil), appArgs...)
	}
	if err := config.Validate(); err != nil {
		fmt.Fprintf(stderr, "himesan dev: %v\n", err)
		return 2
	}

	resolvedSources := resolvePaths(rootDir, config.SourceRoots)
	generate := func(ctx context.Context) error {
		result, generateErr := compiler.Generate(ctx, resolvedSources)
		for _, item := range result.Diagnostics {
			if item.Severity == compiler.SeverityWarning {
				fmt.Fprintf(stderr, "warning: %s\n", item.Error())
			}
		}
		return generateErr
	}

	supervisor, err := devserver.New(devserver.Options{
		RootDir:        rootDir,
		Config:         config,
		Generate:       generate,
		MapDiagnostics: mapDevelopmentDiagnostics,
		OnEvent: func(event devserver.Event) {
			switch event.Type {
			case "ready":
				fmt.Fprintf(stdout, "himesan dev: %s\n", event.Message)
			case "reload":
				fmt.Fprintln(stdout, "himesan dev: healthy candidate activated")
			case "diagnostic":
				fmt.Fprintf(stderr, "himesan dev [%s]: %s\n", event.Phase, event.Message)
			}
		},
		Output:      stdout,
		ErrorOutput: stderr,
	})
	if err != nil {
		fmt.Fprintf(stderr, "himesan dev: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := supervisor.Run(ctx); err != nil {
		fmt.Fprintf(stderr, "himesan dev: %v\n", err)
		return 1
	}
	return 0
}

func splitAppArgs(args []string) ([]string, []string) {
	for index, value := range args {
		if value == "--" {
			return args[:index], args[index+1:]
		}
	}
	return args, nil
}

func loadDevelopmentConfig(requested string) (string, devserver.Config, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", devserver.Config{}, fmt.Errorf("get working directory: %w", err)
	}
	path := requested
	if path == "" {
		candidate := filepath.Join(workingDirectory, "himesan.json")
		if _, statErr := os.Stat(candidate); statErr == nil {
			path = candidate
		} else if !os.IsNotExist(statErr) {
			return "", devserver.Config{}, fmt.Errorf("inspect himesan.json: %w", statErr)
		}
	}
	if path == "" {
		return workingDirectory, devserver.DefaultConfig(), nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", devserver.Config{}, fmt.Errorf("resolve config: %w", err)
	}
	config, err := devserver.LoadConfig(absolute)
	if err != nil {
		return "", devserver.Config{}, err
	}
	return filepath.Dir(absolute), config, nil
}

func resolvePaths(root string, paths []string) []string {
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		if filepath.IsAbs(path) {
			resolved = append(resolved, filepath.Clean(path))
		} else {
			resolved = append(resolved, filepath.Join(root, filepath.Clean(path)))
		}
	}
	return resolved
}

func mapDevelopmentDiagnostics(err error) []devserver.Diagnostic {
	var diagnosticsError *compiler.DiagnosticsError
	if !errors.As(err, &diagnosticsError) {
		return nil
	}
	result := make([]devserver.Diagnostic, 0, len(diagnosticsError.Diagnostics))
	for _, item := range diagnosticsError.Diagnostics {
		result = append(result, devserver.Diagnostic{
			Path: item.Path, Line: item.Line, Column: item.Column,
			Code: item.Code, Message: item.Message, Severity: string(item.Severity),
		})
	}
	return result
}

func printHelp(output io.Writer) {
	fmt.Fprintln(output, "Sandwich Hime / Hime-san — HTML-first typed components for Go")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Usage:")
	fmt.Fprintln(output, "  himesan generate [--json] [paths...]  generate adjacent .sando.go files")
	fmt.Fprintln(output, "  himesan gen [--json] [paths...]       alias for generate")
	fmt.Fprintln(output, "  himesan check [--json] [paths...]     validate sources and committed output without writes")
	fmt.Fprintln(output, "  himesan bless [--json] [paths...]     friendly read-only alias for check")
	fmt.Fprintln(output, "  himesan dev [flags] [package] [-- app-args...]  run the loopback last-good supervisor")
	fmt.Fprintln(output, "  himesan lsp --stdio                  run the read-only language server")
	fmt.Fprintln(output, "  himesan version [--json]              print compiler and runtime ABI versions")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Templates use .sando; .san remains exclusively San language source.")
}
