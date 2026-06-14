package health

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
	edgeruntime "github.com/Bare-Systems/Bare-Systems-Installer/internal/runtime"
)

func TestEvaluateReportsPassAndManifestHealth(t *testing.T) {
	deployment := config.DefaultDeployment()
	registry := modules.BuiltInRegistry()
	report := Evaluate(context.Background(), Options{
		Deployment:       deployment,
		ValidationResult: config.ValidationResult{Profiles: []string{"core"}},
		Registry:         registry,
		Runner:           healthyRunner{},
	})

	if report.HasFailures() {
		t.Fatalf("expected no failures: %#v", report)
	}
	if !hasCheck(report, "manifest-health:tardigrade", StatusPass) {
		t.Fatalf("expected manifest health check to pass: %#v", report.Checks)
	}
}

func TestEvaluateReportsDockerPrereqFailure(t *testing.T) {
	report := Evaluate(context.Background(), Options{
		Deployment:       config.DefaultDeployment(),
		ValidationResult: config.ValidationResult{Profiles: []string{"core"}},
		Registry:         modules.BuiltInRegistry(),
		Runner:           missingDockerRunner{},
	})

	if !report.HasFailures() {
		t.Fatalf("expected failures")
	}
	if !hasCheck(report, "docker-cli", StatusFail) {
		t.Fatalf("expected docker-cli failure: %#v", report.Checks)
	}
}

func TestConfigFailure(t *testing.T) {
	report := ConfigFailure(assertErr("bad config"))
	if !report.HasFailures() {
		t.Fatalf("expected config failure")
	}
	if !hasCheck(report, "config-valid", StatusFail) {
		t.Fatalf("expected config-valid failure: %#v", report.Checks)
	}
}

func hasCheck(report Report, name string, status Status) bool {
	for _, check := range report.Checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

type healthyRunner struct{}

func (healthyRunner) Run(_ context.Context, command edgeruntime.Command) (edgeruntime.Result, error) {
	switch strings.Join(command.Args, " ") {
	case "--version":
		return edgeruntime.Result{Stdout: "Docker version 1\n"}, nil
	case "info":
		return edgeruntime.Result{Stdout: "Server Version: 1\n"}, nil
	case "compose version":
		return edgeruntime.Result{Stdout: "Docker Compose version v2\n"}, nil
	}
	if strings.Contains(strings.Join(command.Args, " "), "ps --format json") {
		return edgeruntime.Result{Stdout: `[{"Service":"tardigrade","State":"running","Health":"healthy"},{"Service":"bearclaw-web","State":"running","Health":"healthy"},{"Service":"bearclaw-agent","State":"running","Health":"healthy"}]`}, nil
	}
	return edgeruntime.Result{}, nil
}

type missingDockerRunner struct{}

func (missingDockerRunner) Run(_ context.Context, command edgeruntime.Command) (edgeruntime.Result, error) {
	if strings.Join(command.Args, " ") == "--version" {
		return edgeruntime.Result{}, &exec.Error{Name: "docker", Err: exec.ErrNotFound}
	}
	return edgeruntime.Result{}, nil
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
