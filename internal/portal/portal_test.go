package portal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	deploymentconfig "github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/health"
	edgeruntime "github.com/Bare-Systems/Bare-Systems-Installer/internal/runtime"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/version"
)

func TestHTTPClientEnrollPostsContract(t *testing.T) {
	var got EnrollmentRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != enrollmentPath {
			t.Fatalf("path = %s, want %s", r.URL.Path, enrollmentPath)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(EnrollmentResponse{
			DeviceID:    "dev_abc123",
			DeviceToken: "device-secret",
			PortalURL:   serverURL(r),
		})
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, WithRetryPolicy(RetryPolicy{Attempts: 1}))
	if err != nil {
		t.Fatalf("NewHTTPClient returned error: %v", err)
	}
	response, err := client.Enroll(context.Background(), EnrollmentRequest{
		EnrollmentToken:    "bootstrap-token",
		Hostname:           "edge-01",
		Platform:           "linux",
		Arch:               "amd64",
		BareSystemsVersion: "0.1.0",
	})
	if err != nil {
		t.Fatalf("Enroll returned error: %v", err)
	}
	if response.DeviceID != "dev_abc123" {
		t.Fatalf("DeviceID = %q", response.DeviceID)
	}
	if got.EnrollmentToken != "bootstrap-token" || got.Hostname != "edge-01" {
		t.Fatalf("request = %#v", got)
	}
}

func TestHTTPClientReportUsesBearerToken(t *testing.T) {
	var got ReportPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/devices/dev_abc123/heartbeat" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if gotAuth := r.Header.Get("Authorization"); gotAuth != "Bearer device-secret" {
			t.Fatalf("Authorization = %q", gotAuth)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, WithRetryPolicy(RetryPolicy{Attempts: 1}))
	if err != nil {
		t.Fatalf("NewHTTPClient returned error: %v", err)
	}
	payload := ReportPayload{DeviceID: "dev_abc123", ConfigRevision: "sha256:abc"}
	if err := client.Report(context.Background(), payload, "device-secret"); err != nil {
		t.Fatalf("Report returned error: %v", err)
	}
	if got.DeviceID != "dev_abc123" || got.ConfigRevision != "sha256:abc" {
		t.Fatalf("payload = %#v", got)
	}
}

func TestPersistEnrollmentWritesRestrictedIdentityAndCredential(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)
	stateDir := t.TempDir()

	identity, err := PersistEnrollment(stateDir, EnrollmentResponse{
		DeviceID:    "dev_abc123",
		DeviceToken: "device-secret",
		PortalURL:   "https://portal.example.test",
	}, now)
	if err != nil {
		t.Fatalf("PersistEnrollment returned error: %v", err)
	}
	if identity.DeviceID != "dev_abc123" {
		t.Fatalf("DeviceID = %q", identity.DeviceID)
	}

	loaded, err := LoadIdentity(stateDir)
	if err != nil {
		t.Fatalf("LoadIdentity returned error: %v", err)
	}
	if loaded.DeviceID != identity.DeviceID || loaded.CredentialFile != identity.CredentialFile {
		t.Fatalf("loaded identity = %#v, want %#v", loaded, identity)
	}
	deviceToken, err := ReadDeviceToken(identity.CredentialFile)
	if err != nil {
		t.Fatalf("ReadDeviceToken returned error: %v", err)
	}
	if deviceToken != "device-secret" {
		t.Fatalf("device token = %q", deviceToken)
	}

	assertMode(t, identity.CredentialFile, 0o600)
	assertMode(t, IdentityPath(stateDir), 0o600)
	identityData, err := os.ReadFile(IdentityPath(stateDir))
	if err != nil {
		t.Fatalf("read identity: %v", err)
	}
	if strings.Contains(string(identityData), "device-secret") {
		t.Fatalf("identity file contains device token: %s", string(identityData))
	}
}

func TestBuildReportPayloadAndSpoolFailure(t *testing.T) {
	deployment := deploymentconfig.DefaultDeployment()
	deployment.Metadata.Customer = "customer-a"
	now := time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)
	state := edgeruntime.RuntimeState{
		Containers: []edgeruntime.Container{{Service: "bear-claw-web", State: "running", Health: "healthy"}},
		Summary: edgeruntime.StateSummary{
			Total:    1,
			ByState:  map[string]int{"running": 1},
			ByHealth: map[string]int{"healthy": 1},
		},
	}
	report := health.Report{
		Summary: health.Summary{
			Status:  health.StatusPass,
			Total:   1,
			Counts:  map[health.Status]int{health.StatusPass: 1},
			Message: "ok",
		},
	}

	payload := BuildReportPayload(ReportInput{
		Identity:         DeviceIdentity{DeviceID: "dev_abc123"},
		Deployment:       deployment,
		ValidationResult: deploymentconfig.ValidationResult{EnabledModules: []string{"core"}, Profiles: []string{"core"}},
		ComposeYAML:      []byte("services:\n  bear-claw-web: {}\n"),
		RuntimeState:     state,
		RuntimeError:     errors.New("runtime warning"),
		HealthReport:     report,
		CLIVersion:       version.Info{Version: "0.1.0", Commit: "abc", Date: "date"},
		Timestamp:        now,
	})
	if payload.DeviceID != "dev_abc123" {
		t.Fatalf("DeviceID = %q", payload.DeviceID)
	}
	if !strings.HasPrefix(payload.ConfigRevision, "sha256:") {
		t.Fatalf("ConfigRevision = %q", payload.ConfigRevision)
	}
	if payload.Deployment.Customer != "customer-a" {
		t.Fatalf("customer = %q", payload.Deployment.Customer)
	}
	if payload.ServiceStatus.Summary.Total != 1 || payload.HealthSummary.Status != health.StatusPass {
		t.Fatalf("payload = %#v", payload)
	}

	spool, err := SpoolReportFailure(t.TempDir(), payload, errors.New("portal unavailable"), now)
	if err != nil {
		t.Fatalf("SpoolReportFailure returned error: %v", err)
	}
	assertMode(t, spool.Path, 0o600)
	data, err := os.ReadFile(spool.Path)
	if err != nil {
		t.Fatalf("read spool: %v", err)
	}
	for _, want := range []string{"dev_abc123", "portal unavailable", "bear-claw-web"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("spool missing %q:\n%s", want, string(data))
		}
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode %s = %#o, want %#o", path, got, want)
	}
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}
