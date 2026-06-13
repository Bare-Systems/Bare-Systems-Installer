package cli

import (
	"bytes"
	"strings"
	"testing"

	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
)

func TestHelpCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"--help"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitOK)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("help output missing usage: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"version"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitOK)
	}
	out := stdout.String()
	for _, want := range []string{"bare-systems", "commit:", "date:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("version output missing %q: %q", want, out)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionCommandJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"--json", "version"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitOK)
	}
	out := stdout.String()
	for _, want := range []string{`"version"`, `"commit"`, `"date"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON version output missing %q: %q", want, out)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"install"}, &stdout, &stderr)

	if code != apperrors.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown command "install"`) {
		t.Fatalf("stderr missing unknown command: %q", stderr.String())
	}
}
