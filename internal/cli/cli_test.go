package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	deploymentconfig "github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/output"
	edgeruntime "github.com/Bare-Systems/Bare-Systems-Installer/internal/runtime"
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

	code := New().Run([]string{"rollback"}, &stdout, &stderr)

	if code != apperrors.ExitGeneric {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitGeneric)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "rollback is recognized but not implemented yet") {
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

	code := app.Run([]string{"--json", "config", "diff"}, &stdout, &stderr)

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
	if envelope.Command != "config diff" {
		t.Fatalf("Command = %q, want config diff", envelope.Command)
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

func TestValidateCommand(t *testing.T) {
	configPath := writeDefaultDeployment(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"--config", configPath, "validate"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Deployment model valid") {
		t.Fatalf("stdout missing validation success: %q", stdout.String())
	}
}

func TestConfigRenderCommand(t *testing.T) {
	configPath := writeDefaultDeployment(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"--config", configPath, "config", "render"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"services:", "tardigrade:", "bearclaw-web:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered output missing %q:\n%s", want, out)
		}
	}
}

func TestValidateCommandRejectsUnknownModule(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "edge.yml")
	if err := os.WriteFile(configPath, []byte(`
apiVersion: bare.systems/v1alpha1
kind: EdgeDeployment
metadata:
  name: test
spec:
  channel: stable
  projectName: bare-systems
  runtime:
    profiles: [core]
  modules:
    core:
      enabled: true
    bogus:
      enabled: true
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"--config", configPath, "validate"}, &stdout, &stderr)

	if code != apperrors.ExitConfig {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitConfig)
	}
	if !strings.Contains(stderr.String(), `unknown module "bogus"`) {
		t.Fatalf("stderr missing validation error: %q", stderr.String())
	}
}

func TestInstallCommandWritesComposeAndRunsPull(t *testing.T) {
	projectDir := t.TempDir()
	runner := newCLIFakeRunner()
	app := New()
	app.runner = runner
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", projectDir, "install"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	composeFile := filepath.Join(projectDir, "compose", "generated.compose.yml")
	if _, err := os.Stat(composeFile); err != nil {
		t.Fatalf("compose file not written: %v", err)
	}
	if !runner.sawArgs("config -q") {
		t.Fatalf("docker compose config -q was not run: %#v", runner.commands)
	}
	if !runner.sawArgs("pull") {
		t.Fatalf("docker compose pull was not run: %#v", runner.commands)
	}
	if !runner.sawComposeDir(filepath.Join(projectDir, "compose")) {
		t.Fatalf("compose commands did not run from resolved project dir: %#v", runner.commands)
	}
}

func TestStatusJSONExposesRuntimeState(t *testing.T) {
	runner := newCLIFakeRunner()
	app := New()
	app.runner = runner
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", t.TempDir(), "--json", "status"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{`"command": "status"`, `"total": 1`, `"service": "tardigrade"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("status JSON missing %q:\n%s", want, out)
		}
	}
}

func TestLogsSupportsOptionalService(t *testing.T) {
	runner := newCLIFakeRunner()
	app := New()
	app.runner = runner
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", t.TempDir(), "logs", "tardigrade"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	if !strings.Contains(stdout.String(), "log line") {
		t.Fatalf("stdout missing logs: %q", stdout.String())
	}
	if !runner.sawArgs("logs --tail 200 tardigrade") {
		t.Fatalf("logs command did not include service: %#v", runner.commands)
	}
}

func TestRuntimeCommandReportsMissingDocker(t *testing.T) {
	runner := newCLIFakeRunner()
	runner.missingDocker = true
	app := New()
	app.runner = runner
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", t.TempDir(), "start"}, &stdout, &stderr)

	if code != apperrors.ExitPrereq {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitPrereq)
	}
	if !strings.Contains(stderr.String(), "Docker CLI not found") {
		t.Fatalf("stderr missing Docker remediation: %q", stderr.String())
	}
}

func writeDefaultDeployment(t *testing.T) string {
	t.Helper()
	data, err := deploymentconfig.DefaultDeploymentYAML()
	if err != nil {
		t.Fatalf("DefaultDeploymentYAML returned error: %v", err)
	}
	path := filepath.Join(t.TempDir(), "edge.yml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

type cliFakeRunner struct {
	commands      []edgeruntime.Command
	missingDocker bool
}

func newCLIFakeRunner() *cliFakeRunner {
	return &cliFakeRunner{}
}

func (r *cliFakeRunner) Run(_ context.Context, command edgeruntime.Command) (edgeruntime.Result, error) {
	r.commands = append(r.commands, command)
	args := strings.Join(command.Args, " ")

	if command.Name == "docker" && args == "--version" {
		if r.missingDocker {
			return edgeruntime.Result{}, &exec.Error{Name: "docker", Err: exec.ErrNotFound}
		}
		return edgeruntime.Result{Stdout: "Docker version 1\n"}, nil
	}
	if command.Name == "docker" && args == "info" {
		return edgeruntime.Result{Stdout: "Server Version: 1\n"}, nil
	}
	if command.Name == "docker" && args == "compose version" {
		return edgeruntime.Result{Stdout: "Docker Compose version v2\n"}, nil
	}
	if strings.Contains(args, "ps --format json") {
		return edgeruntime.Result{Stdout: `[{"ID":"abc","Name":"edge-tardigrade-1","Service":"tardigrade","State":"running","Health":"healthy"}]`}, nil
	}
	if strings.Contains(args, "logs --tail 200") {
		return edgeruntime.Result{Stdout: "log line\n"}, nil
	}
	return edgeruntime.Result{Stdout: "ok\n"}, nil
}

func (r *cliFakeRunner) sawArgs(fragment string) bool {
	for _, command := range r.commands {
		if strings.Contains(strings.Join(command.Args, " "), fragment) {
			return true
		}
	}
	return false
}

func (r *cliFakeRunner) sawComposeDir(dir string) bool {
	sawDeploymentCompose := false
	for _, command := range r.commands {
		args := strings.Join(command.Args, " ")
		if len(command.Args) > 0 && command.Args[0] == "compose" && strings.Contains(args, "--project-directory") {
			sawDeploymentCompose = true
		}
		if len(command.Args) > 0 && command.Args[0] == "compose" && strings.Contains(args, "--project-directory") && command.Dir != dir {
			return false
		}
	}
	return sawDeploymentCompose
}
