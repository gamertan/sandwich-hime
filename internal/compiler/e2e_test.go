// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
