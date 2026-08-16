package server

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// moProxy publishes the mo server through a loopback-only origin owned by sa.
//
// mo answers every request with "frame-ancestors 'none'", which forbids any
// page from framing it. sa needs the preview next to the diff in the same
// window, so the proxy relaxes exactly that directive to sa's own origin and
// forwards everything else untouched.
type moProxy struct {
	ln      net.Listener
	baseURL string
	target  *url.URL
	srv     *http.Server
}

func newMoProxy(targetURL, allowedOrigin string) (*moProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid mo URL %q: %w", targetURL, err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("cannot listen for the preview proxy: %w", err)
	}
	p := &moProxy{
		ln:      ln,
		target:  target,
		baseURL: "http://" + ln.Addr().String(),
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	// mo streams live reload events; do not buffer them.
	rp.FlushInterval = -1
	rp.ModifyResponse = func(resp *http.Response) error {
		relaxFrameAncestors(resp.Header, allowedOrigin, p.baseURL)
		return nil
	}
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "cannot reach the mo server at "+target.String()+": "+err.Error(), http.StatusBadGateway)
	}

	p.srv = &http.Server{
		Handler:           rp,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return p, nil
}

func (p *moProxy) serve() {
	_ = p.srv.Serve(p.ln)
}

func (p *moProxy) close() {
	_ = p.srv.Close()
}

// rewrite maps a URL of the mo server onto the proxy origin so that it can be
// framed. URLs that do not belong to mo are returned unchanged.
func (p *moProxy) rewrite(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if !sameEndpoint(u, p.target) {
		return raw
	}
	proxyURL, err := url.Parse(p.baseURL)
	if err != nil {
		return raw
	}
	u.Scheme, u.Host = proxyURL.Scheme, proxyURL.Host
	return u.String()
}

// sameEndpoint reports whether two URLs point at the same host:port, treating
// localhost and the loopback addresses as equal.
func sameEndpoint(a, b *url.URL) bool {
	if a.Host == b.Host {
		return true
	}
	ah, ap, err := net.SplitHostPort(a.Host)
	if err != nil {
		return false
	}
	bh, bp, err := net.SplitHostPort(b.Host)
	if err != nil {
		return false
	}
	if ap != bp {
		return false
	}
	return normalizeHost(ah) == normalizeHost(bh)
}

func normalizeHost(h string) string {
	switch strings.ToLower(h) {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return "localhost"
	}
	return strings.ToLower(h)
}

// relaxFrameAncestors rewrites the frame-ancestors directive of a Content
// Security Policy so that the given origins may frame the response. Every
// other directive is left exactly as the upstream server sent it.
func relaxFrameAncestors(h http.Header, origins ...string) {
	// X-Frame-Options has no origin list worth keeping; the CSP below is
	// what actually governs framing for browsers that support it.
	h.Del("X-Frame-Options")

	policies := h.Values("Content-Security-Policy")
	if len(policies) == 0 {
		return
	}
	allow := "frame-ancestors 'self'"
	for _, o := range origins {
		if o != "" {
			allow += " " + o
		}
	}
	rewritten := make([]string, 0, len(policies))
	for _, policy := range policies {
		directives := strings.Split(policy, ";")
		out := make([]string, 0, len(directives))
		replaced := false
		for _, d := range directives {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(d)), "frame-ancestors") {
				out = append(out, " "+allow)
				replaced = true
				continue
			}
			out = append(out, d)
		}
		if !replaced {
			out = append(out, " "+allow)
		}
		rewritten = append(rewritten, strings.Join(out, ";"))
	}
	h.Del("Content-Security-Policy")
	for _, p := range rewritten {
		h.Add("Content-Security-Policy", p)
	}
}
