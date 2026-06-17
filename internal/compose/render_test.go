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
	for _, want := range []string{"services:", "tardigrade:", "bearclaw-web:", "bearclaw-agent:", "secrets:", "portal-token:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered compose missing %q:\n%s", want, out)
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
	if !strings.Contains(string(data), "koala-agent:") {
		t.Fatalf("rendered compose missing koala service:\n%s", string(data))
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

func TestRenderResolvesImageRegistryAndPorts(t *testing.T) {
	deployment := config.DefaultDeployment()
	env := config.MergeEnv(config.DerivedEnv(deployment), config.Environment{
		"BARE_IMAGE_REGISTRY": "localhost:5000/bare",
		"BARE_IMAGE_TAG":      "homelab",
		"PUBLIC_HTTP_PORT":    "8080",
		"PUBLIC_HTTPS_PORT":   "8443",
	})

	data, err := Render(deployment, modules.BuiltInRegistry(), env)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		"image: localhost:5000/bare/tardigrade:homelab",
		"image: localhost:5000/bare/bearclaw-web:homelab",
		"image: localhost:5000/bare/bearclaw-agent:homelab",
		"- 8080:80",
		"- 8443:443",
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
		"TARDIGRADE_IMAGE":    "localhost:5000/custom/tardigrade:dev",
	})

	data, err := Render(deployment, modules.BuiltInRegistry(), env)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "image: localhost:5000/custom/tardigrade:dev") {
		t.Fatalf("rendered compose did not use service override:\n%s", out)
	}
	if !strings.Contains(out, "image: localhost:5000/bare/bearclaw-web:homelab") {
		t.Fatalf("rendered compose did not keep base registry for other services:\n%s", out)
	}
}

func TestValidateRenderedRejectsNoServices(t *testing.T) {
	if err := ValidateRendered([]byte("name: bare-systems\nservices: {}\n")); err == nil {
		t.Fatalf("expected validation error")
	}
}
