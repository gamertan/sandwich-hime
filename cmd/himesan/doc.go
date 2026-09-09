// SPDX-License-Identifier: AGPL-3.0-only

// Command himesan is the development-time compiler and tooling entry point for
// Sandwich Hime's HTML-first, ahead-of-time .sando templates.
//
// Hime-san generates deterministic, formatted .sando.go files beside their
// sources. Applications commit those generated files and deploy their ordinary
// Go program with the small Apache-2.0 sando runtime. The compiler, language
// server, and development supervisor are not production dependencies.
//
// # Install
//
// Install the current compiler release with the Go toolchain:
//
//	go install gamertan.com/sandwich-hime/cmd/himesan@latest
//
// The compiler and runtime have independent tags. For a consuming module, add
// the exact sando runtime release first, then install and pin the exact compiler
// version selected by that project's documentation. The module README records
// current release versions and the runtime-first installation sequence.
//
// # Generate and check
//
// Generate adjacent Go files for templates below the current module:
//
//	himesan generate ./...
//
// Check committed output without writing files:
//
//	himesan check ./...
//
// The shorter "gen" command aliases generate. The friendly "bless" command is
// a read-only alias for check. Both generate and check accept --json for one
// bounded machine-readable result.
//
// Generation is deterministic and replaces only compiler-owned output
// atomically. It does not edit handwritten Go files or go.mod. Check reports
// stale, missing, invalid, and orphaned generated output through the same
// compiler diagnostics used by generation.
//
// # Local development
//
// The optional development supervisor regenerates templates, builds the
// application's own net/http program, health-checks a new loopback candidate,
// and preserves the last healthy process when a candidate fails:
//
//	himesan dev [flags] [package] [-- app-args...]
//
// Its stable local proxy and browser diagnostics are development conveniences,
// not an application framework or production server. Projects may configure
// the supervisor with himesan.json or use generate, check, and ordinary Go
// tooling directly.
//
// # Editor integration
//
// The editor-neutral language server runs over standard input and output:
//
//	himesan lsp --stdio
//
// It analyzes saved sources and unsaved document overlays with the compiler's
// parser and context model. It does not generate files, execute project code,
// invoke Go, fetch modules, access the network, or start the development
// supervisor.
//
// # Operational contracts
//
// Use "himesan version --json" for compiler, runtime ABI, Go, and feature
// identity. The repository freezes the command surface, exit meanings,
// diagnostic codes, JSON schemas, and configuration schema in its contracts
// directory. Language syntax and contextual-safety rules are specified in
// SPEC.md and docs/THREAT_MODEL.md at the canonical repository.
package main
