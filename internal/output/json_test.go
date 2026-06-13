package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
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

func TestWriteEnvelopeSuccess(t *testing.T) {
	var buf bytes.Buffer
	clock := func() time.Time {
		return time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)
	}

	if err := WriteEnvelope(&buf, "version", apperrors.CodeOK, "Version information", map[string]string{"version": "dev"}, clock); err != nil {
		t.Fatalf("WriteEnvelope returned error: %v", err)
	}

	var got Envelope
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}
	if !got.OK {
		t.Fatalf("OK = false, want true")
	}
	if got.Command != "version" {
		t.Fatalf("Command = %q, want version", got.Command)
	}
	if got.Code != apperrors.CodeOK {
		t.Fatalf("Code = %q, want %q", got.Code, apperrors.CodeOK)
	}
	if got.Timestamp != "2026-06-13T12:34:56Z" {
		t.Fatalf("Timestamp = %q", got.Timestamp)
	}
	if len(got.Errors) != 0 {
		t.Fatalf("Errors = %#v, want empty", got.Errors)
	}
}

func TestNewEnvelopeFailure(t *testing.T) {
	envelope := NewEnvelope("install", apperrors.CodeUsage, "not implemented", nil, func() time.Time {
		return time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)
	})

	if envelope.OK {
		t.Fatalf("OK = true, want false")
	}
	if len(envelope.Errors) != 1 {
		t.Fatalf("Errors = %#v, want one error", envelope.Errors)
	}
	if envelope.Errors[0].Message != "not implemented" {
		t.Fatalf("error message = %q", envelope.Errors[0].Message)
	}
}

func TestWriteEnvelopeWithWarnings(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteEnvelopeWithWarnings(&buf, "validate", apperrors.CodeOK, "valid", nil, []string{"using default"}, func() time.Time {
		return time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)
	}); err != nil {
		t.Fatalf("WriteEnvelopeWithWarnings returned error: %v", err)
	}

	var got Envelope
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "using default" {
		t.Fatalf("Warnings = %#v", got.Warnings)
	}
}
