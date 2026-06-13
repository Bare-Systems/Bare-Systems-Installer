package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/output"
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
	app := New()
	app.clock = func() time.Time {
		return time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)
	}

	code := app.Run([]string{"--json", "version"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitOK)
	}
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode JSON envelope: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("OK = false, want true")
	}
	if envelope.Command != "version" {
		t.Fatalf("Command = %q, want version", envelope.Command)
	}
	if envelope.Code != apperrors.CodeOK {
		t.Fatalf("Code = %q, want %q", envelope.Code, apperrors.CodeOK)
	}
	if envelope.Timestamp != "2026-06-13T12:34:56Z" {
		t.Fatalf("Timestamp = %q", envelope.Timestamp)
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"wat"}, &stdout, &stderr)

	if code != apperrors.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown or incomplete command "wat"`) {
		t.Fatalf("stderr missing unknown command: %q", stderr.String())
	}
}

func TestRecognizedCommandStub(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"install"}, &stdout, &stderr)

	if code != apperrors.ExitGeneric {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitGeneric)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "install is recognized but not implemented yet") {
		t.Fatalf("stderr missing not implemented message: %q", stderr.String())
	}
}

func TestNestedCommandStubJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := New()
	app.clock = func() time.Time {
		return time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)
	}

	code := app.Run([]string{"--json", "config", "render"}, &stdout, &stderr)

	if code != apperrors.ExitGeneric {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitGeneric)
	}
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode JSON envelope: %v", err)
	}
	if envelope.OK {
		t.Fatalf("OK = true, want false")
	}
	if envelope.Command != "config render" {
		t.Fatalf("Command = %q, want config render", envelope.Command)
	}
	if envelope.Code != apperrors.CodeGeneric {
		t.Fatalf("Code = %q, want %q", envelope.Code, apperrors.CodeGeneric)
	}
}

func TestQuietVersionCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"--quiet", "version"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitOK)
	}
	if strings.TrimSpace(stdout.String()) != "dev" {
		t.Fatalf("stdout = %q, want dev", stdout.String())
	}
}
