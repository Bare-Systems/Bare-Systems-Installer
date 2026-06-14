package diagnostics

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"sort"
	"strings"
	"time"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/health"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
	edgeruntime "github.com/Bare-Systems/Bare-Systems-Installer/internal/runtime"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/version"
	"gopkg.in/yaml.v3"
)

const (
	DefaultLogLimitBytes = 64 * 1024
)

type Options struct {
	Deployment       config.Deployment
	ConfigSource     string
	EnvSource        string
	Env              config.Environment
	ComposeYAML      []byte
	ValidationResult config.ValidationResult
	Registry         modules.Registry
	Runner           edgeruntime.Runner
	HealthReport     health.Report
	Now              time.Time
	OutputDir        string
	LogLimitBytes    int
}

type Result struct {
	Path          string    `json:"path"`
	SizeBytes     int64     `json:"sizeBytes"`
	CreatedAt     time.Time `json:"createdAt"`
	LogLimitBytes int       `json:"logLimitBytes"`
	Sections      []string  `json:"sections"`
}

func Create(ctx context.Context, options Options) (Result, error) {
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	logLimit := options.LogLimitBytes
	if logLimit <= 0 {
		logLimit = DefaultLogLimitBytes
	}
	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(options.Deployment.Spec.Storage.Root, "bundles")
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create diagnostics bundle directory %s: %w", outputDir, err)
	}

	path := filepath.Join(outputDir, "diagnostics-"+now.Format("20060102-150405")+".tar.gz")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return Result{}, fmt.Errorf("create diagnostics bundle %s: %w", path, err)
	}

	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)

	writer := bundleWriter{tar: tarWriter, sections: map[string]bool{}}
	manifest := map[string]any{
		"createdAt":     now.Format(time.RFC3339),
		"cliVersion":    version.Current(),
		"configSource":  options.ConfigSource,
		"envSource":     options.EnvSource,
		"modules":       options.ValidationResult.EnabledModules,
		"profiles":      options.ValidationResult.Profiles,
		"logLimitBytes": logLimit,
		"redaction":     "secret-like values are replaced with " + Redacted,
	}
	if err := writer.addJSON("manifest.json", manifest); err != nil {
		return Result{}, err
	}
	if err := writer.addJSON("version.json", version.Current()); err != nil {
		return Result{}, err
	}
	if err := writer.addText("system/platform.txt", fmt.Sprintf("goos=%s\ngoarch=%s\n", stdruntime.GOOS, stdruntime.GOARCH)); err != nil {
		return Result{}, err
	}

	configYAML, err := yaml.Marshal(options.Deployment)
	if err != nil {
		return Result{}, fmt.Errorf("marshal deployment config for bundle: %w", err)
	}
	if err := writer.addRedacted("config/edge.redacted.yml", configYAML); err != nil {
		return Result{}, err
	}
	if err := writer.addRedacted("config/env.redacted", []byte(envText(options.Env))); err != nil {
		return Result{}, err
	}
	if err := writer.addRedacted("config/compose.rendered.yml", options.ComposeYAML); err != nil {
		return Result{}, err
	}
	for _, manifest := range options.Registry.All() {
		data, err := yaml.Marshal(manifest)
		if err != nil {
			return Result{}, fmt.Errorf("marshal module manifest %s: %w", manifest.Module.ID, err)
		}
		if err := writer.addRedacted("config/module-manifests/"+manifest.Module.ID+".yml", data); err != nil {
			return Result{}, err
		}
	}

	if err := writer.addJSON("health/doctor.json", options.HealthReport); err != nil {
		return Result{}, err
	}
	if err := writer.addJSON("health/checks.json", options.HealthReport.Checks); err != nil {
		return Result{}, err
	}

	runtimeFiles(ctx, &writer, options, logLimit)

	if err := writer.addJSON("portal/last-report.redacted.json", map[string]any{
		"status": "not-configured",
	}); err != nil {
		return Result{}, err
	}

	if err := tarWriter.Close(); err != nil {
		return Result{}, fmt.Errorf("close diagnostics tar: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return Result{}, fmt.Errorf("close diagnostics gzip: %w", err)
	}
	if err := file.Close(); err != nil {
		return Result{}, fmt.Errorf("close diagnostics bundle: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return Result{}, fmt.Errorf("stat diagnostics bundle: %w", err)
	}
	return Result{
		Path:          path,
		SizeBytes:     info.Size(),
		CreatedAt:     now,
		LogLimitBytes: logLimit,
		Sections:      writer.sectionList(),
	}, nil
}

func runtimeFiles(ctx context.Context, writer *bundleWriter, options Options, logLimit int) {
	report, err := edgeruntime.CheckDocker(ctx, options.Runner)
	_ = writer.addJSON("runtime/docker-prereqs.json", map[string]any{"report": report, "error": errorString(err)})
	if err != nil {
		return
	}

	manager := edgeruntime.Compose{
		Runner:      options.Runner,
		ProjectDir:  options.Deployment.ComposeProjectDirectory(),
		ProjectName: options.Deployment.ComposeProjectName(),
		ComposeFile: filepath.Join(options.Deployment.ComposeProjectDirectory(), "generated.compose.yml"),
		Profiles:    options.ValidationResult.Profiles,
	}
	ps, psErr := manager.PS(ctx, true)
	_ = writer.addRedacted("runtime/compose-ps.json", []byte(ps.Stdout+"\n"+ps.Stderr+"\n"+errorString(psErr)))

	for _, service := range enabledServices(options) {
		logs, logErr := manager.Logs(ctx, service)
		content := truncate(logs.Stdout+"\n"+logs.Stderr+"\n"+errorString(logErr), logLimit)
		_ = writer.addRedacted("logs/"+service+".log", []byte(content))
	}
}

func enabledServices(options Options) []string {
	services := []string{}
	for _, manifest := range options.Registry.All() {
		if !manifest.Module.Required && !options.Deployment.ModuleEnabled(manifest.Module.ID) {
			continue
		}
		for _, service := range manifest.Module.Services {
			services = append(services, service.ComposeService)
		}
	}
	sort.Strings(services)
	return services
}

func envText(env config.Environment) string {
	keys := env.Keys()
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(env[key])
		builder.WriteString("\n")
	}
	return builder.String()
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func truncate(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "\n[TRUNCATED]\n"
}

type bundleWriter struct {
	tar      *tar.Writer
	sections map[string]bool
}

func (w *bundleWriter) addJSON(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}
	return w.addRedacted(name, append(data, '\n'))
}

func (w *bundleWriter) addText(name string, value string) error {
	return w.addRedacted(name, []byte(value))
}

func (w *bundleWriter) addRedacted(name string, data []byte) error {
	return w.add(name, RedactBytes(data))
}

func (w *bundleWriter) add(name string, data []byte) error {
	header := &tar.Header{
		Name: name,
		Mode: 0o600,
		Size: int64(len(data)),
	}
	if err := w.tar.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header %s: %w", name, err)
	}
	if _, err := io.Copy(w.tar, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("write tar body %s: %w", name, err)
	}
	section, _, _ := strings.Cut(name, "/")
	w.sections[section] = true
	return nil
}

func (w *bundleWriter) sectionList() []string {
	sections := make([]string, 0, len(w.sections))
	for section := range w.sections {
		sections = append(sections, section)
	}
	sort.Strings(sections)
	return sections
}
