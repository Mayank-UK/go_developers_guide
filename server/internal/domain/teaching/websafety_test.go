package teaching

import (
	"strings"
	"testing"
)

func TestRenderCommentAutoEscapesHostileMarkup(t *testing.T) {
	rendered, err := RenderComment("ada", "<script>fetch('/steal?c='+document.cookie)</script>")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(rendered, "<script>") {
		t.Fatalf("raw script tag survived: %s", rendered)
	}
	if !strings.Contains(rendered, "&lt;script&gt;") {
		t.Errorf("the tag must arrive ESCAPED for the HTML context: %s", rendered)
	}
	if !strings.Contains(rendered, "<b>ada</b>") {
		t.Errorf("the template's own markup must render untouched: %s", rendered)
	}
}

func TestResolveWithinConfinesToTheBase(t *testing.T) {
	for _, ok := range []struct{ requested, want string }{
		{"reports/q3.pdf", "/srv/downloads/reports/q3.pdf"},
		{"./reports/a.txt", "/srv/downloads/reports/a.txt"},
	} {
		got, err := ResolveWithin("/srv/downloads", ok.requested)
		if err != nil {
			t.Errorf("ResolveWithin(%q) errored: %v", ok.requested, err)
			continue
		}
		if got != ok.want {
			t.Errorf("ResolveWithin(%q) = %q, want %q", ok.requested, got, ok.want)
		}
	}
	// The guard's posture is a conservative allowlist: a legitimate
	// download URL never needs '..', so ANY occurrence is refused —
	// even when Clean would have resolved it harmlessly back inside.
	for _, hostile := range []string{
		"../../../etc/passwd",
		"reports/../../secrets",
		"reports/../reports/a.txt",
		"..",
	} {
		if got, err := ResolveWithin("/srv/downloads", hostile); err == nil {
			t.Errorf("ResolveWithin(%q) = %q — traversal must be refused", hostile, got)
		}
	}
}

func TestGuardOutboundURLRefusesTheInternalNetwork(t *testing.T) {
	for _, hostile := range []string{
		"http://localhost:8100/admin",
		"http://127.0.0.1:8100/api/demo/scenario",
		"http://[::1]:8100/",
		"http://169.254.169.254/latest/meta-data/",
		"http://0.0.0.0:8100/",
		"file:///etc/passwd",
		"gopher://internal:70/",
		"http://user:pass@example.com/feed",
		"/relative/only",
	} {
		if got, err := GuardOutboundURL(hostile); err == nil {
			t.Errorf("GuardOutboundURL(%q) = %q — must be refused", hostile, got)
		}
	}
	for _, fine := range []string{
		"https://example.com/feed",
		"http://203.0.113.9/hooks",
	} {
		if _, err := GuardOutboundURL(fine); err != nil {
			t.Errorf("GuardOutboundURL(%q) errored on a legitimate target: %v", fine, err)
		}
	}
}
