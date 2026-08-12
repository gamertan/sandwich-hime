// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDevelopmentAndBetaBinariesShareGeneratedOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compiler-binary integration in short mode")
	}
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	developmentBinary := buildHimesanBinary(t, repositoryRoot, "himesan-development", "")
	betaVersion := "v1.0.0-beta.1"
	betaBinary := buildHimesanBinary(t, repositoryRoot, "himesan-beta", "-X gamertan.com/sandwich-hime/internal/version.Compiler="+betaVersion)

	if got := compilerVersionFromBinary(t, developmentBinary, repositoryRoot); got != "0.1.0-dev" {
		t.Fatalf("development binary version = %q, want 0.1.0-dev", got)
	}
	if got := compilerVersionFromBinary(t, betaBinary, repositoryRoot); got != betaVersion {
		t.Fatalf("beta binary version = %q, want %q", got, betaVersion)
	}

	directory := resolvedTempDir(t)
	mustWrite(t, filepath.Join(directory, "go.mod"), "module example.test/provenance\n\ngo 1.25\n")
	sourcePath := filepath.Join(directory, "hello.sando")
	mustWrite(t, sourcePath, `<?sando go
package views
func Hello(name string)
?>
<p>Hello <?= name ?></p>
`)
	runHimesanBinary(t, developmentBinary, directory, "generate", "hello.sando")
	outputPath := sourcePath + ".go"
	developmentOutput := mustRead(t, outputPath)
	if !bytes.Contains(developmentOutput, []byte("// himesan:compiler 0.1.0-dev\n")) {
		t.Fatalf("development compiler did not record honest provenance:\n%s", developmentOutput)
	}

	runHimesanBinary(t, betaBinary, directory, "check", "hello.sando")
	runHimesanBinary(t, betaBinary, directory, "generate", "hello.sando")
	if after := mustRead(t, outputPath); !bytes.Equal(after, developmentOutput) {
		t.Fatalf("beta compiler rewrote otherwise-current development provenance\n--- before ---\n%s\n--- after ---\n%s", developmentOutput, after)
	}

	if err := os.Remove(outputPath); err != nil {
		t.Fatal(err)
	}
	runHimesanBinary(t, betaBinary, directory, "generate", "hello.sando")
	betaOutput := mustRead(t, outputPath)
	if !bytes.Contains(betaOutput, []byte("// himesan:compiler "+betaVersion+"\n")) {
		t.Fatalf("beta compiler did not record honest provenance:\n%s", betaOutput)
	}
	runHimesanBinary(t, developmentBinary, directory, "check", "hello.sando")
	runHimesanBinary(t, developmentBinary, directory, "generate", "hello.sando")
	if after := mustRead(t, outputPath); !bytes.Equal(after, betaOutput) {
		t.Fatalf("development compiler rewrote otherwise-current beta provenance\n--- before ---\n%s\n--- after ---\n%s", betaOutput, after)
	}

	tampered := bytes.Replace(betaOutput, []byte(".WriteText("), []byte(".WriteAttr("), 1)
	if bytes.Equal(tampered, betaOutput) {
		t.Fatal("semantic tamper did not find generated WriteText call")
	}
	if err := os.WriteFile(outputPath, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(betaBinary, "check", "hello.sando")
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "HIM2204") {
		t.Fatalf("beta check accepted semantic generated-code tamper: err=%v\n%s", err, output)
	}
}

func TestGeneratedOutputCompilesInTemporaryModule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporary-module compilation in short mode")
	}
	t.Parallel()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	runtimeRoot := filepath.Join(repositoryRoot, "sando")
	directory := resolvedTempDir(t)
	goMod := "module example.test/generated\n\ngo 1.25\n\nrequire gamertan.com/sandwich-hime/sando v0.0.0\n\nreplace gamertan.com/sandwich-hime/sando => " + filepath.ToSlash(runtimeRoot) + "\n"
	mustWrite(t, filepath.Join(directory, "go.mod"), goMod)
	mustWrite(t, filepath.Join(directory, "view.go"), `package generated

import "gamertan.com/sandwich-hime/sando"

type View struct {
	Name string
	URL string
	JS sando.TrustedJS
	HTML sando.TrustedHTML
}
`)
	mustWrite(t, filepath.Join(directory, "render_test.go"), `package generated

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"gamertan.com/sandwich-hime/sando"
)

func TestRCDATACannotBeBypassedByTrustedHTML(t *testing.T) {
	var output bytes.Buffer
	view := View{Name: "title", URL: "/", JS: sando.TrustJS(""), HTML: sando.TrustHTML("</textarea><script>bad()</script>")}
	if err := sando.Render(context.Background(), &output, Page(view)); err != nil { t.Fatal(err) }
	if strings.Contains(output.String(), "</textarea><script>") { t.Fatalf("RCDATA boundary escaped: %s", output.String()) }
}
`)
	templatePath := filepath.Join(directory, "page.sando")
	mustWrite(t, templatePath, `<?sando go
package generated
func Page(view View)
?>
<!doctype html>
<html><body>
<a href="<?= view.URL ?>"><?= view.Name ?></a>
<script><?= view.JS ?></script>
<textarea><?= view.HTML ?></textarea>
</body></html>`)
	result, err := Generate(context.Background(), []string{templatePath})
	if err != nil {
		t.Fatalf("Generate failed: %v (%v)", err, result.Diagnostics)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = directory
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generated temporary module did not compile: %v\n%s\n--- generated ---\n%s", err, output, mustRead(t, templatePath+".go"))
	}
	if strings.Contains(string(mustRead(t, templatePath+".go")), repositoryRoot) {
		t.Fatal("generated output leaked the compiler checkout path")
	}
}

func buildHimesanBinary(t *testing.T, repositoryRoot, name, linkerFlags string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	arguments := []string{"build", "-trimpath"}
	if linkerFlags != "" {
		arguments = append(arguments, "-ldflags", linkerFlags)
	}
	arguments = append(arguments, "-o", path, "./cmd/himesan")
	command := exec.Command("go", arguments...)
	command.Dir = repositoryRoot
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
	return path
}

func compilerVersionFromBinary(t *testing.T, binary, directory string) string {
	t.Helper()
	output := runHimesanBinary(t, binary, directory, "version", "--json")
	var information struct {
		Compiler string `json:"compiler"`
	}
	if err := json.Unmarshal(output, &information); err != nil {
		t.Fatalf("decode compiler version from %s: %v\n%s", binary, err, output)
	}
	return information.Compiler
}

func runHimesanBinary(t *testing.T, binary, directory string, arguments ...string) []byte {
	t.Helper()
	command := exec.Command(binary, arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", binary, strings.Join(arguments, " "), err, output)
	}
	return output
}
