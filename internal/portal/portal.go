package portal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	deploymentconfig "github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/health"
	edgeruntime "github.com/Bare-Systems/Bare-Systems-Installer/internal/runtime"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/version"
)

const (
	enrollmentPath     = "/api/v1/devices/enroll"
	heartbeatPath      = "/api/v1/devices/%s/heartbeat"
	IdentityFileName   = "device-identity.json"
	CredentialFileName = "device-token"
)

type Client interface {
	Enroll(ctx context.Context, request EnrollmentRequest) (EnrollmentResponse, error)
	Report(ctx context.Context, payload ReportPayload, deviceToken string) error
}

type HTTPClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	retry      RetryPolicy
}

type RetryPolicy struct {
	Attempts int
	Delay    time.Duration
}

type ClientOption func(*HTTPClient)

func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *HTTPClient) {
		if client != nil {
			c.httpClient = client
		}
	}
}

func WithRetryPolicy(policy RetryPolicy) ClientOption {
	return func(c *HTTPClient) {
		c.retry = policy
	}
}

func NewHTTPClient(baseURL string, options ...ClientOption) (*HTTPClient, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("portal URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse portal URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("portal URL must include scheme and host: %s", baseURL)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""

	client := &HTTPClient{
		baseURL:    parsed,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		retry:      RetryPolicy{Attempts: 2, Delay: 200 * time.Millisecond},
	}
	for _, option := range options {
		option(client)
	}
	if client.retry.Attempts <= 0 {
		client.retry.Attempts = 1
	}
	return client, nil
}

type EnrollmentRequest struct {
	EnrollmentToken    string `json:"enrollment_token"`
	Hostname           string `json:"hostname"`
	Platform           string `json:"platform"`
	Arch               string `json:"arch"`
	BareSystemsVersion string `json:"bare_systems_version"`
}

type EnrollmentResponse struct {
	DeviceID    string `json:"device_id"`
	DeviceToken string `json:"device_token"`
	PortalURL   string `json:"portal_url"`
}

func (c *HTTPClient) Enroll(ctx context.Context, request EnrollmentRequest) (EnrollmentResponse, error) {
	if strings.TrimSpace(request.EnrollmentToken) == "" {
		return EnrollmentResponse{}, errors.New("enrollment token is required")
	}

	var response EnrollmentResponse
	if err := c.postJSONWithAttempts(ctx, enrollmentPath, "", request, &response, 1); err != nil {
		return EnrollmentResponse{}, err
	}
	if strings.TrimSpace(response.DeviceID) == "" {
		return EnrollmentResponse{}, errors.New("portal enrollment response missing device_id")
	}
	if strings.TrimSpace(response.DeviceToken) == "" {
		return EnrollmentResponse{}, errors.New("portal enrollment response missing device_token")
	}
	if strings.TrimSpace(response.PortalURL) == "" {
		response.PortalURL = c.baseURL.String()
	}
	return response, nil
}

func (c *HTTPClient) Report(ctx context.Context, payload ReportPayload, deviceToken string) error {
	if strings.TrimSpace(payload.DeviceID) == "" {
		return errors.New("device ID is required")
	}
	if strings.TrimSpace(deviceToken) == "" {
		return errors.New("device token is required")
	}
	path := fmt.Sprintf(heartbeatPath, url.PathEscape(payload.DeviceID))
	return c.postJSON(ctx, path, deviceToken, payload, nil)
}

func (c *HTTPClient) postJSON(ctx context.Context, path string, bearerToken string, requestValue any, responseValue any) error {
	return c.postJSONWithAttempts(ctx, path, bearerToken, requestValue, responseValue, c.retry.Attempts)
}

func (c *HTTPClient) postJSONWithAttempts(ctx context.Context, path string, bearerToken string, requestValue any, responseValue any, attempts int) error {
	data, err := json.Marshal(requestValue)
	if err != nil {
		return fmt.Errorf("marshal portal request: %w", err)
	}
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(path), bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("build portal request: %w", err)
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Content-Type", "application/json")
		if bearerToken != "" {
			request.Header.Set("Authorization", "Bearer "+bearerToken)
		}

		response, err := c.httpClient.Do(request)
		if err != nil {
			lastErr = fmt.Errorf("portal request failed: %w", err)
			if attempt < attempts {
				sleep(ctx, c.retry.Delay)
				continue
			}
			return lastErr
		}

		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
		closeErr := response.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read portal response: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close portal response: %w", closeErr)
		}

		if response.StatusCode >= 200 && response.StatusCode <= 299 {
			if responseValue == nil || len(strings.TrimSpace(string(body))) == 0 {
				return nil
			}
			if err := json.Unmarshal(body, responseValue); err != nil {
				return fmt.Errorf("decode portal response: %w", err)
			}
			return nil
		}

		lastErr = ResponseError{StatusCode: response.StatusCode}
		if response.StatusCode >= 500 && attempt < attempts {
			sleep(ctx, c.retry.Delay)
			continue
		}
		return lastErr
	}
	return lastErr
}

func (c *HTTPClient) endpoint(path string) string {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	return endpoint.String()
}

func sleep(ctx context.Context, delay time.Duration) {
	if delay <= 0 {
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

type ResponseError struct {
	StatusCode int
}

func (e ResponseError) Error() string {
	return fmt.Sprintf("portal returned HTTP %d", e.StatusCode)
}

func IsAuthError(err error) bool {
	var responseErr ResponseError
	if !errors.As(err, &responseErr) {
		return false
	}
	return responseErr.StatusCode == http.StatusUnauthorized || responseErr.StatusCode == http.StatusForbidden
}

type DeviceIdentity struct {
	DeviceID       string    `json:"deviceId"`
	PortalURL      string    `json:"portalUrl"`
	CredentialFile string    `json:"credentialFile"`
	EnrolledAt     time.Time `json:"enrolledAt"`
}

func StateDir(deployment deploymentconfig.Deployment) string {
	root := strings.TrimSpace(deployment.Spec.Storage.Root)
	if root == "" {
		return deploymentconfig.DefaultStateDir
	}
	return filepath.Join(root, "state")
}

func IdentityPath(stateDir string) string {
	return filepath.Join(stateDir, IdentityFileName)
}

func CredentialPath(stateDir string) string {
	return filepath.Join(stateDir, CredentialFileName)
}

func ReadBootstrapToken(path string) (string, error) {
	return readRequiredSecret(path, "enrollment token")
}

func ReadDeviceToken(path string) (string, error) {
	return readRequiredSecret(path, "device token")
}

func readRequiredSecret(path string, name string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s file %s: %w", name, path, err)
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("%s file %s is empty", name, path)
	}
	return value, nil
}

func PersistEnrollment(stateDir string, response EnrollmentResponse, now time.Time) (DeviceIdentity, error) {
	if strings.TrimSpace(response.DeviceID) == "" {
		return DeviceIdentity{}, errors.New("device ID is required")
	}
	if strings.TrimSpace(response.DeviceToken) == "" {
		return DeviceIdentity{}, errors.New("device token is required")
	}
	if strings.TrimSpace(response.PortalURL) == "" {
		return DeviceIdentity{}, errors.New("portal URL is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return DeviceIdentity{}, fmt.Errorf("create Portal state directory %s: %w", stateDir, err)
	}
	if err := os.Chmod(stateDir, 0o700); err != nil {
		return DeviceIdentity{}, fmt.Errorf("set Portal state directory permissions %s: %w", stateDir, err)
	}

	credentialFile := CredentialPath(stateDir)
	if err := writeRestrictedFile(credentialFile, []byte(response.DeviceToken+"\n")); err != nil {
		return DeviceIdentity{}, fmt.Errorf("write device credential %s: %w", credentialFile, err)
	}

	identity := DeviceIdentity{
		DeviceID:       response.DeviceID,
		PortalURL:      response.PortalURL,
		CredentialFile: credentialFile,
		EnrolledAt:     now,
	}
	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return DeviceIdentity{}, fmt.Errorf("marshal device identity: %w", err)
	}
	if err := writeRestrictedFile(IdentityPath(stateDir), append(data, '\n')); err != nil {
		return DeviceIdentity{}, fmt.Errorf("write device identity: %w", err)
	}
	return identity, nil
}

func LoadIdentity(stateDir string) (DeviceIdentity, error) {
	path := IdentityPath(stateDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return DeviceIdentity{}, fmt.Errorf("read device identity %s: %w", path, err)
	}
	var identity DeviceIdentity
	if err := json.Unmarshal(data, &identity); err != nil {
		return DeviceIdentity{}, fmt.Errorf("parse device identity %s: %w", path, err)
	}
	if strings.TrimSpace(identity.DeviceID) == "" {
		return DeviceIdentity{}, fmt.Errorf("device identity %s missing deviceId", path)
	}
	if strings.TrimSpace(identity.PortalURL) == "" {
		return DeviceIdentity{}, fmt.Errorf("device identity %s missing portalUrl", path)
	}
	if strings.TrimSpace(identity.CredentialFile) == "" {
		return DeviceIdentity{}, fmt.Errorf("device identity %s missing credentialFile", path)
	}
	return identity, nil
}

func writeRestrictedFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

type ReportPayload struct {
	DeviceID       string            `json:"deviceId"`
	Timestamp      time.Time         `json:"timestamp"`
	CLIVersion     version.Info      `json:"cliVersion"`
	ConfigRevision string            `json:"configRevision"`
	Deployment     DeploymentSummary `json:"deployment"`
	EnabledModules []string          `json:"enabledModules"`
	Profiles       []string          `json:"profiles"`
	ServiceStatus  ServiceStatus     `json:"serviceStatus"`
	HealthSummary  health.Summary    `json:"healthSummary"`
	HealthChecks   []health.Check    `json:"healthChecks,omitempty"`
	ReportWarnings []string          `json:"reportWarnings,omitempty"`
}

type DeploymentSummary struct {
	Name     string `json:"name"`
	Customer string `json:"customer,omitempty"`
	Channel  string `json:"channel"`
}

type ServiceStatus struct {
	Summary    edgeruntime.StateSummary `json:"summary"`
	Containers []edgeruntime.Container  `json:"containers"`
	Error      string                   `json:"error,omitempty"`
}

type ReportInput struct {
	Identity         DeviceIdentity
	Deployment       deploymentconfig.Deployment
	ValidationResult deploymentconfig.ValidationResult
	ComposeYAML      []byte
	RuntimeState     edgeruntime.RuntimeState
	RuntimeError     error
	HealthReport     health.Report
	CLIVersion       version.Info
	Timestamp        time.Time
	Warnings         []string
}

func BuildReportPayload(input ReportInput) ReportPayload {
	timestamp := input.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	timestamp = timestamp.UTC()

	serviceStatus := ServiceStatus{
		Summary:    input.RuntimeState.Summary,
		Containers: append([]edgeruntime.Container(nil), input.RuntimeState.Containers...),
	}
	if input.RuntimeError != nil {
		serviceStatus.Error = input.RuntimeError.Error()
	}

	return ReportPayload{
		DeviceID:       input.Identity.DeviceID,
		Timestamp:      timestamp,
		CLIVersion:     input.CLIVersion,
		ConfigRevision: ConfigRevision(input.ComposeYAML),
		Deployment: DeploymentSummary{
			Name:     input.Deployment.Metadata.Name,
			Customer: input.Deployment.Metadata.Customer,
			Channel:  input.Deployment.Spec.Channel,
		},
		EnabledModules: append([]string(nil), input.ValidationResult.EnabledModules...),
		Profiles:       append([]string(nil), input.ValidationResult.Profiles...),
		ServiceStatus:  serviceStatus,
		HealthSummary:  input.HealthReport.Summary,
		HealthChecks:   append([]health.Check(nil), input.HealthReport.Checks...),
		ReportWarnings: append([]string(nil), input.Warnings...),
	}
}

func ConfigRevision(composeYAML []byte) string {
	sum := sha256.Sum256(composeYAML)
	return "sha256:" + hex.EncodeToString(sum[:])
}

type SpoolEntry struct {
	CreatedAt time.Time     `json:"createdAt"`
	Error     string        `json:"error"`
	Payload   ReportPayload `json:"payload"`
}

type SpoolResult struct {
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"createdAt"`
}

func SpoolReportFailure(stateDir string, payload ReportPayload, sendErr error, now time.Time) (SpoolResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	spoolDir := filepath.Join(stateDir, "reports", "spool")
	if err := os.MkdirAll(spoolDir, 0o700); err != nil {
		return SpoolResult{}, fmt.Errorf("create Portal report spool directory %s: %w", spoolDir, err)
	}
	if err := os.Chmod(spoolDir, 0o700); err != nil {
		return SpoolResult{}, fmt.Errorf("set Portal report spool directory permissions %s: %w", spoolDir, err)
	}

	entry := SpoolEntry{
		CreatedAt: now,
		Error:     errorString(sendErr),
		Payload:   payload,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return SpoolResult{}, fmt.Errorf("marshal Portal report spool entry: %w", err)
	}
	path := filepath.Join(spoolDir, "report-"+now.Format("20060102-150405")+".json")
	if err := writeRestrictedFile(path, append(data, '\n')); err != nil {
		return SpoolResult{}, fmt.Errorf("write Portal report spool entry %s: %w", path, err)
	}
	return SpoolResult{Path: path, CreatedAt: now}, nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
