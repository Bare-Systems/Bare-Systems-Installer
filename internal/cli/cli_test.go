package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	deploymentconfig "github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/output"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/portal"
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
	if !strings.Contains(stdout.String(), "--image-registry") {
		t.Fatalf("help output missing init image registry flag: %q", stdout.String())
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

func TestInitCommandCreatesProjectDefaults(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "edge")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{"--project-dir", projectDir, "init"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	for _, path := range []string{
		projectDir,
		filepath.Join(projectDir, "compose"),
		filepath.Join(projectDir, "state"),
		filepath.Join(projectDir, "bundles"),
		filepath.Join(projectDir, "manifests"),
		filepath.Join(projectDir, "backups"),
		filepath.Join(projectDir, "edge.yml"),
		filepath.Join(projectDir, ".env"),
		filepath.Join(projectDir, "compose", "generated.compose.yml"),
		filepath.Join(projectDir, "manifests", "README.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected init path %s: %v", path, err)
		}
	}
	assertFileMode(t, filepath.Join(projectDir, "state"), 0o700)

	stdout.Reset()
	stderr.Reset()
	code = New().Run([]string{"--project-dir", projectDir, "--json", "validate"}, &stdout, &stderr)
	if code != apperrors.ExitOK {
		t.Fatalf("validate exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	for _, want := range []string{`"command": "validate"`, filepath.Join(projectDir, "edge.yml"), `"enabledModules": [`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("validate output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestInitCommandDoesNotOverwriteWithoutForce(t *testing.T) {
	projectDir := t.TempDir()
	app := New()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", projectDir, "init"}, &stdout, &stderr)
	if code != apperrors.ExitOK {
		t.Fatalf("init exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	configPath := filepath.Join(projectDir, "edge.yml")
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	custom := append(append([]byte{}, original...), []byte("\n# homelab custom note\n")...)
	if err := os.WriteFile(configPath, custom, 0o600); err != nil {
		t.Fatalf("write custom config: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"--project-dir", projectDir, "init"}, &stdout, &stderr)
	if code != apperrors.ExitOK {
		t.Fatalf("second init exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after idempotent init: %v", err)
	}
	if !strings.Contains(string(data), "homelab custom note") {
		t.Fatalf("init overwrote existing config without --force:\n%s", string(data))
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"--project-dir", projectDir, "init", "--force"}, &stdout, &stderr)
	if code != apperrors.ExitOK {
		t.Fatalf("force init exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after force init: %v", err)
	}
	if strings.Contains(string(data), "homelab custom note") {
		t.Fatalf("init --force did not regenerate config:\n%s", string(data))
	}
}

func TestInitCommandSupportsLocalImageRegistry(t *testing.T) {
	projectDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New().Run([]string{
		"--project-dir", projectDir,
		"init",
		"--image-registry", "localhost:5000/bare",
		"--image-tag", "homelab",
	}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	envData, err := os.ReadFile(filepath.Join(projectDir, ".env"))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	for _, want := range []string{
		"BARE_IMAGE_REGISTRY=localhost:5000/bare",
		"BARE_IMAGE_TAG=homelab",
		"# TARDIGRADE_IMAGE=",
	} {
		if !strings.Contains(string(envData), want) {
			t.Fatalf(".env missing %q:\n%s", want, string(envData))
		}
	}
	composeData, err := os.ReadFile(filepath.Join(projectDir, "compose", "generated.compose.yml"))
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	for _, want := range []string{
		"image: localhost:5000/bare/tardigrade:homelab",
		"image: localhost:5000/bare/bearclaw-web:homelab",
		"image: localhost:5000/bare/bearclaw-agent:homelab",
	} {
		if !strings.Contains(string(composeData), want) {
			t.Fatalf("compose missing %q:\n%s", want, string(composeData))
		}
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

func TestConfigRenderWriteCommand(t *testing.T) {
	projectDir := t.TempDir()
	app := New()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", projectDir, "init"}, &stdout, &stderr)
	if code != apperrors.ExitOK {
		t.Fatalf("init exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	composeFile := filepath.Join(projectDir, "compose", "generated.compose.yml")
	if err := os.Remove(composeFile); err != nil {
		t.Fatalf("remove compose file: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"--project-dir", projectDir, "config", "render", "--write"}, &stdout, &stderr)
	if code != apperrors.ExitOK {
		t.Fatalf("config render --write exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	data, err := os.ReadFile(composeFile)
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	for _, want := range []string{"services:", "tardigrade:", "bearclaw-web:"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("compose file missing %q:\n%s", want, string(data))
		}
	}
	if !strings.Contains(stdout.String(), composeFile) {
		t.Fatalf("stdout missing compose file path: %q", stdout.String())
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
	for _, want := range []string{`"command": "status"`, `"total": 3`, `"service": "tardigrade"`} {
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

func TestDoctorReportsChecks(t *testing.T) {
	runner := newCLIFakeRunner()
	app := New()
	app.runner = runner
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", t.TempDir(), "doctor"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q stdout=%q", code, apperrors.ExitOK, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{"PASS config-valid", "PASS docker-cli", "manifest-health:tardigrade"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorJSONReportsFailures(t *testing.T) {
	runner := newCLIFakeRunner()
	runner.missingDocker = true
	app := New()
	app.runner = runner
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", t.TempDir(), "--json", "doctor"}, &stdout, &stderr)

	if code != apperrors.ExitHealth {
		t.Fatalf("exit code = %d, want %d", code, apperrors.ExitHealth)
	}
	out := stdout.String()
	for _, want := range []string{`"command": "doctor"`, `"code": "ERR_HEALTH"`, `"name": "docker-cli"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor JSON missing %q:\n%s", want, out)
		}
	}
}

func TestBundleCommandCreatesArchive(t *testing.T) {
	runner := newCLIFakeRunner()
	app := New()
	app.runner = runner
	app.clock = func() time.Time {
		return time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)
	}
	projectDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", projectDir, "--json", "bundle"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"command": "bundle"`) {
		t.Fatalf("bundle JSON missing command: %s", stdout.String())
	}
	expected := filepath.Join(projectDir, "bundles", "diagnostics-20260613-123456.tar.gz")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("bundle not created at %s: %v", expected, err)
	}
}

func TestEnrollCommandWithMockPortal(t *testing.T) {
	projectDir := t.TempDir()
	tokenPath := filepath.Join(projectDir, "bootstrap.token")
	if err := os.WriteFile(tokenPath, []byte("bootstrap-secret\n"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	var enrollmentRequest portal.EnrollmentRequest
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/devices/enroll" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&enrollmentRequest); err != nil {
			t.Fatalf("decode enrollment: %v", err)
		}
		_ = json.NewEncoder(w).Encode(portal.EnrollmentResponse{
			DeviceID:    "dev_abc123",
			DeviceToken: "device-secret",
			PortalURL:   server.URL,
		})
	}))
	defer server.Close()

	app := New()
	app.clock = func() time.Time {
		return time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", projectDir, "enroll", "--portal", server.URL, "--token-file", tokenPath}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, apperrors.ExitOK, stderr.String())
	}
	if enrollmentRequest.EnrollmentToken != "bootstrap-secret" {
		t.Fatalf("EnrollmentToken = %q", enrollmentRequest.EnrollmentToken)
	}
	for _, secret := range []string{"bootstrap-secret", "device-secret"} {
		if strings.Contains(stdout.String()+stderr.String(), secret) {
			t.Fatalf("command output leaked %q: stdout=%q stderr=%q", secret, stdout.String(), stderr.String())
		}
	}
	identity, err := portal.LoadIdentity(filepath.Join(projectDir, "state"))
	if err != nil {
		t.Fatalf("LoadIdentity returned error: %v", err)
	}
	if identity.DeviceID != "dev_abc123" || identity.PortalURL != server.URL {
		t.Fatalf("identity = %#v", identity)
	}
	assertFileMode(t, identity.CredentialFile, 0o600)
}

func TestReportCommandSendsPortalHeartbeat(t *testing.T) {
	projectDir := t.TempDir()
	var payload portal.ReportPayload
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/devices/dev_abc123/heartbeat" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer device-secret" {
			t.Fatalf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode heartbeat: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if _, err := portal.PersistEnrollment(filepath.Join(projectDir, "state"), portal.EnrollmentResponse{
		DeviceID:    "dev_abc123",
		DeviceToken: "device-secret",
		PortalURL:   server.URL,
	}, time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("PersistEnrollment returned error: %v", err)
	}
	runner := newCLIFakeRunner()
	app := New()
	app.runner = runner
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", projectDir, "--json", "report"}, &stdout, &stderr)

	if code != apperrors.ExitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q stdout=%q", code, apperrors.ExitOK, stderr.String(), stdout.String())
	}
	if payload.DeviceID != "dev_abc123" {
		t.Fatalf("DeviceID = %q", payload.DeviceID)
	}
	if !slicesContains(payload.EnabledModules, "core") {
		t.Fatalf("enabled modules = %#v", payload.EnabledModules)
	}
	if payload.ServiceStatus.Summary.Total != 3 {
		t.Fatalf("service summary = %#v", payload.ServiceStatus.Summary)
	}
	if payload.HealthSummary.Total == 0 {
		t.Fatalf("health summary was not populated: %#v", payload.HealthSummary)
	}
	if strings.Contains(stdout.String()+stderr.String(), "device-secret") {
		t.Fatalf("command output leaked device token")
	}
}

func TestReportCommandSpoolsFailedPortalReport(t *testing.T) {
	projectDir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporarily unavailable", http.StatusInternalServerError)
	}))
	defer server.Close()
	if _, err := portal.PersistEnrollment(filepath.Join(projectDir, "state"), portal.EnrollmentResponse{
		DeviceID:    "dev_abc123",
		DeviceToken: "device-secret",
		PortalURL:   server.URL,
	}, time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("PersistEnrollment returned error: %v", err)
	}

	runner := newCLIFakeRunner()
	app := New()
	app.runner = runner
	app.clock = func() time.Time {
		return time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := app.Run([]string{"--project-dir", projectDir, "--json", "report"}, &stdout, &stderr)

	if code != apperrors.ExitNetwork {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, apperrors.ExitNetwork, stdout.String(), stderr.String())
	}
	spoolPath := filepath.Join(projectDir, "state", "reports", "spool", "report-20260613-123456.json")
	if _, err := os.Stat(spoolPath); err != nil {
		t.Fatalf("spool file missing: %v", err)
	}
	assertFileMode(t, spoolPath, 0o600)
	if !strings.Contains(stdout.String(), `"spooled": true`) {
		t.Fatalf("report JSON missing spool marker: %s", stdout.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), "device-secret") {
		t.Fatalf("command output leaked device token")
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"--project-dir", projectDir, "--json", "status"}, &stdout, &stderr)
	if code != apperrors.ExitOK {
		t.Fatalf("status after failed report exit code = %d, stderr=%q", code, stderr.String())
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
		return edgeruntime.Result{Stdout: `[{"ID":"abc","Name":"edge-tardigrade-1","Service":"tardigrade","State":"running","Health":"healthy"},{"Service":"bearclaw-web","State":"running","Health":"healthy"},{"Service":"bearclaw-agent","State":"running","Health":"healthy"}]`}, nil
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

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode %s = %#o, want %#o", path, got, want)
	}
}

func slicesContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
