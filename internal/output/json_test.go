package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestJSON(t *testing.T) {
	var buf bytes.Buffer

	if err := JSON(&buf, map[string]string{"ok": "true"}); err != nil {
		t.Fatalf("JSON returned error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `"ok": "true"`) {
		t.Fatalf("JSON output = %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("JSON output should end with newline: %q", got)
	}
}
