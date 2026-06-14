package runtime

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestCheckDockerSuccess(t *testing.T) {
	runner := &FakeRunner{
		Results: map[string]FakeResult{
			"docker --version":       {Stdout: "Docker version 1\n"},
			"docker info":            {Stdout: "Server Version: 1\n"},
			"docker compose version": {Stdout: "Docker Compose version v2\n"},
		},
	}

	report, err := CheckDocker(context.Background(), runner)
	if err != nil {
		t.Fatalf("CheckDocker returned error: %v", err)
	}
	if len(report.Checks) != 3 {
		t.Fatalf("checks = %#v, want 3", report.Checks)
	}
}

func TestCheckDockerMissingCLI(t *testing.T) {
	runner := &FakeRunner{
		Results: map[string]FakeResult{
			"docker --version": {Err: &exec.Error{Name: "docker", Err: exec.ErrNotFound}},
		},
	}

	_, err := CheckDocker(context.Background(), runner)
	if err == nil {
		t.Fatalf("expected missing docker error")
	}
	var prereqErr PrereqError
	if !errors.As(err, &prereqErr) {
		t.Fatalf("error = %T, want PrereqError", err)
	}
	if prereqErr.Report.Checks[0].Remediation == "" {
		t.Fatalf("expected remediation")
	}
}

func TestCheckDockerRequiresComposePlugin(t *testing.T) {
	runner := &FakeRunner{
		Results: map[string]FakeResult{
			"docker --version":       {Stdout: "Docker version 1\n"},
			"docker info":            {Stdout: "Server Version: 1\n"},
			"docker compose version": {Stderr: "unknown command compose", Err: errFake},
		},
	}

	_, err := CheckDocker(context.Background(), runner)
	if err == nil {
		t.Fatalf("expected compose plugin error")
	}
	if !strings.Contains(err.Error(), "legacy `docker-compose` is not supported") {
		t.Fatalf("error missing compose remediation: %v", err)
	}
}
