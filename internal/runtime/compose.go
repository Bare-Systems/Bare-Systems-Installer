package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Compose struct {
	Runner      Runner
	ProjectDir  string
	ProjectName string
	ComposeFile string
	Profiles    []string
}

type Container struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Service string `json:"service,omitempty"`
	State   string `json:"state,omitempty"`
	Health  string `json:"health,omitempty"`
}

type RuntimeState struct {
	Containers []Container  `json:"containers"`
	Summary    StateSummary `json:"summary"`
	Raw        string       `json:"raw,omitempty"`
}

type StateSummary struct {
	Total    int            `json:"total"`
	ByState  map[string]int `json:"byState"`
	ByHealth map[string]int `json:"byHealth"`
}

func (c Compose) Config(ctx context.Context) (Result, error) {
	return c.run(ctx, "config", "-q")
}

func (c Compose) Pull(ctx context.Context) (Result, error) {
	return c.run(ctx, "pull")
}

func (c Compose) Up(ctx context.Context) (Result, error) {
	return c.run(ctx, "up", "-d")
}

func (c Compose) Stop(ctx context.Context) (Result, error) {
	return c.run(ctx, "stop")
}

func (c Compose) Restart(ctx context.Context) (Result, error) {
	return c.run(ctx, "restart")
}

func (c Compose) PS(ctx context.Context, jsonOutput bool) (Result, error) {
	if jsonOutput {
		return c.run(ctx, "ps", "--format", "json")
	}
	return c.run(ctx, "ps")
}

func (c Compose) Logs(ctx context.Context, service string) (Result, error) {
	args := []string{"logs", "--tail", "200"}
	if service != "" {
		args = append(args, service)
	}
	return c.run(ctx, args...)
}

func (c Compose) run(ctx context.Context, subcommand ...string) (Result, error) {
	runner := c.Runner
	if runner == nil {
		runner = ExecRunner{}
	}

	command := Command{
		Name: "docker",
		Args: append(c.baseArgs(), subcommand...),
		Dir:  c.ProjectDir,
	}
	result, err := runner.Run(ctx, command)
	return result, CommandError(command, result, err)
}

func (c Compose) baseArgs() []string {
	projectName := c.ProjectName
	if projectName == "" {
		projectName = ComposeProjectName
	}

	args := []string{
		"compose",
		"--project-directory", c.ProjectDir,
		"--project-name", projectName,
	}
	if c.ComposeFile != "" {
		args = append(args, "-f", c.ComposeFile)
	}
	for _, profile := range sortedProfiles(c.Profiles) {
		args = append(args, "--profile", profile)
	}
	return args
}

func ParsePSJSON(raw string) (RuntimeState, error) {
	state := RuntimeState{
		Raw: raw,
		Summary: StateSummary{
			ByState:  map[string]int{},
			ByHealth: map[string]int{},
		},
	}
	if strings.TrimSpace(raw) == "" {
		return state, nil
	}

	var decoded []map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return RuntimeState{}, fmt.Errorf("parse docker compose ps JSON: %w", err)
	}

	for _, item := range decoded {
		container := Container{
			ID:      stringField(item, "ID", "Id", "ContainerID"),
			Name:    stringField(item, "Name"),
			Service: stringField(item, "Service"),
			State:   stringField(item, "State"),
			Health:  stringField(item, "Health"),
		}
		if container.Health == "" {
			container.Health = stringField(item, "HealthStatus")
		}
		state.Containers = append(state.Containers, container)
		state.Summary.Total++
		if container.State != "" {
			state.Summary.ByState[container.State]++
		}
		if container.Health != "" {
			state.Summary.ByHealth[container.Health]++
		}
	}
	return state, nil
}

func stringField(item map[string]any, names ...string) string {
	for _, name := range names {
		value, ok := item[name]
		if !ok || value == nil {
			continue
		}
		if text, ok := value.(string); ok {
			return text
		}
		return fmt.Sprint(value)
	}
	return ""
}

func sortedProfiles(profiles []string) []string {
	copied := append([]string(nil), profiles...)
	sort.Strings(copied)
	return copied
}
