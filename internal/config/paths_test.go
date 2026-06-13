package config

import "testing"

func TestDefaultPaths(t *testing.T) {
	paths := DefaultPaths()

	if paths.ConfigPath != DefaultConfigPath {
		t.Fatalf("ConfigPath = %q, want %q", paths.ConfigPath, DefaultConfigPath)
	}
	if paths.EnvPath != DefaultEnvPath {
		t.Fatalf("EnvPath = %q, want %q", paths.EnvPath, DefaultEnvPath)
	}
	if paths.SecretsDir != DefaultSecretsDir {
		t.Fatalf("SecretsDir = %q, want %q", paths.SecretsDir, DefaultSecretsDir)
	}
	if paths.ProjectDir != DefaultProjectDir {
		t.Fatalf("ProjectDir = %q, want %q", paths.ProjectDir, DefaultProjectDir)
	}
	if paths.ComposeDir != DefaultComposeDir {
		t.Fatalf("ComposeDir = %q, want %q", paths.ComposeDir, DefaultComposeDir)
	}
	if paths.ManifestDir != DefaultManifestDir {
		t.Fatalf("ManifestDir = %q, want %q", paths.ManifestDir, DefaultManifestDir)
	}
	if paths.StateDir != DefaultStateDir {
		t.Fatalf("StateDir = %q, want %q", paths.StateDir, DefaultStateDir)
	}
	if paths.BundleDir != DefaultBundleDir {
		t.Fatalf("BundleDir = %q, want %q", paths.BundleDir, DefaultBundleDir)
	}
	if paths.LogDir != DefaultLogDir {
		t.Fatalf("LogDir = %q, want %q", paths.LogDir, DefaultLogDir)
	}
}
