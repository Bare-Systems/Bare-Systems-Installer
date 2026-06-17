package runtime

import (
	"context"
	"strings"
	"testing"
)

func TestComposeUsesProjectDirAndSortedProfiles(t *testing.T) {
	runner := &FakeRunner{}
	compose := Compose{
		Runner:      runner,
		ProjectDir:  "/tmp/edge/compose",
		ProjectName: "bare-systems",
		ComposeFile: "/tmp/edge/compose/generated.compose.yml",
		Profiles:    []string{"koala", "core"},
	}

	if _, err := compose.Up(context.Background()); err != nil {
		t.Fatalf("Up returned error: %v", err)
	}
	if len(runner.Commands) != 1 {
		t.Fatalf("commands = %#v", runner.Commands)
	}
	command := runner.Commands[0]
	if command.Dir != "/tmp/edge/compose" {
		t.Fatalf("Dir = %q", command.Dir)
	}
	got := strings.Join(command.Args, " ")
	for _, want := range []string{
		"compose --project-directory /tmp/edge/compose",
		"--project-name bare-systems",
		"-f /tmp/edge/compose/generated.compose.yml",
		"--profile core --profile koala",
		"up -d",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("args missing %q: %s", want, got)
		}
	}
}

func TestLogsAllowsOptionalService(t *testing.T) {
	runner := &FakeRunner{}
	compose := Compose{Runner: runner, ProjectDir: "/tmp/edge/compose", ProjectName: "bare-systems"}

	if _, err := compose.Logs(context.Background(), "bear-claw-web"); err != nil {
		t.Fatalf("Logs returned error: %v", err)
	}
	got := strings.Join(runner.Commands[0].Args, " ")
	if !strings.Contains(got, "logs --tail 200 bear-claw-web") {
		t.Fatalf("args = %s", got)
	}
}

func TestParsePSJSON(t *testing.T) {
	state, err := ParsePSJSON(`[{"ID":"abc","Name":"edge","Service":"bear-claw-web","State":"running","Health":"healthy"}]`)
	if err != nil {
		t.Fatalf("ParsePSJSON returned error: %v", err)
	}
	if state.Summary.Total != 1 {
		t.Fatalf("Total = %d, want 1", state.Summary.Total)
	}
	if state.Summary.ByState["running"] != 1 {
		t.Fatalf("ByState = %#v", state.Summary.ByState)
	}
	if state.Containers[0].Service != "bear-claw-web" {
		t.Fatalf("Service = %q", state.Containers[0].Service)
	}
}
