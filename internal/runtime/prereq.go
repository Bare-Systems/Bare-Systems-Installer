package runtime

import (
	"context"
	"fmt"
	"strings"
)

type PrereqCheck struct {
	Name        string `json:"name"`
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

type PrereqReport struct {
	Checks []PrereqCheck `json:"checks"`
}

type PrereqError struct {
	Message string
	Report  PrereqReport
}

func (e PrereqError) Error() string {
	return e.Message
}

func CheckDocker(ctx context.Context, runner Runner) (PrereqReport, error) {
	report := PrereqReport{}

	cliCheck, err := runPrereq(ctx, runner, Command{Name: "docker", Args: []string{"--version"}})
	if err != nil {
		check := PrereqCheck{
			Name:        "docker-cli",
			OK:          false,
			Message:     "Docker CLI not found",
			Remediation: "Install Docker Engine and ensure `docker` is on PATH.",
		}
		if !IsCommandNotFound(err) {
			check.Message = "Docker CLI check failed"
			check.Remediation = "Verify Docker is installed and `docker --version` works."
		}
		report.Checks = append(report.Checks, check)
		return report, PrereqError{Message: check.Message + ": " + check.Remediation, Report: report}
	}
	report.Checks = append(report.Checks, PrereqCheck{Name: "docker-cli", OK: true, Message: firstLine(cliCheck.Stdout)})

	daemonCheck, err := runPrereq(ctx, runner, Command{Name: "docker", Args: []string{"info"}})
	if err != nil {
		check := PrereqCheck{
			Name:        "docker-daemon",
			OK:          false,
			Message:     "Docker daemon unavailable",
			Remediation: "Start Docker Engine and retry.",
		}
		report.Checks = append(report.Checks, check)
		return report, PrereqError{Message: check.Message + ": " + check.Remediation, Report: report}
	}
	report.Checks = append(report.Checks, PrereqCheck{Name: "docker-daemon", OK: true, Message: firstLine(daemonCheck.Stdout)})

	composeCheck, err := runPrereq(ctx, runner, Command{Name: "docker", Args: []string{"compose", "version"}})
	if err != nil {
		check := PrereqCheck{
			Name:        "docker-compose-plugin",
			OK:          false,
			Message:     "Docker Compose plugin unavailable",
			Remediation: "Install the modern `docker compose` plugin; legacy `docker-compose` is not supported.",
		}
		report.Checks = append(report.Checks, check)
		return report, PrereqError{Message: check.Message + ": " + check.Remediation, Report: report}
	}
	report.Checks = append(report.Checks, PrereqCheck{Name: "docker-compose-plugin", OK: true, Message: firstLine(composeCheck.Stdout)})

	return report, nil
}

func runPrereq(ctx context.Context, runner Runner, command Command) (Result, error) {
	result, err := runner.Run(ctx, command)
	if err != nil {
		if commandErr := CommandError(command, result, err); commandErr != nil {
			return result, fmt.Errorf("%w", commandErr)
		}
	}
	return result, err
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "ok"
	}
	line, _, _ := strings.Cut(value, "\n")
	return line
}
