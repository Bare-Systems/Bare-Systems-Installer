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

func TestValidateRenderedRejectsNoServices(t *testing.T) {
	if err := ValidateRendered([]byte("name: bare-systems\nservices: {}\n")); err == nil {
		t.Fatalf("expected validation error")
	}
}
