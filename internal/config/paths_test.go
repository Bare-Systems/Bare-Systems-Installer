package config

import "testing"

func TestDefaultPaths(t *testing.T) {
	paths := DefaultPaths()

	if paths.ConfigPath != DefaultConfigPath {
		t.Fatalf("ConfigPath = %q, want %q", paths.ConfigPath, DefaultConfigPath)
	}
	if paths.ProjectDir != DefaultProjectDir {
		t.Fatalf("ProjectDir = %q, want %q", paths.ProjectDir, DefaultProjectDir)
	}
	if paths.LogDir != DefaultLogDir {
		t.Fatalf("LogDir = %q, want %q", paths.LogDir, DefaultLogDir)
	}
}
