// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestInjectDevelopmentClientAndCSP(t *testing.T) {
	t.Parallel()
	body := "<!doctype html><html><body><h1>Hello</h1></body></html>"
	request := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	request.Header.Set("Sec-Fetch-Dest", "document")
	response := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       request,
	}
	response.Header.Set("Content-Type", "text/html; charset=utf-8")
	response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	response.Header.Set("ETag", `"old"`)
	response.Header.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; script-src-elem 'none'")

	if err := injectDevelopmentClient(response); err != nil {
		t.Fatalf("injectDevelopmentClient() error = %v", err)
	}
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), string(reloadClientTag)) {
		t.Fatalf("injected body does not contain reload client: %s", got)
	}
	if strings.Index(string(got), string(reloadClientTag)) > strings.Index(string(got), "</body>") {
		t.Fatal("reload client was not inserted inside body")
	}
	policy := response.Header.Get("Content-Security-Policy")
	if !strings.Contains(policy, reloadClientHash) || strings.Contains(policy, "unsafe-inline") {
		t.Fatalf("CSP did not contain only the reload hash allowance: %q", policy)
	}
	scriptElementPolicy := cspDirective(policy, "script-src-elem")
	if !strings.Contains(scriptElementPolicy, reloadClientHash) || strings.Contains(scriptElementPolicy, "'none'") {
		t.Fatalf("CSP script-src-elem still blocks the reload client: %q", policy)
	}
	if !strings.Contains(policy, "connect-src") || !strings.Contains(policy, "'self'") {
		t.Fatalf("CSP does not allow same-origin SSE: %q", policy)
	}
	if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("ETag") != "" {
		t.Fatalf("development cache headers = %#v", response.Header)
	}
	if response.ContentLength != int64(len(got)) {
		t.Fatalf("ContentLength = %d, want %d", response.ContentLength, len(got))
	}
}

func cspDirective(policy, name string) string {
	for _, raw := range strings.Split(policy, ";") {
		fields := strings.Fields(raw)
		if len(fields) != 0 && strings.EqualFold(fields[0], name) {
			return strings.Join(fields, " ")
		}
	}
	return ""
}

func TestInjectionExcludesFragmentsAndNonHTML(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		header      string
		method      string
		status      int
	}{
		{name: "unmarked HTML fragment", contentType: "text/html", status: http.StatusOK},
		{name: "htmx fragment", contentType: "text/html", header: "HX-Request", status: http.StatusOK},
		{name: "turbo fragment", contentType: "text/html", header: "Turbo-Frame", status: http.StatusOK},
		{name: "json api", contentType: "application/json", status: http.StatusOK},
		{name: "HEAD response", contentType: "text/html", method: http.MethodHead, status: http.StatusOK},
		{name: "no content", contentType: "text/html", status: http.StatusNoContent},
		{name: "not modified", contentType: "text/html", status: http.StatusNotModified},
		{name: "partial content", contentType: "text/html", status: http.StatusPartialContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := "<p>fragment</p>"
			method := test.method
			if method == "" {
				method = http.MethodGet
			}
			request := httptest.NewRequest(method, "http://example.test/items", nil)
			if test.header != "" {
				request.Header.Set(test.header, "true")
			}
			response := &http.Response{
				StatusCode: test.status,
				Header:     http.Header{"Content-Type": []string{test.contentType}},
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    request,
			}
			if err := injectDevelopmentClient(response); err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != body {
				t.Fatalf("fragment was modified: %q", got)
			}
			if response.Header.Get("Cache-Control") != "no-store" {
				t.Fatal("fragment caching was not disabled")
			}
		})
	}
}

func TestFullDocumentEvidenceAndCSPNone(t *testing.T) {
	t.Parallel()
	body := " \n<!-- generated -->\n<!DOCTYPE HTML><html><body>page</body></html>"
	request := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":            []string{"text/html"},
			"Content-Security-Policy": []string{"default-src 'none'"},
		},
		Body:    io.NopCloser(strings.NewReader(body)),
		Request: request,
	}
	if err := injectDevelopmentClient(response); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), string(reloadClientTag)) {
		t.Fatal("full document with a leading comment was not injected")
	}
	policy := response.Header.Get("Content-Security-Policy")
	if strings.Contains(policy, "script-src 'none'") || strings.Contains(policy, "connect-src 'none'") {
		t.Fatalf("CSP 'none' was combined with an allowance: %q", policy)
	}
	if !strings.Contains(policy, "script-src "+reloadClientHash) || !strings.Contains(policy, "connect-src 'self'") {
		t.Fatalf("CSP missing narrow development allowances: %q", policy)
	}
}

func TestCSPFallbackUsesFirstDuplicateDirective(t *testing.T) {
	t.Parallel()
	for _, first := range []string{"'none'", ""} {
		header := make(http.Header)
		header.Set("Content-Security-Policy", "default-src "+first+"; default-src https://ignored-attacker.example")
		adjustCSP(header, "Content-Security-Policy")
		policy := header.Get("Content-Security-Policy")
		for _, directive := range []string{"script-src", "script-src-elem"} {
			value := cspDirective(policy, directive)
			if !strings.Contains(value, reloadClientHash) || strings.Contains(value, "ignored-attacker.example") {
				t.Fatalf("%s was broadened from an ignored duplicate fallback: %q", directive, policy)
			}
		}
	}
}

func TestEventStream(t *testing.T) {
	hub := newEventHub()
	hub.publish(Event{Type: "diagnostic", Phase: "generate", Message: "broken before connect"})
	server := httptest.NewServer(http.HandlerFunc(hub.serveHTTP))
	t.Cleanup(func() {
		hub.close()
		server.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	ready := readSSEEvent(t, scanner)
	if ready.Type != "ready" {
		t.Fatalf("first event = %#v", ready)
	}
	replayed := readSSEEvent(t, scanner)
	if replayed.Type != "diagnostic" || replayed.Message != "broken before connect" {
		t.Fatalf("replayed event = %#v", replayed)
	}

	deadline := time.Now().Add(time.Second)
	for {
		hub.mu.Lock()
		count := len(hub.subscribers)
		hub.mu.Unlock()
		if count != 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("SSE handler did not subscribe")
		}
		time.Sleep(time.Millisecond)
	}
	hub.publish(Event{Type: "diagnostic", Phase: "generate", Message: "broken"})
	event := readSSEEvent(t, scanner)
	if event.Type != "diagnostic" || event.Phase != "generate" || event.Message != "broken" {
		t.Fatalf("streamed event = %#v", event)
	}
}

func TestWaitingPageConnectsToEvents(t *testing.T) {
	t.Parallel()
	proxy := newDevelopmentProxy(newEventHub())
	if err := proxy.setAuthority("127.0.0.1:7331"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7331/", nil)
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, request)
	result := recorder.Result()
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), string(reloadClientTag)) {
		t.Fatalf("waiting response status/body = %d %q", result.StatusCode, body)
	}
	policy := result.Header.Get("Content-Security-Policy")
	if !strings.Contains(policy, reloadClientHash) || strings.Contains(policy, "unsafe-inline") {
		t.Fatalf("waiting page CSP = %q", policy)
	}
}

func TestClearTargetOnlyClearsSelectedUpstream(t *testing.T) {
	t.Parallel()
	proxy := newDevelopmentProxy(newEventHub())
	if err := proxy.setTarget("127.0.0.1:7001"); err != nil {
		t.Fatal(err)
	}
	if proxy.clearTarget("127.0.0.1:7002") {
		t.Fatal("clearTarget cleared a different selected upstream")
	}
	if target := proxy.target.Load(); target == nil || target.Host != "127.0.0.1:7001" {
		t.Fatalf("selected upstream changed unexpectedly: %v", target)
	}
	if !proxy.clearTarget("127.0.0.1:7001") {
		t.Fatal("clearTarget did not clear the selected upstream")
	}
	if target := proxy.target.Load(); target != nil {
		t.Fatalf("selected upstream remains after clear: %v", target)
	}
}

func TestDevelopmentProxyRequiresLocalAuthorityAndSameOrigin(t *testing.T) {
	t.Parallel()
	var upstreamRequests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamRequests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	proxy := newDevelopmentProxy(newEventHub())
	if err := proxy.setAuthority("127.0.0.1:7331"); err != nil {
		t.Fatal(err)
	}
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := proxy.setTarget(target.Host); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		host          string
		origin        string
		fetchSite     string
		wantStatus    int
		wantForwarded bool
	}{
		{name: "IPv4 loopback", host: "127.0.0.1:7331", wantStatus: http.StatusNoContent, wantForwarded: true},
		{name: "alternate loopback", host: "127.0.0.2:7331", wantStatus: http.StatusNoContent, wantForwarded: true},
		{name: "IPv6 loopback", host: "[::1]:7331", wantStatus: http.StatusNoContent, wantForwarded: true},
		{name: "localhost", host: "localhost:7331", wantStatus: http.StatusNoContent, wantForwarded: true},
		{name: "localhost trailing dot", host: "LOCALHOST.:7331", wantStatus: http.StatusNoContent, wantForwarded: true},
		{name: "same origin", host: "localhost:7331", origin: "http://localhost:7331", fetchSite: "same-origin", wantStatus: http.StatusNoContent, wantForwarded: true},
		{name: "DNS rebinding host", host: "attacker.example:7331", wantStatus: http.StatusMisdirectedRequest},
		{name: "localhost suffix", host: "localhost.attacker.example:7331", wantStatus: http.StatusMisdirectedRequest},
		{name: "public IP host", host: "192.0.2.1:7331", wantStatus: http.StatusMisdirectedRequest},
		{name: "wrong port", host: "127.0.0.1:7332", wantStatus: http.StatusMisdirectedRequest},
		{name: "missing port", host: "127.0.0.1", wantStatus: http.StatusMisdirectedRequest},
		{name: "cross origin", host: "127.0.0.1:7331", origin: "https://attacker.example", wantStatus: http.StatusForbidden},
		{name: "different local origin", host: "127.0.0.1:7331", origin: "http://localhost:7331", wantStatus: http.StatusForbidden},
		{name: "null origin", host: "127.0.0.1:7331", origin: "null", wantStatus: http.StatusForbidden},
		{name: "cross site metadata", host: "127.0.0.1:7331", fetchSite: "cross-site", wantStatus: http.StatusForbidden},
		{name: "same site but cross origin metadata", host: "localhost:7331", fetchSite: "same-site", wantStatus: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := upstreamRequests.Load()
			request := httptest.NewRequest(http.MethodGet, "http://"+test.host+"/", nil)
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.fetchSite != "" {
				request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			}
			recorder := httptest.NewRecorder()
			proxy.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			forwarded := upstreamRequests.Load() != before
			if forwarded != test.wantForwarded {
				t.Fatalf("forwarded = %v, want %v", forwarded, test.wantForwarded)
			}
			if !test.wantForwarded && recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("rejection was cacheable")
			}
		})
	}
}

func TestDevelopmentProxyProtectsEventStream(t *testing.T) {
	t.Parallel()
	hub := newEventHub()
	t.Cleanup(hub.close)
	proxy := newDevelopmentProxy(hub)
	if err := proxy.setAuthority("127.0.0.1:7331"); err != nil {
		t.Fatal(err)
	}

	rejected := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7331"+eventsPath, nil)
	rejected.Header.Set("Origin", "https://attacker.example")
	rejectedRecorder := httptest.NewRecorder()
	proxy.ServeHTTP(rejectedRecorder, rejected)
	if rejectedRecorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin event stream status = %d, want %d", rejectedRecorder.Code, http.StatusForbidden)
	}

	ctx, cancel := context.WithCancel(context.Background())
	allowed := httptest.NewRequest(http.MethodGet, "http://localhost:7331"+eventsPath, nil).WithContext(ctx)
	allowed.Header.Set("Origin", "http://localhost:7331")
	cancel()
	allowedRecorder := httptest.NewRecorder()
	proxy.ServeHTTP(allowedRecorder, allowed)
	if allowedRecorder.Code != http.StatusOK || !strings.Contains(allowedRecorder.Body.String(), "event: ready") {
		t.Fatalf("same-origin event stream status/body = %d %q", allowedRecorder.Code, allowedRecorder.Body.String())
	}
}

func TestEventHubRetainsNewestEventForSlowSubscriber(t *testing.T) {
	t.Parallel()
	hub := newEventHub()
	updates, unsubscribe := hub.subscribe()
	defer unsubscribe()
	for index := 0; index < 20; index++ {
		hub.publish(Event{Type: "diagnostic", Message: strconv.Itoa(index)})
	}
	hub.publish(Event{Type: "reload"})
	var last Event
	for len(updates) != 0 {
		last = <-updates
	}
	if last.Type != "reload" {
		t.Fatalf("newest queued event = %#v, want reload", last)
	}
}

func readSSEEvent(t *testing.T, scanner *bufio.Scanner) Event {
	t.Helper()
	var data string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		}
		if line == "" && data != "" {
			var event Event
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				t.Fatalf("decode SSE event: %v", err)
			}
			return event
		}
	}
	t.Fatalf("SSE stream ended: %v", scanner.Err())
	return Event{}
}
