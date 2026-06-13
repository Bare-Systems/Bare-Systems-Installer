package version

import "testing"

func TestCurrent(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, Commit, Date
	t.Cleanup(func() {
		Version, Commit, Date = oldVersion, oldCommit, oldDate
	})

	Version = "1.2.3"
	Commit = "abc123"
	Date = "2026-06-13T00:00:00Z"

	info := Current()
	if info.Version != Version {
		t.Fatalf("Version = %q, want %q", info.Version, Version)
	}
	if info.Commit != Commit {
		t.Fatalf("Commit = %q, want %q", info.Commit, Commit)
	}
	if info.Date != Date {
		t.Fatalf("Date = %q, want %q", info.Date, Date)
	}
}
