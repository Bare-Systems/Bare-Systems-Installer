package runtime

import (
	"context"
	"strings"
	"testing"
)

func TestRenderTardigradeConfig(t *testing.T) {
	config := RenderTardigradeConfig(TardigradeConfigOptions{
		ListenPort:  8088,
		PidFile:     "/tmp/tardigrade.pid",
		PublicDir:   "/srv/bare/public",
		UpstreamURL: "http://127.0.0.1:8080/",
	})

	for _, want := range []string{
		"pid /tmp/tardigrade.pid;",
		"listen 8088;",
		"server_name localhost 127.0.0.1;",
		"root /srv/bare/public;",
		"proxy_pass http://127.0.0.1:8080/up;",
		"proxy_pass http://127.0.0.1:8080;",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("rendered config missing %q:\n%s", want, config)
		}
	}
}

func TestRenderTardigradeConfigUsesServerNames(t *testing.T) {
	config := RenderTardigradeConfig(TardigradeConfigOptions{
		ServerNames: []string{"localhost", "127.0.0.1", "192.168.86.53"},
	})

	if !strings.Contains(config, "server_name localhost 127.0.0.1 192.168.86.53;") {
		t.Fatalf("rendered config missing server names:\n%s", config)
	}
}

func TestTardigradeServerNamesParsesAndSanitizes(t *testing.T) {
	names := TardigradeServerNames("localhost, 127.0.0.1 192.168.86.53 bad;name localhost")
	got := strings.Join(names, " ")
	want := "localhost 127.0.0.1 192.168.86.53"
	if got != want {
		t.Fatalf("TardigradeServerNames() = %q, want %q", got, want)
	}
}

func TestTardigradeStartOrReloadStartsWhenStopped(t *testing.T) {
	runner := &FakeRunner{
		Results: map[string]FakeResult{
			"tardigrade status -c /tmp/tardigrade.conf": {Stdout: "status: stopped\n"},
		},
	}
	proxy := Tardigrade{Runner: runner, ConfigFile: "/tmp/tardigrade.conf"}

	if _, err := proxy.StartOrReload(context.Background()); err != nil {
		t.Fatalf("StartOrReload returned error: %v", err)
	}

	assertRuntimeCommand(t, runner.Commands, "tardigrade status -c /tmp/tardigrade.conf")
	assertRuntimeCommand(t, runner.Commands, "tardigrade run -c /tmp/tardigrade.conf --daemon")
}

func TestTardigradeStartOrReloadReloadsWhenRunning(t *testing.T) {
	runner := &FakeRunner{
		Results: map[string]FakeResult{
			"tardigrade status -c /tmp/tardigrade.conf": {Stdout: "status: running\n"},
		},
	}
	proxy := Tardigrade{Runner: runner, ConfigFile: "/tmp/tardigrade.conf"}

	if _, err := proxy.StartOrReload(context.Background()); err != nil {
		t.Fatalf("StartOrReload returned error: %v", err)
	}

	assertRuntimeCommand(t, runner.Commands, "tardigrade reload -c /tmp/tardigrade.conf")
	for _, command := range runner.Commands {
		if command.String() == "tardigrade run -c /tmp/tardigrade.conf --daemon" {
			t.Fatalf("running proxy should reload, not start: %#v", runner.Commands)
		}
	}
}

func TestTardigradeStopIfRunningSkipsWhenStopped(t *testing.T) {
	runner := &FakeRunner{
		Results: map[string]FakeResult{
			"tardigrade status -c /tmp/tardigrade.conf": {Stdout: "status: stopped\n"},
		},
	}
	proxy := Tardigrade{Runner: runner, ConfigFile: "/tmp/tardigrade.conf"}

	if _, err := proxy.StopIfRunning(context.Background()); err != nil {
		t.Fatalf("StopIfRunning returned error: %v", err)
	}

	if len(runner.Commands) != 1 {
		t.Fatalf("expected only status command, got %#v", runner.Commands)
	}
	assertRuntimeCommand(t, runner.Commands, "tardigrade status -c /tmp/tardigrade.conf")
}

func TestTardigradeStopIfRunningStopsWhenRunning(t *testing.T) {
	runner := &FakeRunner{
		Results: map[string]FakeResult{
			"tardigrade status -c /tmp/tardigrade.conf": {Stdout: "status: running\n"},
		},
	}
	proxy := Tardigrade{Runner: runner, ConfigFile: "/tmp/tardigrade.conf"}

	if _, err := proxy.StopIfRunning(context.Background()); err != nil {
		t.Fatalf("StopIfRunning returned error: %v", err)
	}

	assertRuntimeCommand(t, runner.Commands, "tardigrade stop -c /tmp/tardigrade.conf")
}

func TestTardigradeUpstreamURLDefaultsToLoopback(t *testing.T) {
	for _, tc := range []struct {
		name string
		host string
		port string
		want string
	}{
		{name: "empty", want: "http://127.0.0.1:8080"},
		{name: "wildcard", host: "0.0.0.0", port: "8081", want: "http://127.0.0.1:8081"},
		{name: "explicit", host: "192.168.1.10", port: "9000", want: "http://192.168.1.10:9000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := TardigradeUpstreamURL(tc.host, tc.port); got != tc.want {
				t.Fatalf("TardigradeUpstreamURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func assertRuntimeCommand(t *testing.T, commands []Command, want string) {
	t.Helper()
	for _, command := range commands {
		if command.String() == want {
			return
		}
	}
	t.Fatalf("missing command %q in %#v", want, commands)
}
