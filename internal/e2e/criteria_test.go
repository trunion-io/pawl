package e2e

// Checks for criteria that were delivered as `checkable: partially` (PAWL-022).
//
// Each is written from the criterion's wording rather than from the
// implementation. A test written from the code passes by construction and
// proves nothing.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"trunion.io/pawl/internal/claimlog"
	"trunion.io/pawl/internal/evidence"
	"trunion.io/pawl/internal/model"
	"trunion.io/pawl/internal/policy"
)

// TestClaimLogIsExcludedFromTheChangeset — PAWL-004 AC4.
//
// "The system shall exclude its own claim log from the changeset it analyses."
//
// Found in a live demo rather than by the suite: the trail was auditing itself,
// inflating the denominator and putting the records on their own reading list.
func TestClaimLogIsExcludedFromTheChangeset(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "exp is unix seconds",
		Path: "src/auth.py", StartLine: 4, EndLine: 6,
	})
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 1, EndLine: 9})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		t.Fatal(err)
	}
	rl := buildWithAcks(t, repo, "HEAD~1", loadClaims(t, repo), acks, collect(t, evidence.Sources{}))

	for _, s := range rl.Spans {
		if strings.HasPrefix(s.Path, ".pawl/") {
			t.Errorf("the record log is in its own changeset: %s:%d-%d",
				s.Path, s.StartLine, s.EndLine)
		}
	}
	// The records are committed and are real changed lines, so if they were not
	// excluded they would dominate a small changeset.
	if rl.ChangedLines() > 20 {
		t.Errorf("changed lines = %d; the record log looks to be counted", rl.ChangedLines())
	}
}

// TestNonSemanticLinesAreExcludedButCommentsAreNot — PAWL-004 AC5.
//
// "…exclude blank lines and bare delimiters from the reading list, and shall
// treat comments as reviewable."
//
// The second half is the deliberate part: a wrong comment is a defect, and
// agents write plenty of them.
func TestNonSemanticLinesAreExcludedButCommentsAreNot(t *testing.T) {
	repo := newRepo(t)
	mustWrite(t, filepath.Join(repo, "src", "shapes.go"),
		"package shapes\n\nfunc a() int {\n\treturn 1\n}\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "base")

	// A change containing a blank line, a bare delimiter, and a comment.
	mustWrite(t, filepath.Join(repo, "src", "shapes.go"),
		"package shapes\n\nfunc a() int {\n\treturn 1\n}\n\n"+
			"// this comment is reviewable\n"+
			"func b() int {\n"+
			"\treturn 2\n"+
			"}\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	rl := build(t, repo, "HEAD~1", nil, collect(t, evidence.Sources{}))

	source, err := os.ReadFile(filepath.Join(repo, "src", "shapes.go"))
	if err != nil {
		t.Fatal(err)
	}
	lines := model.SplitLines(string(source))

	sawComment := false
	for _, s := range rl.Spans {
		for n := s.StartLine; n <= s.EndLine; n++ {
			text := strings.TrimSpace(lines[n-1])
			if text == "" {
				t.Errorf("a blank line reached the reading list at %d", n)
			}
			if text == "}" {
				t.Errorf("a bare delimiter reached the reading list at %d: %q", n, text)
			}
			if strings.HasPrefix(text, "//") {
				sawComment = true
			}
		}
	}
	if !sawComment {
		t.Error("a comment was excluded; comments are reviewable on purpose")
	}
}

// TestPolicyIsReadFromTheRepository — PAWL-006 AC1.
//
// "…read thresholds from .pawl/policy.toml in the target repository and shall
// fall back to defaults where absent."
func TestPolicyIsReadFromTheRepository(t *testing.T) {
	repo := newRepo(t)

	// Absent: defaults.
	got, err := policy.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	def := policy.Defaults()
	if got.MaxChangedLines != def.MaxChangedLines ||
		got.MaxMustReadRatio != def.MaxMustReadRatio ||
		got.MaxUnclaimedLines != def.MaxUnclaimedLines ||
		got.BlockOnUndetermined != def.BlockOnUndetermined {
		t.Errorf("with no policy file the defaults must apply, got %+v", got)
	}

	// Present: the file wins.
	mustMkdir(t, filepath.Join(repo, ".pawl"))
	mustWrite(t, filepath.Join(repo, ".pawl", "policy.toml"), `[gate]
max_changed_lines = 7
max_must_read_ratio = 0.9
max_unclaimed_lines = 3
block_on_undetermined = false
sensitive_paths = ["src/auth/"]
`)
	got, err = policy.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxChangedLines != 7 {
		t.Errorf("max_changed_lines = %d, want 7", got.MaxChangedLines)
	}
	if got.MaxMustReadRatio != 0.9 {
		t.Errorf("max_must_read_ratio = %v, want 0.9", got.MaxMustReadRatio)
	}
	if got.MaxUnclaimedLines != 3 {
		t.Errorf("max_unclaimed_lines = %d, want 3", got.MaxUnclaimedLines)
	}
	if got.BlockOnUndetermined {
		t.Error("block_on_undetermined = true, want the file's false")
	}
	if len(got.SensitivePaths) != 1 || got.SensitivePaths[0] != "src/auth/" {
		t.Errorf("sensitive_paths = %v", got.SensitivePaths)
	}
}

// TestGateBlocksOnTheMustReadRatio — PAWL-006 AC3.
//
// "…fail a changeset where must-read lines exceed the configured ratio."
//
// Isolated from the other rules: everything is claimed, so unclaimed_lines
// cannot be what fails it, and the budget is generous.
func TestGateBlocksOnTheMustReadRatio(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	// Claimed, but the asserted test failed — so the span needs a human.
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "cites a failing test",
		Path: "src/auth.py", StartLine: 1, EndLine: 9,
		VerifiedBy: testRef("tests.test_auth.test_clock_skew"),
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), ev)

	if rl.Summary().UnclaimedLines != 0 {
		t.Fatalf("fixture is wrong: %d unclaimed lines would fail the gate for "+
			"the wrong reason", rl.Summary().UnclaimedLines)
	}

	p := policy.Defaults()
	p.MaxChangedLines = 1000
	p.MaxMustReadRatio = 0.10

	d := policy.Evaluate(rl, p)
	if d.Allowed {
		t.Fatal("everything needs a human and the limit is 10%; the gate must block")
	}
	if !hasRule(d, "must_read_ratio") {
		t.Errorf("blocked for the wrong reason: %+v", d.Violations)
	}

	// And it passes once the ratio allows it.
	p.MaxMustReadRatio = 1.0
	if d := policy.Evaluate(rl, p); hasRule(d, "must_read_ratio") {
		t.Error("the ratio rule fired when the ratio was within the limit")
	}
}

// TestSensitivePathRefusesImplicitCoverage — PAWL-006 AC5.
//
// "…require a named check on claims touching them and shall not accept implicit
// coverage."
//
// The second half is what makes the rule worth having: a span can be exercised
// by the suite without anyone having said what it is supposed to do.
func TestSensitivePathRefusesImplicitCoverage(t *testing.T) {
	repo := newRepo(t)
	mustMkdir(t, filepath.Join(repo, "src", "auth"))
	mustWrite(t, filepath.Join(repo, "src", "auth", "token.py"), "def check():\n    return True\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "base")
	mustWrite(t, filepath.Join(repo, "src", "auth", "token.py"),
		"def check():\n    return True\n\ndef rotate():\n    return None\n")

	// A claim with no named check — implicit coverage at best.
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "rotation is a no-op for now",
		Path: "src/auth/token.py", StartLine: 4, EndLine: 5,
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), collect(t, evidence.Sources{}))

	p := policy.Defaults()
	p.SensitivePaths = []string{"src/auth/"}
	d := policy.Evaluate(rl, p)
	if !hasRule(d, "sensitive_path_needs_named_check") {
		t.Errorf("a claim on a sensitive path with no named check must fail: %+v", d.Violations)
	}

	// Off a sensitive path, the same claim is not required to name a check.
	p.SensitivePaths = []string{"src/billing/"}
	if d := policy.Evaluate(rl, p); hasRule(d, "sensitive_path_needs_named_check") {
		t.Error("the rule fired on a path that is not configured as sensitive")
	}
}

// TestSkippedTestIsAbsentNotPassing — PAWL-003 AC3.
//
// "…treat a skipped test as absent, not as passing."
//
// Strict on purpose. A claim citing a skipped test has cited a check that did
// not run, and a check that did not run is not evidence.
func TestSkippedTestIsAbsentNotPassing(t *testing.T) {
	repo := newRepo(t)
	junit := filepath.Join(repo, "skipped.xml")
	mustWrite(t, junit, `<?xml version="1.0"?>
<testsuites><testsuite name="pytest">
<testcase classname="tests.test_auth" name="test_skipped"><skipped/></testcase>
<testcase classname="tests.test_auth" name="test_ran"/>
</testsuite></testsuites>
`)
	ev := collect(t, evidence.Sources{JUnit: []string{junit}})

	if _, present := ev.TestPassed("tests.test_auth.test_skipped"); present {
		t.Error("a skipped test must be absent from the evidence, not present")
	}
	// The distinction only means something if a test that did run is present.
	passed, present := ev.TestPassed("tests.test_auth.test_ran")
	if !present || !passed {
		t.Error("a test that ran and passed should be present and passing")
	}
}

// TestOnlyCheckableSpecCriteriaAreEvidence — PAWL-003 AC5.
//
// "…accept a criterion as evidence only if that criterion is marked checkable."
//
// PAWL-003 records this as untested pending PAWL-009, but the spec tool is
// needed to *produce* signed specs, not to test the code that *reads* one.
func TestOnlyCheckableSpecCriteriaAreEvidence(t *testing.T) {
	repo := newRepo(t)
	spec := filepath.Join(repo, "spec.json")
	mustWrite(t, spec, `{"predicate":{"criteria":[
	  {"id":"PAWL-003-AC1","checkable":true},
	  {"id":"PAWL-003-AC9","checkable":false}
	]}}`)

	ev := collect(t, evidence.Sources{Spec: spec})

	if !ev.SpecCriteria["PAWL-003-AC1"] {
		t.Error("a criterion marked checkable should count as evidence")
	}
	if ev.SpecCriteria["PAWL-003-AC9"] {
		t.Error("a criterion marked not checkable must never count as evidence; " +
			"an unverifiable criterion is a permanent tax on human attention " +
			"and must not clear a span mechanically")
	}
}

// TestMigrateMovesRecordsAndOnlyThenRemovesTheLog — PAWL-018 AC8.
//
// Losing evidence is the one thing this component may never do, so the log is
// removed only once every record it held is confirmed present.
func TestMigrateMovesRecordsAndOnlyThenRemovesTheLog(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	// One record in the new layout, two left in a legacy log.
	kept := record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "already migrated",
		Path: "src/auth.py", StartLine: 4, EndLine: 6,
	})
	legacy := `{"schema_version":"0.1","id":"legacyA","ts":"2026-01-01T00:00:00Z","kind":"assumption",` +
		`"text":"first legacy","path":"src/auth.py","start_line":1,"end_line":2,` +
		`"fingerprint":"sha256:a","verified_by":[],"author":{"role":"agent"}}` + "\n" +
		`{"schema_version":"0.1","id":"legacyB","ts":"2026-01-02T00:00:00Z","kind":"assumption",` +
		`"text":"second legacy","path":"src/auth.py","start_line":8,"end_line":9,` +
		`"fingerprint":"sha256:b","verified_by":[],"author":{"role":"agent"}}` + "\n"
	mustWrite(t, claimlog.LogPath(repo), legacy)

	claims, _, err := claimlog.Migrate(repo)
	if err != nil {
		t.Fatal(err)
	}
	if claims != 2 {
		t.Errorf("migrated %d claims, want 2", claims)
	}
	if _, err := os.Stat(claimlog.LogPath(repo)); !os.IsNotExist(err) {
		t.Error("the legacy log should be gone once every record moved")
	}

	// Every record survives, and the content is unchanged — migration moves the
	// container, never the record.
	all := loadClaims(t, repo)
	byID := map[string]string{}
	for _, c := range all {
		byID[c.ID] = c.Text
	}
	for id, want := range map[string]string{
		kept.ID: "already migrated", "legacyA": "first legacy", "legacyB": "second legacy",
	} {
		if byID[id] != want {
			t.Errorf("record %s = %q, want %q", id, byID[id], want)
		}
	}

	// Safe to re-run.
	if _, _, err := claimlog.Migrate(repo); err != nil {
		t.Errorf("migration must be safe to re-run: %v", err)
	}
}
