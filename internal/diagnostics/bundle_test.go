package diagnostics

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/health"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
	edgeruntime "github.com/Bare-Systems/Bare-Systems-Installer/internal/runtime"
)

func TestCreateBundleRedactsAndTruncates(t *testing.T) {
	deployment := config.DefaultDeployment()
	deployment.Spec.Storage.Root = t.TempDir()
	deployment.Spec.Runtime.ComposeProjectDirectory = filepath.Join(deployment.Spec.Storage.Root, "compose")
	env := config.MergeEnv(config.DerivedEnv(deployment), config.Environment{"PORTAL_TOKEN": "do-not-print"})

	result, err := Create(context.Background(), Options{
		Deployment:       deployment,
		ConfigSource:     "test",
		EnvSource:        "test",
		Env:              env,
		ComposeYAML:      []byte("services:\n  app:\n    environment:\n      PORTAL_TOKEN: do-not-print\n"),
		ValidationResult: config.ValidationResult{EnabledModules: []string{"core"}, Profiles: []string{"core"}},
		Registry:         modules.BuiltInRegistry(),
		Runner:           bundleRunner{},
		HealthReport:     health.ConfigFailure(assertErr("token=do-not-print")),
		Now:              time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC),
		LogLimitBytes:    16,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.SizeBytes <= 0 {
		t.Fatalf("SizeBytes = %d", result.SizeBytes)
	}

	contents := readBundle(t, result.Path)
	joined := strings.Join(mapValues(contents), "\n")
	if strings.Contains(joined, "do-not-print") {
		t.Fatalf("bundle leaked secret value:\n%s", joined)
	}
	if !strings.Contains(joined, "[TRUNCATED]") {
		t.Fatalf("bundle missing truncation marker:\n%s", joined)
	}
	for _, want := range []string{"manifest.json", "config/compose.rendered.yml", "runtime/compose-ps.json", "health/doctor.json", "logs/tardigrade.log"} {
		if _, ok := contents[want]; !ok {
			t.Fatalf("bundle missing %s; entries=%v", want, keys(contents))
		}
	}
}

func readBundle(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open bundle: %v", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	contents := map[string]string{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		data, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("read tar entry %s: %v", header.Name, err)
		}
		contents[header.Name] = string(data)
	}
	return contents
}

func keys(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	return out
}

func mapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

type bundleRunner struct{}

func (bundleRunner) Run(_ context.Context, command edgeruntime.Command) (edgeruntime.Result, error) {
	args := strings.Join(command.Args, " ")
	switch args {
	case "--version":
		return edgeruntime.Result{Stdout: "Docker version 1\n"}, nil
	case "info":
		return edgeruntime.Result{Stdout: "Server Version: 1\n"}, nil
	case "compose version":
		return edgeruntime.Result{Stdout: "Docker Compose version v2\n"}, nil
	}
	if strings.Contains(args, "ps --format json") {
		return edgeruntime.Result{Stdout: `[{"Service":"tardigrade","State":"running","Health":"healthy"}]`}, nil
	}
	if strings.Contains(args, "logs") {
		return edgeruntime.Result{Stdout: "very long log line with token=do-not-print\n"}, nil
	}
	return edgeruntime.Result{}, nil
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
