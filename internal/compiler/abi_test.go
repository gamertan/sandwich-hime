// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedCodeRequiresVersionedRuntimeABIMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporary-module ABI compilation in short mode")
	}
	t.Parallel()

	directory := resolvedTempDir(t)
	templatePath := filepath.Join(directory, "page.sando")
	mustWrite(t, templatePath, "<?sando go\npackage generated\nfunc Page()\n?>")
	result, err := Generate(context.Background(), []string{templatePath})
	if err != nil {
		t.Fatalf("Generate: %v (%v)", err, result.Diagnostics)
	}
	generated := string(mustRead(t, templatePath+".go"))
	if !strings.Contains(generated, ".ABISandoV1") {
		t.Fatalf("generated code does not require the sando.v1 marker:\n%s", generated)
	}

	mustWrite(t, filepath.Join(directory, "go.mod"), `module example.test/abi

go 1.25

require gamertan.com/sandwich-hime/sando v0.0.0

replace gamertan.com/sandwich-hime/sando => ./fake-sando
`)
	mustWrite(t, filepath.Join(directory, "fake-sando", "go.mod"), `module gamertan.com/sandwich-hime/sando

go 1.25
`)
	mustWrite(t, filepath.Join(directory, "fake-sando", "component.go"), `package sando

import (
	"context"
	"io"
)

const ABI = "sando.incompatible"

type Component interface { Render(context.Context, io.Writer) error }
type ComponentFunc func(context.Context, io.Writer) error
func (f ComponentFunc) Render(ctx context.Context, w io.Writer) error { return f(ctx, w) }
`)

	command := exec.Command("go", "test", "./...")
	command.Dir = directory
	command.Env = append(os.Environ(), "GOWORK=off")
	output, buildErr := command.CombinedOutput()
	if buildErr == nil {
		t.Fatalf("generated code compiled against an incompatible runtime:\n%s", output)
	}
	if !strings.Contains(string(output), "undefined: __himesan_sando.ABISandoV1") {
		t.Fatalf("incompatible runtime failed for an unexpected reason: %v\n%s", buildErr, output)
	}
}
