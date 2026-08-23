// SPDX-License-Identifier: AGPL-3.0-only
//go:build himesan_browser_evidence

package devserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gamertan.com/sandwich-hime/internal/compiler"
)

const realBrowserTimeout = 20 * time.Second

func TestRealBrowserDevelopmentClient(t *testing.T) {
	chrome := os.Getenv("HIMESAN_CHROME")
	if chrome == "" {
		t.Fatal("HIMESAN_CHROME must name the reviewed Chrome or Chromium executable")
	}
	if info, err := os.Stat(chrome); err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		t.Fatalf("HIMESAN_CHROME is not an executable regular file: %q", chrome)
	}

	t.Run("diagnostic overlay and CSP", func(t *testing.T) {
		harness := newRealBrowserHarness(t)
		document := runRealBrowser(t, chrome, harness.url("/"), "browser evidence diagnostic", func() {
			waitForRealBrowserSubscriber(t, harness.hub)
			harness.hub.publish(Event{
				Type: "diagnostic", Phase: "generate", Message: "browser evidence diagnostic",
				Diagnostics: []Diagnostic{{Path: "views/home.sando", Line: 7, Column: 3, Code: "HIM1300", Message: "deliberate evidence fixture"}},
			})
		})
		for _, want := range []string{"id=\"__himesan_overlay\"", "browser evidence diagnostic", "views/home.sando:7:3 [HIM1300]"} {
			if !strings.Contains(document, want) {
				t.Fatalf("browser DOM lacks %q:\n%s", want, document)
			}
		}
	})

	t.Run("reload", func(t *testing.T) {
		harness := newRealBrowserHarness(t)
		document := runRealBrowser(t, chrome, harness.url("/"), "version two", func() {
			waitForRealBrowserSubscriber(t, harness.hub)
			harness.version.Store(2)
			harness.hub.publish(Event{Type: "reload", Phase: "serve", Message: "healthy application activated"})
		})
		if !strings.Contains(document, `<main id="page-version">version two</main>`) {
			t.Fatalf("browser did not reload the selected document:\n%s", document)
		}
	})

	t.Run("fragment and API exclusions", func(t *testing.T) {
		harness := newRealBrowserHarness(t)
		for _, path := range []string{"/fragment", "/api"} {
			document := runRealBrowser(t, chrome, harness.url(path), "", nil)
			if strings.Contains(document, "data-himesan-reload") || strings.Contains(document, "__himesan_overlay") {
				t.Fatalf("development client leaked into %s:\n%s", path, document)
			}
		}
	})

	t.Run("generated document parsing", func(t *testing.T) {
		root := writeGeneratedBrowserApplication(t)
		cfg := DefaultConfig()
		cfg.ProxyAddress = "127.0.0.1:0"
		cfg.HealthPath = "/healthz"
		supervisor, err := New(Options{
			RootDir: root,
			Config:  cfg,
			Generate: func(ctx context.Context) error {
				_, generateErr := compiler.Generate(ctx, []string{root})
				return generateErr
			},
			CacheDir:        filepath.Join(t.TempDir(), "cache"),
			PollInterval:    30 * time.Second,
			Debounce:        25 * time.Millisecond,
			BuildTimeout:    30 * time.Second,
			StartupTimeout:  integrationCandidateStartupTimeout,
			ShutdownTimeout: 2 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		runResult := make(chan error, 1)
		go func() { runResult <- supervisor.Run(ctx) }()
		proxyAddress := waitForProxyAddress(t, supervisor)
		waitForBody(t, "http://"+proxyAddress+"/", "generated-browser-document")
		document := runRealBrowser(t, chrome, "http://"+proxyAddress+"/", "generated-browser-document", nil)
		for _, want := range []string{
			`<main id="generated-browser-document"`,
			`<h1 id="title">&lt;unsafe&gt; &amp; "quoted"</h1>`,
			`<textarea id="note">&lt;/textarea&gt;&lt;script id="attacker"&gt;window.evidenceFailed=true&lt;/script&gt;</textarea>`,
			`<a id="destination" href="/safe?q=a&amp;b=c">Open</a>`,
			`<table id="table"><tbody><tr><td id="cell">&lt;unsafe&gt; &amp; "quoted"</td></tr></tbody></table>`,
		} {
			if !strings.Contains(document, want) {
				t.Fatalf("generated browser DOM lacks %q:\n%s", want, document)
			}
		}
		if strings.Contains(document, `<script id="attacker">`) {
			t.Fatalf("hostile RCDATA became executable structure:\n%s", document)
		}
		cancel()
		select {
		case err := <-runResult:
			if err != nil {
				t.Fatalf("stop generated-document supervisor: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("generated-document supervisor did not stop")
		}
	})
}

func writeGeneratedBrowserApplication(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve test-owned browser fixture: %v", err)
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate browser evidence source")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	files := map[string]string{
		"go.mod": fmt.Sprintf("module example.test/himesan-browser-evidence\n\ngo 1.25\n\nrequire gamertan.com/sandwich-hime/sando v0.0.0\nreplace gamertan.com/sandwich-hime/sando => %s/sando\n", filepath.ToSlash(repositoryRoot)),
		"page.sando": `<?sando go
package main

func Page(title string, note string, destination string)
?>
<!doctype html>
<html><head><title><?= title ?></title></head><body>
<main id="generated-browser-document" data-note="<?= note ?>">
<h1 id="title"><?= title ?></h1>
<textarea id="note"><?= note ?></textarea>
<a id="destination" href="<?= destination ?>">Open</a>
<table id="table"><tbody><tr><td id="cell"><?= title ?></td></tr></tbody></table>
</main>
</body></html>
`,
		"main.go": `package main

import (
	"net/http"
	"os"

	"gamertan.com/sandwich-hime/sando"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := sando.Render(request.Context(), w, Page(
			` + "`<unsafe> & \"quoted\"`" + `,
			` + "`</textarea><script id=\"attacker\">window.evidenceFailed=true</script>`" + `,
			"/safe?q=a&b=c",
		)); err != nil {
			panic(err)
		}
	})
	if err := http.ListenAndServe(os.Getenv("HIMESAN_LISTEN_ADDR"), mux); err != nil {
		panic(err)
	}
}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := compiler.Generate(context.Background(), []string{root}); err != nil {
		t.Fatalf("generate browser evidence fixture: %v", err)
	}
	return root
}

type realBrowserHarness struct {
	hub      *eventHub
	version  atomic.Int32
	upstream *httptest.Server
	proxy    *httptest.Server
}

func newRealBrowserHarness(t *testing.T) *realBrowserHarness {
	t.Helper()
	harness := &realBrowserHarness{hub: newEventHub()}
	harness.version.Store(1)
	harness.upstream = httptest.NewServer(http.HandlerFunc(harness.serveApplication))
	development := newDevelopmentProxy(harness.hub)
	upstreamURL, err := url.Parse(harness.upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := development.setTarget(upstreamURL.Host); err != nil {
		t.Fatal(err)
	}
	harness.proxy = httptest.NewServer(development)
	proxyURL, err := url.Parse(harness.proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := development.setAuthority(proxyURL.Host); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		harness.hub.close()
		harness.proxy.Close()
		harness.upstream.Close()
	})
	return harness
}

func (h *realBrowserHarness) url(path string) string {
	return h.proxy.URL + path
}

func (h *realBrowserHarness) serveApplication(w http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/":
		version := "one"
		if h.version.Load() == 2 {
			version = "two"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'none'; script-src-elem 'none'; connect-src 'none'")
		fmt.Fprintf(w, `<!doctype html><html><body><main id="page-version">version %s</main></body></html>`, version)
	case "/fragment":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<p id="fragment">fragment only</p>`)
	case "/api":
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"kind":"api","ok":true}`)
	default:
		http.NotFound(w, request)
	}
}

func runRealBrowser(t *testing.T, chrome, target, want string, afterStart func()) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), realBrowserTimeout)
	defer cancel()
	toBrowserRead, toBrowserWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fromBrowserRead, fromBrowserWrite, err := os.Pipe()
	if err != nil {
		_ = toBrowserRead.Close()
		_ = toBrowserWrite.Close()
		t.Fatal(err)
	}
	defer toBrowserWrite.Close()
	defer fromBrowserRead.Close()

	stderrPath := filepath.Join(t.TempDir(), "chrome.stderr")
	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		_ = stderr.Close()
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, chrome,
		"--headless=new",
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-default-apps",
		"--disable-sync",
		"--metrics-recording-only",
		"--no-first-run",
		"--no-default-browser-check",
		"--user-data-dir="+t.TempDir(),
		"--remote-debugging-pipe",
		"about:blank",
	)
	command.ExtraFiles = []*os.File{toBrowserRead, fromBrowserWrite}
	command.Stdout = devNull
	command.Stderr = stderr
	command.WaitDelay = 2 * time.Second
	if err := command.Start(); err != nil {
		_ = toBrowserRead.Close()
		_ = fromBrowserWrite.Close()
		_ = stderr.Close()
		_ = devNull.Close()
		t.Fatalf("start real browser: %v", err)
	}
	_ = toBrowserRead.Close()
	_ = fromBrowserWrite.Close()
	client := &devToolsPipe{reader: bufio.NewReader(fromBrowserRead), writer: toBrowserWrite}
	targetResult := client.call(t, "", "Target.createTarget", map[string]any{"url": target})
	var created struct {
		TargetID string `json:"targetId"`
	}
	if err := json.Unmarshal(targetResult, &created); err != nil || created.TargetID == "" {
		t.Fatalf("decode Chrome target: %v", err)
	}
	attachResult := client.call(t, "", "Target.attachToTarget", map[string]any{"targetId": created.TargetID, "flatten": true})
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(attachResult, &attached); err != nil || attached.SessionID == "" {
		t.Fatalf("decode Chrome session: %v", err)
	}
	client.call(t, attached.SessionID, "Runtime.enable", map[string]any{})
	waitForBrowserDOM(t, client, attached.SessionID, "")
	if afterStart != nil {
		afterStart()
	}
	document := waitForBrowserDOM(t, client, attached.SessionID, want)
	_ = client.callIgnoringClose("", "Browser.close", map[string]any{})
	waitResult := make(chan error, 1)
	go func() { waitResult <- command.Wait() }()
	select {
	case err := <-waitResult:
		if err != nil && !errors.Is(err, exec.ErrWaitDelay) {
			_ = stderr.Close()
			contents, _ := os.ReadFile(stderrPath)
			t.Fatalf("real browser failed: %v; stderr=%s", err, boundedBrowserOutput(string(contents)))
		}
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("real browser did not exit after Browser.close")
	}
	_ = stderr.Close()
	_ = devNull.Close()
	return document
}

type devToolsPipe struct {
	reader *bufio.Reader
	writer io.Writer
	nextID int
}

type devToolsEnvelope struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *devToolsPipe) call(t *testing.T, session, method string, parameters map[string]any) json.RawMessage {
	t.Helper()
	c.nextID++
	request := map[string]any{"id": c.nextID, "method": method, "params": parameters}
	if session != "" {
		request["sessionId"] = session
	}
	contents, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	contents = append(contents, 0)
	if _, err := c.writer.Write(contents); err != nil {
		t.Fatalf("write Chrome DevTools request: %v", err)
	}
	for {
		message, err := c.reader.ReadBytes(0)
		if err != nil {
			t.Fatalf("read Chrome DevTools response: %v", err)
		}
		if len(message) > 4<<20 {
			t.Fatal("Chrome DevTools response exceeds 4 MiB")
		}
		var response devToolsEnvelope
		if err := json.Unmarshal(bytes.TrimSuffix(message, []byte{0}), &response); err != nil {
			t.Fatalf("decode Chrome DevTools response: %v", err)
		}
		if response.ID != c.nextID {
			continue
		}
		if response.Error != nil {
			t.Fatalf("Chrome DevTools %s failed (%d): %s", method, response.Error.Code, response.Error.Message)
		}
		return response.Result
	}
}

func (c *devToolsPipe) callIgnoringClose(session, method string, parameters map[string]any) error {
	c.nextID++
	request := map[string]any{"id": c.nextID, "method": method, "params": parameters}
	if session != "" {
		request["sessionId"] = session
	}
	contents, err := json.Marshal(request)
	if err != nil {
		return err
	}
	_, err = c.writer.Write(append(contents, 0))
	return err
}

func waitForBrowserDOM(t *testing.T, client *devToolsPipe, session, want string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		result := client.call(t, session, "Runtime.evaluate", map[string]any{
			"expression":    "document.documentElement && document.documentElement.outerHTML",
			"returnByValue": true,
		})
		var evaluated struct {
			Result struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"result"`
		}
		if err := json.Unmarshal(result, &evaluated); err == nil && evaluated.Result.Type == "string" {
			last = evaluated.Result.Value
			if last != "" && (want == "" || strings.Contains(last, want)) {
				return last
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("browser DOM did not contain %q; last DOM:\n%s", want, last)
	return ""
}

func waitForRealBrowserSubscriber(t *testing.T, hub *eventHub) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.Lock()
		count := len(hub.subscribers)
		hub.mu.Unlock()
		if count != 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("real browser did not establish the same-origin event stream")
}

func boundedBrowserOutput(value string) string {
	const limit = 4096
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n[Chrome output truncated]"
}
