package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
)

func TestDecodeDeploymentStrictUnknownField(t *testing.T) {
	_, err := DecodeDeployment([]byte(`
apiVersion: bare.systems/v1alpha1
kind: EdgeDeployment
metadata:
  name: test
spec:
  channel: stable
  projectName: bare-systems
  unknown: true
`))
	if err == nil {
		t.Fatalf("expected unknown field error")
	}
}

func TestLoadDeploymentUsesDefaultWhenAllowed(t *testing.T) {
	result, err := LoadDeployment(filepath.Join(t.TempDir(), "missing.yml"), true)
	if err != nil {
		t.Fatalf("LoadDeployment returned error: %v", err)
	}
	if result.Source != "built-in-default" {
		t.Fatalf("Source = %q, want built-in-default", result.Source)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected warning for missing config")
	}
}

func TestValidateDefaultDeployment(t *testing.T) {
	deployment := DefaultDeployment()
	env := MergeEnv(DerivedEnv(deployment), Environment{})

	result, err := ValidateDeployment(deployment, env, modules.BuiltInRegistry())
	if err != nil {
		t.Fatalf("ValidateDeployment returned error: %v", err)
	}
	if len(result.EnabledModules) != 1 || result.EnabledModules[0] != "core" {
		t.Fatalf("EnabledModules = %#v, want [core]", result.EnabledModules)
	}
	if len(result.Profiles) != 1 || result.Profiles[0] != "core" {
		t.Fatalf("Profiles = %#v, want [core]", result.Profiles)
	}
}

func TestValidateRejectsUnknownModuleAndSecretEnv(t *testing.T) {
	deployment := DefaultDeployment()
	deployment.Spec.Modules["bogus"] = ModuleConfig{Enabled: true}
	env := MergeEnv(DerivedEnv(deployment), Environment{"PORTAL_TOKEN": "do-not-print"})

	_, err := ValidateDeployment(deployment, env, modules.BuiltInRegistry())
	if err == nil {
		t.Fatalf("expected validation error")
	}
	got := err.Error()
	for _, want := range []string{`unknown module "bogus"`, `.env key "PORTAL_TOKEN" looks secret-like`} {
		if !strings.Contains(got, want) {
			t.Fatalf("validation error missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "do-not-print") {
		t.Fatalf("validation error leaked secret value: %q", got)
	}
}

func TestValidateExternalOptionalModuleDoesNotRequireLocalConfig(t *testing.T) {
	deployment := DefaultDeployment()
	deployment.Spec.Modules["koala"] = ModuleConfig{
		Enabled: true,
		Deployment: ModuleDeploymentConfig{
			Mode: "external",
			URL:  "http://jetson:6705",
		},
	}
	deployment.Spec.Runtime.Profiles = []string{"core"}
	env := MergeEnv(DerivedEnv(deployment), Environment{})

	result, err := ValidateDeployment(deployment, env, modules.BuiltInRegistry())
	if err != nil {
		t.Fatalf("ValidateDeployment returned error: %v", err)
	}
	if strings.Join(result.EnabledModules, ",") != "core,koala" {
		t.Fatalf("EnabledModules = %#v, want [core koala]", result.EnabledModules)
	}
	if strings.Join(result.Profiles, ",") != "core" {
		t.Fatalf("Profiles = %#v, want [core]", result.Profiles)
	}
	if env["KOALA_URL"] != "http://jetson:6705" {
		t.Fatalf("KOALA_URL = %q, want external URL", env["KOALA_URL"])
	}
}

func TestValidateRejectsInvalidExternalDeployment(t *testing.T) {
	deployment := DefaultDeployment()
	deployment.Spec.Modules["core"] = ModuleConfig{
		Enabled: true,
		Deployment: ModuleDeploymentConfig{
			Mode: "external",
			URL:  "http://core.example",
		},
	}
	deployment.Spec.Modules["koala"] = ModuleConfig{
		Enabled: true,
		Deployment: ModuleDeploymentConfig{
			Mode: "external",
		},
	}
	deployment.Spec.Modules["polar"] = ModuleConfig{
		Enabled: true,
		Deployment: ModuleDeploymentConfig{
			Mode: "remote",
			URL:  "http://polar.example",
		},
	}

	_, err := ValidateDeployment(deployment, DerivedEnv(deployment), modules.BuiltInRegistry())
	if err == nil {
		t.Fatalf("expected validation error")
	}
	got := err.Error()
	for _, want := range []string{
		"core module cannot use external deployment mode",
		`external module "koala" requires deployment.url`,
		`module "polar" deployment.mode must be "local" or "external"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("validation error missing %q: %q", want, got)
		}
	}
}

func TestLoadEnvExplicitMissingFails(t *testing.T) {
	_, err := LoadEnv(filepath.Join(t.TempDir(), ".env"), true)
	if err == nil {
		t.Fatalf("expected explicit missing env to fail")
	}
}

func TestParseEnv(t *testing.T) {
	env, err := ParseEnv([]byte("A=1\n# comment\nB='two'\n"))
	if err != nil {
		t.Fatalf("ParseEnv returned error: %v", err)
	}
	if env["A"] != "1" || env["B"] != "two" {
		t.Fatalf("env = %#v", env)
	}
}

func TestLoadDeploymentFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "edge.yml")
	if err := os.WriteFile(path, []byte(`
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
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := LoadDeployment(path, false)
	if err != nil {
		t.Fatalf("LoadDeployment returned error: %v", err)
	}
	if result.Deployment.Metadata.Name != "test" {
		t.Fatalf("metadata.name = %q", result.Deployment.Metadata.Name)
	}
}
