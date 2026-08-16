package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const moCSP = "default-src 'self'; script-src 'self' 'unsafe-eval'; connect-src 'self'; frame-ancestors 'none'"

func TestRelaxFrameAncestors(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Security-Policy", moCSP)
	h.Set("X-Frame-Options", "DENY")

	relaxFrameAncestors(h, "http://localhost:6280")

	got := h.Get("Content-Security-Policy")
	if strings.Contains(got, "frame-ancestors 'none'") {
		t.Errorf("frame-ancestors was not relaxed: %q", got)
	}
	if !strings.Contains(got, "frame-ancestors 'self' http://localhost:6280") {
		t.Errorf("policy = %q", got)
	}
	// Everything else has to survive untouched.
	for _, directive := range []string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-eval'",
		"connect-src 'self'",
	} {
		if !strings.Contains(got, directive) {
			t.Errorf("policy lost %q: %q", directive, got)
		}
	}
	if h.Get("X-Frame-Options") != "" {
		t.Error("X-Frame-Options should be dropped")
	}
}

func TestRelaxFrameAncestorsAddsMissingDirective(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Security-Policy", "default-src 'self'")
	relaxFrameAncestors(h, "http://localhost:6280")
	if !strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'self' http://localhost:6280") {
		t.Errorf("policy = %q", h.Get("Content-Security-Policy"))
	}
}

func TestRelaxFrameAncestorsKeepsUnsetHeader(t *testing.T) {
	h := http.Header{}
	relaxFrameAncestors(h, "http://localhost:6280")
	if got := h.Get("Content-Security-Policy"); got != "" {
		t.Errorf("policy = %q, want no policy invented for a server that sends none", got)
	}
}

func TestMoProxyServesFramablePages(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", moCSP)
		io.WriteString(w, "mo page for "+r.URL.String())
	}))
	defer upstream.Close()

	proxy, err := newMoProxy(upstream.URL, "http://localhost:6280")
	if err != nil {
		t.Fatal(err)
	}
	go proxy.serve()
	defer proxy.close()

	resp, err := http.Get(proxy.baseURL + "/sa-default?file=abc")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "mo page for /sa-default?file=abc") {
		t.Errorf("body = %q", body)
	}
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "frame-ancestors 'self' http://localhost:6280") {
		t.Errorf("policy = %q", resp.Header.Get("Content-Security-Policy"))
	}
}

func TestMoProxyRewritesURLs(t *testing.T) {
	proxy, err := newMoProxy("http://localhost:6275", "http://localhost:6280")
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.close()

	// mo may answer with either spelling of the loopback host.
	for _, in := range []string{
		"http://localhost:6275/sa-default?file=abc",
		"http://127.0.0.1:6275/sa-default?file=abc",
	} {
		got := proxy.rewrite(in)
		if !strings.HasPrefix(got, proxy.baseURL) || !strings.HasSuffix(got, "/sa-default?file=abc") {
			t.Errorf("rewrite(%q) = %q", in, got)
		}
	}
	// A URL of some other server is none of the proxy's business.
	other := "http://example.com/page"
	if got := proxy.rewrite(other); got != other {
		t.Errorf("rewrite(%q) = %q", other, got)
	}
}
