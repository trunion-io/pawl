package claimlog_test

// PAWL-025 AC8 and AC9 for the record readers of PAWL-018.
//
// Record files are written by pawl, but they are committed to the tree, which
// means they arrive through a pull request and are therefore not trusted input
// by the time they are read back. A malformed record must be reported, not
// panicked on.

import (
	"os"
	"path/filepath"
	"testing"

	"trunion.io/pawl/internal/claimlog"
)

func FuzzRecord(f *testing.F) {
	f.Add([]byte(`{"schema_version":"0.1","id":"abc","ts":"2026-08-19T00:00:00Z","kind":"assumption","text":"t","path":"a.go","start_line":1,"end_line":2}`))
	f.Add([]byte(`{"id":"","start_line":-1,"end_line":-9223372036854775808}`))
	f.Add([]byte(`{"start_line":99999999999999999999}`))
	f.Add([]byte(`{`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		repo := t.TempDir()

		// Per-record layout.
		dir := filepath.Join(repo, claimlog.ClaimDir, claimlog.ClaimsSubdir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "fuzz.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		_, _ = claimlog.Load(repo)

		// Legacy shared log, still read under AC6 and equally untrusted.
		legacy := t.TempDir()
		if err := os.MkdirAll(filepath.Join(legacy, claimlog.ClaimDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(claimlog.LogPath(legacy), data, 0o644); err != nil {
			t.Fatal(err)
		}
		_, _ = claimlog.Load(legacy)
	})
}
