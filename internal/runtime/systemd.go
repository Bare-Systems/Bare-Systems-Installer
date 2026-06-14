package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
)

const (
	DefaultUnitName = "bare-systems-edge.service"
	DefaultUnitPath = "/etc/systemd/system/bare-systems-edge.service"
)

type ServiceOptions struct {
	BinaryPath string
	ProjectDir string
	UnitPath   string
	GOOS       string
}

type ServiceResult struct {
	UnitPath string `json:"unitPath"`
	Changed  bool   `json:"changed"`
	Message  string `json:"message"`
}

func RenderSystemdUnit(options ServiceOptions) string {
	binaryPath := options.BinaryPath
	if binaryPath == "" {
		binaryPath = "/usr/bin/bare-systems"
	}
	projectDir := options.ProjectDir
	if projectDir == "" {
		projectDir = "/opt/bare-systems"
	}

	return fmt.Sprintf(`[Unit]
Description=Bare Systems Edge Deployment
After=docker.service network-online.target
Requires=docker.service
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=%s --project-dir %s start
ExecStop=%s --project-dir %s stop
ExecReload=%s --project-dir %s restart

[Install]
WantedBy=multi-user.target
`, binaryPath, projectDir, binaryPath, projectDir, binaryPath, projectDir)
}

func InstallSystemdService(ctx context.Context, runner Runner, options ServiceOptions) (ServiceResult, error) {
	if err := requireLinux(options.GOOS); err != nil {
		return ServiceResult{}, err
	}
	if runner == nil {
		runner = ExecRunner{}
	}
	unitPath := serviceUnitPath(options)
	unit := []byte(RenderSystemdUnit(options))

	changed := true
	if existing, err := os.ReadFile(unitPath); err == nil && string(existing) == string(unit) {
		changed = false
	}
	if changed {
		if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
			return ServiceResult{}, fmt.Errorf("create systemd unit directory: %w", err)
		}
		if err := os.WriteFile(unitPath, unit, 0o644); err != nil {
			return ServiceResult{}, fmt.Errorf("write systemd unit %s: %w", unitPath, err)
		}
	}

	if _, err := runner.Run(ctx, Command{Name: "systemctl", Args: []string{"daemon-reload"}}); err != nil {
		return ServiceResult{}, fmt.Errorf("reload systemd units: %w", err)
	}
	if _, err := runner.Run(ctx, Command{Name: "systemctl", Args: []string{"enable", "--now", DefaultUnitName}}); err != nil {
		return ServiceResult{}, fmt.Errorf("enable systemd unit: %w", err)
	}

	message := "systemd service installed"
	if !changed {
		message = "systemd service already up to date"
	}
	return ServiceResult{UnitPath: unitPath, Changed: changed, Message: message}, nil
}

func UninstallSystemdService(ctx context.Context, runner Runner, options ServiceOptions) (ServiceResult, error) {
	if err := requireLinux(options.GOOS); err != nil {
		return ServiceResult{}, err
	}
	if runner == nil {
		runner = ExecRunner{}
	}
	unitPath := serviceUnitPath(options)

	_, _ = runner.Run(ctx, Command{Name: "systemctl", Args: []string{"disable", "--now", DefaultUnitName}})

	changed := false
	if err := os.Remove(unitPath); err == nil {
		changed = true
	} else if err != nil && !os.IsNotExist(err) {
		return ServiceResult{}, fmt.Errorf("remove systemd unit %s: %w", unitPath, err)
	}

	if _, err := runner.Run(ctx, Command{Name: "systemctl", Args: []string{"daemon-reload"}}); err != nil {
		return ServiceResult{}, fmt.Errorf("reload systemd units: %w", err)
	}

	message := "systemd service uninstalled"
	if !changed {
		message = "systemd service already absent"
	}
	return ServiceResult{UnitPath: unitPath, Changed: changed, Message: message}, nil
}

func serviceUnitPath(options ServiceOptions) string {
	if options.UnitPath != "" {
		return options.UnitPath
	}
	return DefaultUnitPath
}

func requireLinux(goos string) error {
	if goos == "" {
		goos = stdruntime.GOOS
	}
	if !strings.EqualFold(goos, "linux") {
		return fmt.Errorf("systemd service management requires Linux/systemd")
	}
	return nil
}
