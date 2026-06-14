package diagnostics

import (
	"strings"
	"testing"
)

func TestRedactString(t *testing.T) {
	input := "PORTAL_TOKEN=do-not-print\npassword: also-secret\nnormal=value\n"
	got := RedactString(input)
	if strings.Contains(got, "do-not-print") || strings.Contains(got, "also-secret") {
		t.Fatalf("redaction leaked secret: %q", got)
	}
	if !strings.Contains(got, "PORTAL_TOKEN="+Redacted) {
		t.Fatalf("token key not redacted as expected: %q", got)
	}
	if !strings.Contains(got, "normal=value") {
		t.Fatalf("non-secret value should remain: %q", got)
	}
}
