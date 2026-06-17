package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderSystemdUnitStartsComposeOnBoot(t *testing.T) {
	unit := RenderSystemdUnit(ServiceOptions{BinaryPath: "/usr/bin/bare-systems"})
	for _, want := range []string{
		"WantedBy=multi-user.target",
		"After=docker.service network-online.target",
		"ExecStart=/usr/bin/bare-systems start",
		"ExecStop=/usr/bin/bare-systems stop",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
}

func TestRenderSystemdUnitPreservesExplicitProjectDir(t *testing.T) {
	unit := RenderSystemdUnit(ServiceOptions{BinaryPath: "/usr/bin/bare-systems", ProjectDir: "/tmp/edge"})
	for _, want := range []string{
		"ExecStart=/usr/bin/bare-systems --project-dir /tmp/edge start",
		"ExecStop=/usr/bin/bare-systems --project-dir /tmp/edge stop",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
}

func TestInstallSystemdServiceIsIdempotent(t *testing.T) {
	unitPath := filepath.Join(t.TempDir(), "bare-systems-edge.service")
	runner := &FakeRunner{}
	options := ServiceOptions{BinaryPath: "/bin/bare-systems", ProjectDir: "/tmp/edge", UnitPath: unitPath, GOOS: "linux"}

	first, err := InstallSystemdService(context.Background(), runner, options)
	if err != nil {
		t.Fatalf("first install returned error: %v", err)
	}
	second, err := InstallSystemdService(context.Background(), runner, options)
	if err != nil {
		t.Fatalf("second install returned error: %v", err)
	}
	if !first.Changed {
		t.Fatalf("first install Changed = false, want true")
	}
	if second.Changed {
		t.Fatalf("second install Changed = true, want false")
	}
	if _, err := os.Stat(unitPath); err != nil {
		t.Fatalf("unit not written: %v", err)
	}
}

func TestUninstallSystemdServiceIsIdempotent(t *testing.T) {
	unitPath := filepath.Join(t.TempDir(), "bare-systems-edge.service")
	if err := os.WriteFile(unitPath, []byte("unit"), 0o644); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	runner := &FakeRunner{}
	options := ServiceOptions{UnitPath: unitPath, GOOS: "linux"}

	first, err := UninstallSystemdService(context.Background(), runner, options)
	if err != nil {
		t.Fatalf("first uninstall returned error: %v", err)
	}
	second, err := UninstallSystemdService(context.Background(), runner, options)
	if err != nil {
		t.Fatalf("second uninstall returned error: %v", err)
	}
	if !first.Changed {
		t.Fatalf("first uninstall Changed = false, want true")
	}
	if second.Changed {
		t.Fatalf("second uninstall Changed = true, want false")
	}
}
