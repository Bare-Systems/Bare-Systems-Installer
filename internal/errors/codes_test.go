package errors

import "testing"

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		code Code
		want int
	}{
		{CodeOK, ExitOK},
		{CodeUsage, ExitUsage},
		{CodeConfig, ExitConfig},
		{CodePrereq, ExitPrereq},
		{CodeAuth, ExitAuth},
		{CodeNetwork, ExitNetwork},
		{CodeRuntime, ExitRuntime},
		{CodeHealth, ExitHealth},
		{CodeUpdate, ExitUpdate},
		{CodeRollback, ExitRollback},
		{CodeDiagnostics, ExitDiagnostics},
		{CodePermissions, ExitPermissions},
		{Code("ERR_UNKNOWN"), ExitGeneric},
	}

	for _, tt := range tests {
		if got := ExitCodeFor(tt.code); got != tt.want {
			t.Fatalf("ExitCodeFor(%q) = %d, want %d", tt.code, got, tt.want)
		}
	}
}
