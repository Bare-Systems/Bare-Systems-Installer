package modules

import "testing"

func TestBuiltInRegistry(t *testing.T) {
	registry := BuiltInRegistry()
	for _, id := range []string{"core", "koala", "polar", "kodiak", "ursa"} {
		manifest, ok := registry.Get(id)
		if !ok {
			t.Fatalf("missing manifest %q", id)
		}
		if manifest.APIVersion != APIVersion {
			t.Fatalf("%s apiVersion = %q", id, manifest.APIVersion)
		}
		if len(manifest.Module.Profiles) == 0 {
			t.Fatalf("%s profiles missing", id)
		}
		if len(manifest.Module.Services) == 0 {
			t.Fatalf("%s services missing", id)
		}
	}
}

func TestCoreManifestModelsDockerServicesOnly(t *testing.T) {
	manifest, ok := BuiltInRegistry().Get("core")
	if !ok {
		t.Fatalf("core manifest missing")
	}
	if len(manifest.Module.Services) != 1 || manifest.Module.Services[0].ComposeService != "bear-claw-web" {
		t.Fatalf("core should declare only the Bear Claw Web Compose service: %#v", manifest.Module.Services)
	}
	if len(manifest.Module.Secrets) != 0 {
		t.Fatalf("core Compose services should not declare Tardigrade host secrets")
	}
	if len(manifest.Module.Config.Required) == 0 {
		t.Fatalf("core should declare required config")
	}
}
