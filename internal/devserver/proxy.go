// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
)

const (
	maxInjectableHTML = 16 << 20
	reloadClient      = `(function(){var id="__himesan_overlay";function show(e){var d=document.getElementById(id);if(!d){d=document.createElement("dialog");d.id=id;var b=document.createElement("button");b.textContent="Close";b.addEventListener("click",function(){d.close()});var p=document.createElement("pre");d.appendChild(b);d.appendChild(p);document.body.appendChild(d)}var p=d.querySelector("pre"),xs=e.diagnostics||[];p.textContent=(e.phase?e.phase+": ":"")+(e.message||"Hime-san could not reload")+(xs.length?"\n\n"+xs.map(function(x){return (x.path||"")+(x.line?":"+x.line+(x.column?":"+x.column:""):"")+(x.code?" ["+x.code+"]":"")+" "+x.message}).join("\n"):"");if(!d.open)d.showModal()}var s=new EventSource("/__himesan/events");s.addEventListener("reload",function(){location.reload()});s.addEventListener("diagnostic",function(e){try{show(JSON.parse(e.data))}catch(_){show({message:e.data})}})})();`
)

var (
	reloadClientTag  = []byte("<script data-himesan-reload>" + reloadClient + "</script>")
	reloadClientHash = makeReloadClientHash()
)

func makeReloadClientHash() string {
	digest := sha256.Sum256([]byte(reloadClient))
	return "'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "'"
}

type developmentProxy struct {
	target    atomic.Pointer[url.URL]
	authority atomic.Pointer[localProxyAuthority]
	hub       *eventHub
	proxy     *httputil.ReverseProxy
}

type localProxyAuthority struct {
	port int
}

func newDevelopmentProxy(hub *eventHub) *developmentProxy {
	d := &developmentProxy{hub: hub}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	d.proxy = &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(request *httputil.ProxyRequest) {
			target := d.target.Load()
			if target == nil {
				return
			}
			request.SetURL(target)
			request.SetXForwarded()
			request.Out.Header.Set("Accept-Encoding", "identity")
			request.Out.Header.Del("If-Modified-Since")
			request.Out.Header.Del("If-None-Match")
			request.Out.Header.Set("Cache-Control", "no-cache")
		},
		ModifyResponse: injectDevelopmentClient,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, "Hime-san development upstream is unavailable: "+err.Error(), http.StatusBadGateway)
		},
	}
	return d
}

func (d *developmentProxy) closeIdleConnections() {
	if transport, ok := d.proxy.Transport.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

func (d *developmentProxy) setAuthority(address string) error {
	if err := ValidateLoopbackAddress(address); err != nil {
		return fmt.Errorf("development proxy authority %q: %w", address, err)
	}
	_, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("split development proxy authority: %w", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return fmt.Errorf("parse development proxy port: %w", err)
	}
	d.authority.Store(&localProxyAuthority{port: port})
	return nil
}

func (d *developmentProxy) setTarget(address string) error {
	if err := ValidateLoopbackAddress(address); err != nil {
		return fmt.Errorf("development upstream %q: %w", address, err)
	}
	target, err := url.Parse("http://" + address)
	if err != nil {
		return fmt.Errorf("parse development upstream: %w", err)
	}
	d.target.Store(target)
	return nil
}

func (d *developmentProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if status, message := d.validateRequest(r); status != 0 {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.Error(w, message, status)
		return
	}
	if r.URL.Path == eventsPath {
		d.hub.serveHTTP(w, r)
		return
	}
	if d.target.Load() == nil {
		d.serveWaitingPage(w)
		return
	}
	d.proxy.ServeHTTP(w, r)
}

func (d *developmentProxy) validateRequest(r *http.Request) (int, string) {
	authority := d.authority.Load()
	if authority == nil {
		return http.StatusServiceUnavailable, "Hime-san development proxy is not ready"
	}
	requestAuthority, ok := canonicalLoopbackAuthority(r.Host, authority.port)
	if !ok {
		return http.StatusMisdirectedRequest, "Hime-san development proxy requires a loopback Host authority"
	}

	if values := r.Header.Values("Sec-Fetch-Site"); len(values) > 1 {
		return http.StatusForbidden, "cross-origin development proxy request denied"
	} else if len(values) == 1 {
		switch strings.ToLower(strings.TrimSpace(values[0])) {
		case "", "none", "same-origin":
		default:
			return http.StatusForbidden, "cross-origin development proxy request denied"
		}
	}

	origins := r.Header.Values("Origin")
	if len(origins) > 1 {
		return http.StatusForbidden, "cross-origin development proxy request denied"
	}
	if len(origins) == 1 {
		originAuthority, ok := canonicalHTTPOrigin(origins[0], authority.port)
		if !ok || originAuthority != requestAuthority {
			return http.StatusForbidden, "cross-origin development proxy request denied"
		}
	}
	return 0, ""
}

func canonicalHTTPOrigin(raw string, port int) (string, bool) {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return "", false
	}
	origin, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(origin.Scheme, "http") || origin.Host == "" || origin.User != nil || origin.Opaque != "" || origin.Path != "" || origin.RawPath != "" || origin.RawQuery != "" || origin.Fragment != "" || origin.ForceQuery {
		return "", false
	}
	return canonicalLoopbackAuthority(origin.Host, port)
}

func canonicalLoopbackAuthority(authority string, requiredPort int) (string, bool) {
	if authority == "" || strings.TrimSpace(authority) != authority {
		return "", false
	}
	host, rawPort, err := net.SplitHostPort(authority)
	if err != nil {
		if requiredPort != 80 {
			return "", false
		}
		host = authority
		rawPort = "80"
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = host[1 : len(host)-1]
		}
	}
	requestPort, err := strconv.Atoi(rawPort)
	if err != nil || requestPort != requiredPort {
		return "", false
	}

	normalizedHost := strings.ToLower(host)
	if normalizedHost == "localhost" || normalizedHost == "localhost." {
		return "localhost:" + strconv.Itoa(requiredPort), true
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", false
	}
	return ip.String() + ":" + strconv.Itoa(requiredPort), true
}

func (d *developmentProxy) serveWaitingPage(w http.ResponseWriter) {
	body := append([]byte("<!doctype html><html><body><h1>Hime-san is waiting for a healthy application build.</h1>"), reloadClientTag...)
	body = append(body, []byte("</body></html>")...)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src "+reloadClientHash+"; connect-src 'self'")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write(body)
}

func injectDevelopmentClient(response *http.Response) error {
	disableDevelopmentCaching(response)
	if !eligibleHTMLResponse(response) {
		return nil
	}

	prefix, err := io.ReadAll(io.LimitReader(response.Body, maxInjectableHTML+1))
	if err != nil {
		return fmt.Errorf("read HTML for development reload injection: %w", err)
	}
	if len(prefix) > maxInjectableHTML {
		response.Body = &prefixedReadCloser{
			Reader: io.MultiReader(bytes.NewReader(prefix), response.Body),
			Closer: response.Body,
		}
		return nil
	}
	if err := response.Body.Close(); err != nil {
		return fmt.Errorf("close upstream HTML response: %w", err)
	}
	if !isFullHTMLDocument(prefix) {
		response.Body = io.NopCloser(bytes.NewReader(prefix))
		response.ContentLength = int64(len(prefix))
		response.Header.Set("Content-Length", strconv.Itoa(len(prefix)))
		return nil
	}

	contents := insertReloadClient(prefix)
	response.Body = io.NopCloser(bytes.NewReader(contents))
	response.ContentLength = int64(len(contents))
	response.Header.Set("Content-Length", strconv.Itoa(len(contents)))
	response.Header.Del("ETag")
	response.Header.Del("Last-Modified")
	adjustCSP(response.Header, "Content-Security-Policy")
	adjustCSP(response.Header, "Content-Security-Policy-Report-Only")
	return nil
}

func disableDevelopmentCaching(response *http.Response) {
	response.Header.Set("Cache-Control", "no-store")
	response.Header.Set("Pragma", "no-cache")
	response.Header.Set("Expires", "0")
	response.Header.Del("ETag")
	response.Header.Del("Last-Modified")
}

func eligibleHTMLResponse(response *http.Response) bool {
	if response.StatusCode != http.StatusOK || response.Request == nil || response.Body == nil || response.Request.Method == http.MethodHead {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "text/html") {
		return false
	}
	if encoding := strings.TrimSpace(response.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return false
	}
	rawDisposition := strings.TrimSpace(response.Header.Get("Content-Disposition"))
	disposition, _, dispositionErr := mime.ParseMediaType(rawDisposition)
	if strings.EqualFold(disposition, "attachment") || (dispositionErr != nil && strings.HasPrefix(strings.ToLower(rawDisposition), "attachment")) {
		return false
	}
	request := response.Request
	for _, header := range []string{"HX-Request", "Turbo-Frame", "X-PJAX", "X-Requested-With"} {
		if strings.TrimSpace(request.Header.Get(header)) != "" {
			return false
		}
	}
	if strings.EqualFold(response.Header.Get("X-Himesan-Fragment"), "true") {
		return false
	}
	destination := strings.TrimSpace(request.Header.Get("Sec-Fetch-Dest"))
	return destination == "" || strings.EqualFold(destination, "document")
}

func isFullHTMLDocument(contents []byte) bool {
	remaining := bytes.TrimSpace(contents)
	remaining = bytes.TrimSpace(bytes.TrimPrefix(remaining, []byte{0xef, 0xbb, 0xbf}))
	for bytes.HasPrefix(remaining, []byte("<!--")) {
		end := bytes.Index(remaining[4:], []byte("-->"))
		if end < 0 {
			return false
		}
		remaining = bytes.TrimSpace(remaining[4+end+3:])
	}
	lower := bytes.ToLower(remaining)
	return hasHTMLTokenPrefix(lower, "<!doctype html") || hasHTMLTokenPrefix(lower, "<html")
}

func hasHTMLTokenPrefix(contents []byte, prefix string) bool {
	if !bytes.HasPrefix(contents, []byte(prefix)) || len(contents) == len(prefix) {
		return false
	}
	next := contents[len(prefix)]
	return next == '>' || next == '/' || next == ' ' || next == '\t' || next == '\r' || next == '\n' || next == '\f'
}

func insertReloadClient(contents []byte) []byte {
	lower := bytes.ToLower(contents)
	position := bytes.LastIndex(lower, []byte("</body>"))
	if position < 0 {
		position = bytes.LastIndex(lower, []byte("</html>"))
	}
	if position < 0 {
		position = len(contents)
	}
	result := make([]byte, 0, len(contents)+len(reloadClientTag))
	result = append(result, contents[:position]...)
	result = append(result, reloadClientTag...)
	result = append(result, contents[position:]...)
	return result
}

func adjustCSP(header http.Header, name string) {
	policies := header.Values(name)
	if len(policies) == 0 {
		return
	}
	header.Del(name)
	for _, policy := range policies {
		policy = addCSPSource(policy, "script-src", reloadClientHash, "default-src")
		// CSP3 gives script-src-elem precedence over script-src for an inline
		// <script>. Preserve that directive's restrictions while granting the
		// same single hash, otherwise a policy such as script-src-elem 'none'
		// silently blocks the injected reload client.
		policy = addCSPSource(policy, "script-src-elem", reloadClientHash, "script-src")
		policy = addCSPSource(policy, "connect-src", "'self'", "default-src")
		header.Add(name, policy)
	}
}

func addCSPSource(policy, directive, source, fallback string) string {
	parts := strings.Split(policy, ";")
	fallbackSources := []string(nil)
	fallbackSeen := false
	for index, raw := range parts {
		fields := strings.Fields(raw)
		if len(fields) == 0 {
			continue
		}
		// CSP ignores duplicate directives after the first occurrence. Mirror
		// that rule when deriving a missing directive from its fallback so an
		// ignored, more-permissive duplicate cannot broaden the development page.
		if !fallbackSeen && strings.EqualFold(fields[0], fallback) {
			fallbackSeen = true
			fallbackSources = append([]string(nil), fields[1:]...)
		}
		if !strings.EqualFold(fields[0], directive) {
			continue
		}
		for _, existing := range fields[1:] {
			if existing == source {
				return strings.Join(parts, ";")
			}
		}
		currentSources := withoutCSPNone(fields[1:])
		parts[index] = fields[0]
		if len(currentSources) != 0 {
			parts[index] += " " + strings.Join(currentSources, " ")
		}
		parts[index] += " " + source
		return strings.Join(parts, ";")
	}
	if !fallbackSeen {
		// Without this directive or a default-src fallback the resource is
		// already unrestricted; introducing a directive would unnecessarily
		// restrict the application under test.
		return policy
	}
	addition := directive
	if retained := withoutCSPNone(fallbackSources); len(retained) != 0 {
		addition += " " + strings.Join(retained, " ")
	}
	addition += " " + source
	if strings.TrimSpace(policy) == "" {
		return addition
	}
	if strings.HasSuffix(strings.TrimSpace(policy), ";") {
		return policy + " " + addition
	}
	return policy + "; " + addition
}

func withoutCSPNone(sources []string) []string {
	result := make([]string, 0, len(sources))
	for _, source := range sources {
		if !strings.EqualFold(source, "'none'") {
			result = append(result, source)
		}
	}
	return result
}

type prefixedReadCloser struct {
	io.Reader
	io.Closer
}
