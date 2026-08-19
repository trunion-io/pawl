package evidence_test

// PAWL-025 AC8 and AC9.
//
// This package reads the only bytes pawl does not write itself: JUnit XML and
// Cobertura XML emitted by the client's test run, and typecheck output. On a
// fork pull request that content is attacker-influenced, which makes these the
// parsers worth fuzzing and the others not.
//
// AC9 is what the targets assert: an error is a fine outcome, a panic is not. A
// panic in the gate denies the merge pipeline pawl was installed to protect, and
// C-3 applies with force — a parser that crashed produced no evidence, and no
// evidence must never be read as coverage.

import (
	"os"
	"path/filepath"
	"testing"

	"trunion.io/pawl/internal/evidence"
)

// write puts the fuzz input somewhere the Load functions can reach, since they
// take a path rather than a reader.
func write(t *testing.T, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func FuzzJUnit(f *testing.F) {
	f.Add([]byte(`<testsuite><testcase classname="pkg.Mod" name="TestOne"/></testsuite>`))
	f.Add([]byte(`<testsuites><testsuite><testcase classname="a" name="b"><failure/></testcase></testsuite></testsuites>`))
	f.Add([]byte(`<testsuite>`))                    // truncated
	f.Add([]byte("<testsuite><\x00/></testsuite>")) // NUL in a name
	f.Add([]byte(`<?xml version="1.0"?><testsuite/>`))
	// Entity expansion. Go's encoding/xml does not resolve external entities, so
	// this is not XXE — it is here because a decoder that expanded it would hang
	// a client's pipeline, and that should stay untrue.
	f.Add([]byte(`<!DOCTYPE t [<!ENTITY a "aaaaaaaaaa"><!ENTITY b "&a;&a;&a;&a;&a;">]><testsuite>&b;</testsuite>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		ev := evidence.New()
		_ = evidence.LoadJUnit(ev, write(t, "junit.xml", data))
	})
}

func FuzzCoverage(f *testing.F) {
	f.Add([]byte(`<coverage><packages><package><classes><class filename="a.go"><lines><line number="1" hits="1"/></lines></class></classes></package></packages></coverage>`))
	f.Add([]byte(`<coverage><class filename="x"><lines><line number="-1" hits="0"/></lines></class></coverage>`))
	f.Add([]byte(`<coverage><class filename="x"><lines><line number="99999999999999999999" hits="1"/></lines></class></coverage>`))
	f.Add([]byte(`<coverage>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		ev := evidence.New()
		_ = evidence.LoadCoverageXML(ev, write(t, "coverage.xml", data), "")
	})
}

func FuzzTypecheck(f *testing.F) {
	f.Add([]byte(`[{"file":"a.go","line":1,"message":"oops"}]`))
	f.Add([]byte(`{"file":"a.go","line":1}` + "\n" + `{"file":"b.go","line":2}`)) // JSON lines
	f.Add([]byte(`[{"file":"a.go","line":"not-a-number"}]`))
	f.Add([]byte(`[`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		ev := evidence.New()
		changed := map[string]bool{"a.go": true, "b.go": true}
		_ = evidence.LoadTypecheck(ev, write(t, "typecheck.json", data), changed)
	})
}
