package health

import (
	"context"
	"fmt"
	"sort"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
	edgeruntime "github.com/Bare-Systems/Bare-Systems-Installer/internal/runtime"
)

type Status string

const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusWarn    Status = "warn"
	StatusUnknown Status = "unknown"
)

type Level string

const (
	LevelHost      Level = "host"
	LevelConfig    Level = "config"
	LevelCompose   Level = "compose"
	LevelRuntime   Level = "runtime"
	LevelContainer Level = "container-health"
	LevelProduct   Level = "product-health"
	LevelPortal    Level = "portal"
)

type Check struct {
	Name        string         `json:"name"`
	Level       Level          `json:"level"`
	Status      Status         `json:"status"`
	Message     string         `json:"message"`
	Remediation string         `json:"remediation,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

type Summary struct {
	Status  Status         `json:"status"`
	Total   int            `json:"total"`
	Counts  map[Status]int `json:"counts"`
	Message string         `json:"message"`
}

type Report struct {
	Summary Summary `json:"summary"`
	Checks  []Check `json:"checks"`
}

type Options struct {
	Deployment       config.Deployment
	ValidationResult config.ValidationResult
	ComposeYAML      []byte
	Registry         modules.Registry
	Runner           edgeruntime.Runner
	ComposeFile      string
}

func Evaluate(ctx context.Context, options Options) Report {
	builder := reportBuilder{}
	builder.add(Check{Name: "config-valid", Level: LevelConfig, Status: StatusPass, Message: "edge.yml parsed and validated"})
	builder.add(Check{Name: "compose-render-valid", Level: LevelCompose, Status: StatusPass, Message: "Compose model rendered successfully"})

	prereqReport, prereqErr := edgeruntime.CheckDocker(ctx, options.Runner)
	for _, check := range prereqReport.Checks {
		status := StatusPass
		if !check.OK {
			status = StatusFail
		}
		builder.add(Check{
			Name:        check.Name,
			Level:       LevelHost,
			Status:      status,
			Message:     check.Message,
			Remediation: check.Remediation,
		})
	}

	var runtimeState edgeruntime.RuntimeState
	if prereqErr != nil {
		builder.add(Check{
			Name:        "runtime-containers",
			Level:       LevelRuntime,
			Status:      StatusUnknown,
			Message:     "Container state unavailable because Docker prerequisites failed",
			Remediation: "Fix Docker prerequisite checks and rerun doctor.",
		})
	} else {
		manager := edgeruntime.Compose{
			Runner:      options.Runner,
			ProjectDir:  options.Deployment.ComposeProjectDirectory(),
			ProjectName: options.Deployment.ComposeProjectName(),
			ComposeFile: options.ComposeFile,
			Profiles:    options.ValidationResult.Profiles,
		}
		result, err := manager.PS(ctx, true)
		if err != nil {
			builder.add(Check{
				Name:        "runtime-containers",
				Level:       LevelRuntime,
				Status:      StatusFail,
				Message:     err.Error(),
				Remediation: "Run `bare-systems start` after fixing Docker/Compose errors.",
			})
		} else {
			state, err := edgeruntime.ParsePSJSON(result.Stdout)
			if err != nil {
				builder.add(Check{
					Name:        "runtime-containers",
					Level:       LevelRuntime,
					Status:      StatusFail,
					Message:     err.Error(),
					Remediation: "Upgrade Docker Compose or inspect `docker compose ps --format json` output.",
				})
			} else {
				runtimeState = state
				status := StatusPass
				message := fmt.Sprintf("%d containers reported by Compose", state.Summary.Total)
				if state.Summary.Total == 0 {
					status = StatusWarn
					message = "No containers reported by Compose"
				}
				builder.add(Check{
					Name:    "runtime-containers",
					Level:   LevelRuntime,
					Status:  status,
					Message: message,
					Data: map[string]any{
						"summary": state.Summary,
					},
				})
			}
		}
	}

	addManifestHealthChecks(&builder, options, runtimeState)
	builder.add(Check{
		Name:        "portal-reporting",
		Level:       LevelPortal,
		Status:      StatusUnknown,
		Message:     "Portal reporting health is not configured in this implementation pass",
		Remediation: "Enroll the device and enable reporting when Portal integration is implemented.",
	})

	return builder.report()
}

func ConfigFailure(err error) Report {
	builder := reportBuilder{}
	builder.add(Check{
		Name:        "config-valid",
		Level:       LevelConfig,
		Status:      StatusFail,
		Message:     err.Error(),
		Remediation: "Fix edge.yml or .env and rerun doctor.",
	})
	return builder.report()
}

func addManifestHealthChecks(builder *reportBuilder, options Options, state edgeruntime.RuntimeState) {
	containersByService := map[string]edgeruntime.Container{}
	for _, container := range state.Containers {
		containersByService[container.Service] = container
	}

	for _, manifest := range options.Registry.All() {
		if !manifest.Module.Required && !options.Deployment.ModuleLocal(manifest.Module.ID) {
			continue
		}
		for _, service := range manifest.Module.Services {
			if service.Health.Type == "" {
				continue
			}
			name := "manifest-health:" + service.ComposeService
			container, ok := containersByService[service.ComposeService]
			if !ok {
				builder.add(Check{
					Name:        name,
					Level:       LevelProduct,
					Status:      StatusUnknown,
					Message:     "No Compose container state available for manifest health check",
					Remediation: "Start the deployment and rerun doctor.",
					Data:        healthCheckData(service),
				})
				continue
			}
			status := StatusPass
			message := "Container health is healthy"
			if container.Health == "" {
				status = StatusUnknown
				message = "Container did not report health status"
			} else if container.Health != "healthy" {
				status = StatusFail
				message = "Container health is " + container.Health
			}
			builder.add(Check{
				Name:    name,
				Level:   LevelProduct,
				Status:  status,
				Message: message,
				Data:    healthCheckData(service),
			})
		}
	}
}

func healthCheckData(service modules.Service) map[string]any {
	data := map[string]any{
		"service": service.ComposeService,
		"type":    service.Health.Type,
	}
	if service.Health.URL != "" {
		data["url"] = service.Health.URL
	}
	if len(service.Health.Command) > 0 {
		data["command"] = service.Health.Command
	}
	return data
}

type reportBuilder struct {
	checks []Check
}

func (b *reportBuilder) add(check Check) {
	b.checks = append(b.checks, check)
}

func (b *reportBuilder) report() Report {
	sort.SliceStable(b.checks, func(i, j int) bool {
		if b.checks[i].Level == b.checks[j].Level {
			return b.checks[i].Name < b.checks[j].Name
		}
		return b.checks[i].Level < b.checks[j].Level
	})

	counts := map[Status]int{}
	for _, check := range b.checks {
		counts[check.Status]++
	}
	summaryStatus := StatusPass
	message := "All required doctor checks passed"
	if counts[StatusFail] > 0 {
		summaryStatus = StatusFail
		message = "Doctor found failing checks"
	} else if counts[StatusWarn] > 0 {
		summaryStatus = StatusWarn
		message = "Doctor found warnings"
	} else if counts[StatusUnknown] > 0 {
		summaryStatus = StatusUnknown
		message = "Doctor found checks with unknown status"
	}

	return Report{
		Summary: Summary{
			Status:  summaryStatus,
			Total:   len(b.checks),
			Counts:  counts,
			Message: message,
		},
		Checks: b.checks,
	}
}

func (r Report) HasFailures() bool {
	return r.Summary.Counts[StatusFail] > 0
}
