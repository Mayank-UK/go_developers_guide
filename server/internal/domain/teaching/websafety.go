// Web-safety teaching exhibits: the three input-to-danger guards the
// security topic quotes — HTML rendering that auto-escapes (XSS),
// download paths that cannot escape their base (path traversal), and
// outbound URLs that refuse the internal network (SSRF). Compilable,
// tested, and quoted; the production API never renders HTML or fetches
// caller-supplied URLs, so these live as exhibits rather than dead
// production code.
package teaching

import (
	"fmt"
	"html/template"
	"net"
	"net/url"
	"path/filepath"
	"strings"
)

// RenderComment renders one user-submitted comment as HTML through
// html/template — the standard library's CONTEXT-AWARE auto-escaping.
// A comment reading <script>fetch('/steal')</script> arrives at the
// browser as &lt;script&gt;…: escaped for the HTML context it lands
// in, because the template engine escapes by default and un-escaping
// requires the deliberate template.HTML conversion. That default is
// the whole defense: rendering untrusted input safely is the property
// of the engine you choose, not the discipline of every call site.
func RenderComment(author, body string) (string, error) {
	page, err := template.New("comment").Parse(
		"<div class=comment><b>{{.Author}}</b><p>{{.Body}}</p></div>")
	if err != nil {
		return "", fmt.Errorf("parse comment template: %w", err)
	}
	var builder strings.Builder
	if err := page.Execute(&builder, map[string]string{"Author": author, "Body": body}); err != nil {
		return "", fmt.Errorf("execute comment template: %w", err)
	}
	return builder.String(), nil
}

// ResolveWithin answers the path a download request actually refers
// to, confined to base — the path-traversal guard. The rule is clean
// BEFORE the confinement test: ".." segments inside the request are
// rejected outright (a legitimate download URL never needs them), and
// then filepath.Clean plus the base prefix check catches every
// remaining escape shape, including encoded separators that survived
// decoding upstream. Symlinks are named honestly: this guard confines
// the STRING; a symlink inside base pointing outside needs an
// filepath.EvalSymlinks pass in addition.
func ResolveWithin(base, requested string) (string, error) {
	if strings.Contains(requested, "..") {
		return "", fmt.Errorf("download path %q contains '..' — rejected", requested)
	}
	cleaned := filepath.Clean(filepath.Join(base, requested))
	absoluteBase := filepath.Clean(base)
	if cleaned != absoluteBase && !strings.HasPrefix(cleaned, absoluteBase+string(filepath.Separator)) {
		return "", fmt.Errorf("download path %q escapes the allowed base", requested)
	}
	return cleaned, nil
}

// blockedHosts are the SSRF destinations a caller has no business
// pointing a server at: the loopback, the link-local cloud metadata
// service, and the all-zero network address. The list is deliberately
// short and named — it is a STARTER guard, and its limits are the
// teaching: DNS names that RESOLVE to these addresses, redirect chains
// that hop to them, and private CIDR ranges need a full
// ip-is-private check (netip spans) in production.
var blockedHosts = map[string]bool{
	"localhost":       true,
	"127.0.0.1":       true,
	"::1":             true,
	"169.254.169.254": true,
	"metadata.goog":   true,
	"0.0.0.0":         true,
}

// GuardOutboundURL validates a caller-supplied URL before the server
// fetches it — the SSRF guard. Allowlist the SCHEME (http/https only),
// refuse embedded credentials, and refuse the hosts nobody but an
// interior probe has a reason to name. Every rule answers one attack:
// scheme confusion (file://, gopher://), credential capture via
// userinfo, and the internal-network probe.
func GuardOutboundURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse outbound url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("outbound url scheme %q refused — http/https only", parsed.Scheme)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("outbound url carries credentials in its authority — refused")
	}
	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("outbound url has no host")
	}
	if blockedHosts[strings.ToLower(host)] {
		return "", fmt.Errorf("outbound url host %q is an internal address — the server does not fetch on its own network's behalf", host)
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return "", fmt.Errorf("outbound url host %q is an internal address — refused", host)
	}
	return parsed.String(), nil
}
