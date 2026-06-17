package compose

import (
	"strings"
	"testing"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
)

func TestRenderDefaultDeployment(t *testing.T) {
	data, err := Render(config.DefaultDeployment(), modules.BuiltInRegistry())
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	out := string(data)
	for _, want := range []string{"services:", "bear-claw-web:", "127.0.0.1:8080:80", "http://localhost/up"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered compose missing %q:\n%s", want, out)
		}
	}
	for _, notWant := range []string{"tardigrade:", "bearclaw-agent:"} {
		if strings.Contains(out, notWant) {
			t.Fatalf("rendered compose should not include %q:\n%s", notWant, out)
		}
	}
	if strings.Contains(out, "portal-token: do-not-print") {
		t.Fatalf("render leaked a secret value:\n%s", out)
	}
	if err := ValidateRendered(data); err != nil {
		t.Fatalf("ValidateRendered returned error: %v", err)
	}
}

func TestRenderEnabledOptionalModule(t *testing.T) {
	deployment := config.DefaultDeployment()
	deployment.Spec.Modules["koala"] = config.ModuleConfig{Enabled: true}

	data, err := Render(deployment, modules.BuiltInRegistry())
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	for _, want := range []string{"koala-orchestrator:", "koala-worker:"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("rendered compose missing %q:\n%s", want, string(data))
		}
	}
}

func TestRenderUsesProvidedEnvironment(t *testing.T) {
	deployment := config.DefaultDeployment()
	env := config.MergeEnv(config.DerivedEnv(deployment), config.Environment{"ADMIN_BIND_ADDRESS": "127.0.0.1"})

	data, err := Render(deployment, modules.BuiltInRegistry(), env)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !strings.Contains(string(data), "ADMIN_BIND_ADDRESS: 127.0.0.1") {
		t.Fatalf("rendered compose did not use provided env:\n%s", string(data))
	}
}

func TestRenderResolvesImageRegistry(t *testing.T) {
	deployment := config.DefaultDeployment()
	deployment.Spec.Modules["koala"] = config.ModuleConfig{Enabled: true}
	env := config.MergeEnv(config.DerivedEnv(deployment), config.Environment{
		"BARE_IMAGE_REGISTRY": "localhost:5000/bare",
		"BARE_IMAGE_TAG":      "homelab",
	})

	data, err := Render(deployment, modules.BuiltInRegistry(), env)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		"image: localhost:5000/bare/bear-claw-web:homelab",
		"image: localhost:5000/bare/koala-orchestrator:homelab",
		"image: localhost:5000/bare/koala-worker:homelab",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered compose missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "${") {
		t.Fatalf("rendered compose kept unresolved interpolation:\n%s", out)
	}
}

func TestRenderUsesServiceImageOverride(t *testing.T) {
	deployment := config.DefaultDeployment()
	env := config.MergeEnv(config.DerivedEnv(deployment), config.Environment{
		"BARE_IMAGE_REGISTRY": "localhost:5000/bare",
		"BARE_IMAGE_TAG":      "homelab",
		"BEARCLAW_WEB_IMAGE":  "localhost:5000/custom/bear-claw-web:dev",
	})

	data, err := Render(deployment, modules.BuiltInRegistry(), env)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "image: localhost:5000/custom/bear-claw-web:dev") {
		t.Fatalf("rendered compose did not use service override:\n%s", out)
	}
	if strings.Contains(out, "image: localhost:5000/bare/bear-claw-web:homelab") {
		t.Fatalf("rendered compose used base registry instead of service override:\n%s", out)
	}
}

func TestValidateRenderedRejectsNoServices(t *testing.T) {
	if err := ValidateRendered([]byte("name: bare-systems\nservices: {}\n")); err == nil {
		t.Fatalf("expected validation error")
	}
}
