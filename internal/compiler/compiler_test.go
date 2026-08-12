// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const profileSource = `<?sando go
package views

func Profile(name string, admin bool)
?>
<section class="profile" data-name="<?= name ?>">
  <h1><?= name ?></h1>
  <? if admin { ?>
    <strong>Admin</strong>
  <? } ?>
</section>
`

func TestCompileDeterministicContextAnnotatedBackend(t *testing.T) {
	t.Parallel()
	first, diagnostics := Compile("views/profile.sando", []byte(profileSource))
	assertNoErrorDiagnostics(t, diagnostics)
	second, secondDiagnostics := Compile("views/profile.sando", []byte(profileSource))
	assertNoErrorDiagnostics(t, secondDiagnostics)
	if !bytes.Equal(first.Code, second.Code) {
		t.Fatal("repeated compilation was not deterministic")
	}
	generated := string(first.Code)
	for _, expected := range []string{
		generatedPrefix,
		"// himesan:compiler ",
		"// himesan:runtime-abi sando.v1",
		"// himesan:source-sha256 ",
		"func Profile(name string, admin bool)",
		".WriteAttr(",
		".WriteText(",
		".WriteString(",
		"ComponentFunc(func(",
		"//line profile.sando:",
	} {
		if !strings.Contains(generated, expected) {
			t.Fatalf("generated code does not contain %q:\n%s", expected, generated)
		}
	}
	if strings.Contains(generated, "AGPL") || strings.Contains(generated, "SPDX-License-Identifier") {
		t.Fatal("generated application code inherited the compiler license header")
	}
}

func TestCommittedGoldenOutput(t *testing.T) {
	t.Parallel()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), "testdata", "golden", "basic.sando")
	wantPath := sourcePath + ".go"
	compiled, diagnostics := compileWithMapping(sourcePath, mustRead(t, sourcePath), "internal/compiler/testdata/golden/basic.sando")
	assertNoErrorDiagnostics(t, diagnostics)
	if want := mustRead(t, wantPath); !bytes.Equal(compiled.Code, want) {
		t.Fatalf("committed golden output is stale; run himesan generate\n--- got ---\n%s\n--- want ---\n%s", compiled.Code, want)
	}
}

func TestHeaderAllowsBOMWhitespaceAndGoLexicalDelimiters(t *testing.T) {
	t.Parallel()
	source := "\xef\xbb\xbf \r\n\t<?sando go\npackage views\nfunc Lexical(v struct { Tag string `json:\"?>\"` })\n?>\n<p><?= \"?>\" ?></p>"
	compiled, diagnostics := Compile("lexical.sando", []byte(source))
	assertNoErrorDiagnostics(t, diagnostics)
	if !strings.Contains(string(compiled.Code), `WriteText(`) || !strings.Contains(string(compiled.Code), `"?>"`) {
		t.Fatalf("lexically protected delimiter was not preserved:\n%s", compiled.Code)
	}
}

func TestHeaderTrailingLineCommentDoesNotConsumeSyntheticBody(t *testing.T) {
	t.Parallel()
	source := `<?sando go
package p
func Commented() // ?> remains inside the Go line comment
?>
<p>ok</p>`
	if _, diagnostics := Compile("commented.sando", []byte(source)); hasErrors(diagnostics) {
		t.Fatalf("trailing header comment failed: %v", diagnostics)
	}
}

func TestHeaderRequiresMarkerWhitespace(t *testing.T) {
	t.Parallel()
	_, diagnostics := Compile("bad.sando", []byte("<?sandogo\npackage p\nfunc Bad()\n?>"))
	assertDiagnosticCode(t, diagnostics, "HIM1105")
}

func TestGoReservedFunctionNamesAreRejected(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		packageName string
		function    string
	}{
		{packageName: "views", function: "init"},
		{packageName: "main", function: "main"},
	} {
		source := "<?sando go\npackage " + test.packageName + "\nfunc " + test.function + "()\n?>"
		_, diagnostics := Compile(test.function+".sando", []byte(source))
		assertDiagnosticCode(t, diagnostics, "HIM1123")
	}
}

func TestCRLFAndGeneratedSyntaxDiagnostics(t *testing.T) {
	t.Parallel()
	crlf := strings.ReplaceAll(simpleSource("CRLF", "snow 雪"), "\n", "\r\n")
	if _, diagnostics := Compile("crlf.sando", []byte(crlf)); hasErrors(diagnostics) {
		t.Fatalf("CRLF source failed: %v", diagnostics)
	}
	invalid := `<?sando go
package p
func Invalid(value bool)
?>
<? if value { ?>ok<? definitely-not-go ?><? } ?>`
	_, diagnostics := Compile("mapped.sando", []byte(invalid))
	assertDiagnosticCode(t, diagnostics, "HIM1401")
	for _, item := range diagnostics {
		if item.Code == "HIM1401" && (item.Line < 5 || item.Column < 1) {
			t.Fatalf("generated syntax diagnostic was not mapped to source: %+v", item)
		}
	}
}

func TestLineDirectivePathCannotInjectGeneratedGo(t *testing.T) {
	t.Parallel()
	path := "bad\ngo-build-injected.sando"
	compiled, diagnostics := Compile(path, []byte(simpleSource("SafePath", "safe")))
	assertNoErrorDiagnostics(t, diagnostics)
	generated := string(compiled.Code)
	if strings.Contains(generated, "//line go-build-injected") || !strings.Contains(generated, "%0A") {
		t.Fatalf("unsafe source path was not encoded in //line directive:\n%s", generated)
	}
}

func TestContextRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		code string
	}{
		{name: "dynamic tag", body: `<<?= "div" ?>>ok</div>`, code: "HIM1303"},
		{name: "unquoted attribute", body: `<p title=<?= "x" ?>></p>`, code: "HIM1328"},
		{name: "event attribute", body: `<p onclick="fixed"></p>`, code: "HIM1340"},
		{name: "component in attribute", body: `<p title="<?~ Child() ?>"></p>`, code: "HIM1302"},
		{name: "component in textarea", body: `<textarea><?~ Child() ?></textarea>`, code: "HIM1302"},
		{name: "unbalanced", body: `<div><span></div>`, code: "HIM1352"},
		{name: "unfinished", body: `<div>`, code: "HIM1311"},
		{name: "self closing nonvoid", body: `<div/>`, code: "HIM1354"},
		{name: "foreign SVG", body: `<svg></svg>`, code: "HIM1355"},
		{name: "ambiguous noscript", body: `<noscript>fallback</noscript>`, code: "HIM1356"},
		{name: "script escaped state", body: "<script><!--<script></script>\n<?= css ?>\n<!--\n</script>\n-->", code: "HIM1357"},
		{name: "script escaped state split by template comment", body: `<script><!<?# emits nothing ?>--alert(1)</script>`, code: "HIM1357"},
		{name: "dynamic iframe raw text", body: `<iframe><?= css ?></iframe>`, code: "HIM1303"},
		{name: "dynamic style attribute", body: `<p style="<?= css ?>"></p>`, code: "HIM1343"},
		{name: "dynamic srcdoc attribute", body: `<iframe srcdoc="<?= css ?>"></iframe>`, code: "HIM1345"},
		{name: "dynamic srcset attribute", body: `<img srcset="<?= css ?>">`, code: "HIM1345"},
		{name: "meta refresh", body: `<meta content="0;url=javascript:alert(1)" http-equiv="refresh">`, code: "HIM1346"},
		{name: "dynamic meta http equiv", body: `<meta http-equiv="<?= css ?>" content="safe">`, code: "HIM1346"},
		{name: "duplicate attribute", body: `<meta http-equiv="refresh" http-equiv="safe">`, code: "HIM1347"},
		{name: "dangerous static URL", body: `<a href="javascript&#58;alert(1)">x</a>`, code: "HIM1344"},
		{name: "ambiguous URL pieces", body: `<a href="<?= scheme ?>:payload">x</a>`, code: "HIM1341"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "<?sando go\npackage p\nfunc Example(css, scheme string)\n?>\n" + test.body
			_, diagnostics := Compile(test.name+".sando", []byte(source))
			assertDiagnosticCode(t, diagnostics, test.code)
		})
	}
}

func TestHTMLCommentSyntaxInOtherRawTextStatesRemainsSupported(t *testing.T) {
	t.Parallel()
	source := `<?sando go
package p
func RawText()
?>
<style><!-- .old-browser { display: none } --></style>
<textarea><!-- literal text --></textarea>
<iframe><!-- literal text --></iframe>`
	_, diagnostics := Compile("raw-text.sando", []byte(source))
	assertNoErrorDiagnostics(t, diagnostics)
}

func TestTextareaExpressionUsesTextEscaping(t *testing.T) {
	t.Parallel()
	source := `<?sando go
package p
func Field(value string)
?>
<textarea><?= value ?></textarea>`
	compiled, diagnostics := Compile("field.sando", []byte(source))
	assertNoErrorDiagnostics(t, diagnostics)
	if !strings.Contains(string(compiled.Code), ".WriteRCDATA(") {
		t.Fatalf("textarea expression did not use dedicated RCDATA escaping:\n%s", compiled.Code)
	}
}

func TestSupportedURLAndRawTextContexts(t *testing.T) {
	t.Parallel()
	source := `<?sando go
package p
import "gamertan.com/sandwich-hime/sando"
func Safe(id string, js sando.TrustedJS, css sando.TrustedCSS)
?>
<a href="/item/<?= id ?>">relative</a>
<a href="<?= sando.TrustURL("https://example.test/") ?>">trusted</a>
<script><?= js ?></script>
<style><?= css ?></style>`
	compiled, diagnostics := Compile("safe.sando", []byte(source))
	assertNoErrorDiagnostics(t, diagnostics)
	assertDiagnosticCode(t, diagnostics, "HIM1901")
	assertDiagnosticCode(t, diagnostics, "HIM1902")
	assertDiagnosticCode(t, diagnostics, "HIM1903")
	generated := string(compiled.Code)
	for _, helper := range []string{".WriteURL(", ".WriteJS(", ".WriteCSS("} {
		if !strings.Contains(generated, helper) {
			t.Fatalf("generated code does not contain %s:\n%s", helper, generated)
		}
	}
}

func TestTrustWarningsDoNotFailGenerateOrCheck(t *testing.T) {
	t.Parallel()
	directory := resolvedTempDir(t)
	path := filepath.Join(directory, "trusted.sando")
	source := `<?sando go
package p
import "gamertan.com/sandwich-hime/sando"
func Trusted()
?>
<?= sando.TrustHTML("<b>reviewed</b>") ?>`
	mustWrite(t, path, source)
	generated, err := Generate(context.Background(), []string{path})
	if err != nil {
		t.Fatalf("warning unexpectedly failed generation: %v", err)
	}
	assertDiagnosticCode(t, generated.Diagnostics, "HIM1901")
	checked, err := Check(context.Background(), []string{path})
	if err != nil {
		t.Fatalf("warning unexpectedly failed check: %v", err)
	}
	assertDiagnosticCode(t, checked.Diagnostics, "HIM1901")
}

func TestGenerateCheckAndUnchangedTimestamp(t *testing.T) {
	t.Parallel()
	directory := resolvedTempDir(t)
	path := filepath.Join(directory, "hello.sando")
	mustWrite(t, path, `<?sando go
package demo
func Hello(name string)
?>
<p>Hello <?= name ?></p>`)
	first, err := Generate(context.Background(), []string{directory})
	if err != nil {
		t.Fatalf("Generate: %v (%v)", err, first.Diagnostics)
	}
	if first.Changed != 1 || first.Discovered != 1 {
		t.Fatalf("unexpected first result: %+v", first)
	}
	outputPath := path + ".go"
	before, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(context.Background(), []string{directory})
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if second.Unchanged != 1 || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("unchanged generation changed output metadata: before=%v after=%v result=%+v", before.ModTime(), after.ModTime(), second)
	}
	checked, err := Check(context.Background(), []string{directory})
	if err != nil || checked.Unchanged != 1 {
		t.Fatalf("fresh check failed: result=%+v err=%v", checked, err)
	}
	mustWrite(t, path, strings.ReplaceAll(string(mustRead(t, path)), "Hello", "Welcome"))
	stale, err := Check(context.Background(), []string{directory})
	if err == nil || stale.Stale != 1 {
		t.Fatalf("stale check did not fail: result=%+v err=%v", stale, err)
	}
	assertDiagnosticCode(t, stale.Diagnostics, "HIM2204")
}

func TestCompileFailurePreservesEveryLastGoodOutput(t *testing.T) {
	t.Parallel()
	directory := resolvedTempDir(t)
	firstPath := filepath.Join(directory, "first.sando")
	secondPath := filepath.Join(directory, "second.sando")
	mustWrite(t, firstPath, simpleSource("First", "first"))
	mustWrite(t, secondPath, simpleSource("Second", "second"))
	if _, err := Generate(context.Background(), []string{directory}); err != nil {
		t.Fatal(err)
	}
	firstLastGood := mustRead(t, firstPath+".go")
	secondLastGood := mustRead(t, secondPath+".go")
	mustWrite(t, firstPath, simpleSource("First", "changed"))
	mustWrite(t, secondPath, "<?sando go\npackage demo\nfunc Second(\n?>")
	if result, err := Generate(context.Background(), []string{directory}); err == nil {
		t.Fatalf("invalid batch unexpectedly generated: %+v", result)
	}
	if !bytes.Equal(firstLastGood, mustRead(t, firstPath+".go")) || !bytes.Equal(secondLastGood, mustRead(t, secondPath+".go")) {
		t.Fatal("a last-good output changed after batch compilation failed")
	}
}

func TestGenerateRefusesUnownedOutput(t *testing.T) {
	t.Parallel()
	directory := resolvedTempDir(t)
	path := filepath.Join(directory, "page.sando")
	mustWrite(t, path, simpleSource("Page", "page"))
	mustWrite(t, path+".go", "package demo\n")
	result, err := Generate(context.Background(), []string{path})
	if err == nil {
		t.Fatalf("Generate overwrote an unowned output: %+v", result)
	}
	assertDiagnosticCode(t, result.Diagnostics, "HIM2104")
	if got := string(mustRead(t, path+".go")); got != "package demo\n" {
		t.Fatalf("unowned output changed to %q", got)
	}
}

func TestDiscoveryBoundariesAndExplicitNestedFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires additional Windows privileges")
	}
	t.Parallel()
	directory := resolvedTempDir(t)
	mustWrite(t, filepath.Join(directory, "root.sando"), simpleSource("Root", "root"))
	mustWrite(t, filepath.Join(directory, ".git", "ignored.sando"), simpleSource("Git", "git"))
	mustWrite(t, filepath.Join(directory, "vendor", "ignored.sando"), simpleSource("Vendor", "vendor"))
	mustWrite(t, filepath.Join(directory, "other-repository", ".git"), "gitdir: elsewhere\n")
	mustWrite(t, filepath.Join(directory, "other-repository", "ignored.sando"), simpleSource("OtherRepository", "other"))
	nested := filepath.Join(directory, "nested")
	mustWrite(t, filepath.Join(nested, "go.mod"), "module nested.test\n")
	nestedSource := filepath.Join(nested, "nested.sando")
	mustWrite(t, nestedSource, simpleSource("Nested", "nested"))
	discovered, diagnostics := discover(context.Background(), []string{directory})
	assertNoErrorDiagnostics(t, diagnostics)
	if len(discovered) != 1 || filepath.Base(discovered[0]) != "root.sando" {
		t.Fatalf("unexpected discovery result: %v", discovered)
	}
	explicit, diagnostics := discover(context.Background(), []string{nestedSource})
	assertNoErrorDiagnostics(t, diagnostics)
	if len(explicit) != 1 || explicit[0] != nestedSource {
		t.Fatalf("explicit nested source was not accepted: %v", explicit)
	}
	symlink := filepath.Join(directory, "linked")
	if err := os.Symlink(nested, symlink); err != nil {
		t.Fatal(err)
	}
	_, diagnostics = discover(context.Background(), []string{filepath.Join(symlink, "nested.sando")})
	assertDiagnosticCode(t, diagnostics, "HIM2003")
}

func TestCheckRejectsSymlinkOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires additional Windows privileges")
	}
	t.Parallel()
	directory := resolvedTempDir(t)
	path := filepath.Join(directory, "page.sando")
	mustWrite(t, path, simpleSource("Page", "page"))
	target := filepath.Join(directory, "handwritten.go")
	mustWrite(t, target, "package demo\n")
	if err := os.Symlink(target, path+".go"); err != nil {
		t.Fatal(err)
	}
	result, err := Check(context.Background(), []string{path})
	if err == nil {
		t.Fatalf("symlink output unexpectedly passed check: %+v", result)
	}
	assertDiagnosticCode(t, result.Diagnostics, "HIM2205")
}

func TestGenerateReportsReadOnlyDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory mode test")
	}
	t.Parallel()
	directory := resolvedTempDir(t)
	path := filepath.Join(directory, "page.sando")
	mustWrite(t, path, simpleSource("Page", "page"))
	if err := os.Chmod(directory, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o755) })
	result, err := Generate(context.Background(), []string{path})
	if err == nil {
		t.Fatalf("read-only directory unexpectedly generated: %+v", result)
	}
	assertDiagnosticCode(t, result.Diagnostics, "HIM2110")
}

func TestNestedNonRegularGoModIsBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires additional Windows privileges")
	}
	t.Parallel()
	directory := resolvedTempDir(t)
	target := filepath.Join(directory, "actual.mod")
	mustWrite(t, target, "module nested.test\n")
	nested := filepath.Join(directory, "nested")
	mustWrite(t, filepath.Join(nested, "hidden.sando"), simpleSource("Hidden", "hidden"))
	if err := os.Symlink(target, filepath.Join(nested, "go.mod")); err != nil {
		t.Fatal(err)
	}
	discovered, diagnostics := discover(context.Background(), []string{directory})
	if len(discovered) != 0 {
		t.Fatalf("traversed nested module with symlink go.mod: %v", discovered)
	}
	assertDiagnosticCode(t, diagnostics, "HIM2008")
}

func TestModuleRelativeLineMappings(t *testing.T) {
	t.Parallel()
	directory := resolvedTempDir(t)
	mustWrite(t, filepath.Join(directory, "go.mod"), "module example.test/app\n")
	path := filepath.Join(directory, "views", "card.sando")
	mustWrite(t, path, simpleSource("Card", "card"))
	if _, err := Generate(context.Background(), []string{directory}); err != nil {
		t.Fatal(err)
	}
	generated := string(mustRead(t, path+".go"))
	if !strings.Contains(generated, "//line views/card.sando:") {
		t.Fatalf("line mapping was not module-relative:\n%s", generated)
	}
	if strings.Contains(generated, filepath.ToSlash(directory)) {
		t.Fatal("generated output contains an absolute build-machine path")
	}
}

func TestStaticComponentCycle(t *testing.T) {
	t.Parallel()
	directory := resolvedTempDir(t)
	mustWrite(t, filepath.Join(directory, "a.sando"), `<?sando go
package demo
func A()
?>
<?~ B() ?>`)
	mustWrite(t, filepath.Join(directory, "b.sando"), `<?sando go
package demo
func B()
?>
<?~ A() ?>`)
	result, err := Generate(context.Background(), []string{directory})
	if err == nil {
		t.Fatalf("component cycle unexpectedly generated: %+v", result)
	}
	assertDiagnosticCode(t, result.Diagnostics, "HIM1501")
	if _, statErr := os.Stat(filepath.Join(directory, "a.sando.go")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("cycle wrote output: %v", statErr)
	}
}

func TestDuplicateComponentNamesFailBeforeWrites(t *testing.T) {
	t.Parallel()
	directory := resolvedTempDir(t)
	mustWrite(t, filepath.Join(directory, "one.sando"), simpleSource("Duplicate", "one"))
	mustWrite(t, filepath.Join(directory, "two.sando"), simpleSource("Duplicate", "two"))
	result, err := Generate(context.Background(), []string{directory})
	if err == nil {
		t.Fatalf("duplicate component names unexpectedly generated: %+v", result)
	}
	assertDiagnosticCode(t, result.Diagnostics, "HIM1500")
}

func simpleSource(component, text string) string {
	return "<?sando go\npackage demo\nfunc " + component + "()\n?>\n<p>" + text + "</p>\n"
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func resolvedTempDir(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func assertNoErrorDiagnostics(t *testing.T, diagnostics []Diagnostic) {
	t.Helper()
	if hasErrors(diagnostics) {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func assertDiagnosticCode(t *testing.T, diagnostics []Diagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostic %s not found in %v", code, diagnostics)
}
