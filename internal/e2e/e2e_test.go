// Package e2e runs end-to-end over a real git repo. No mocks: the failure modes
// worth catching here are all in the git and anchoring layers (C-9).
//
// These are the same ten cases as the Python suite, in the same order, asserting
// the same things. Where a test reads differently it is because Go's testing
// package works differently, not because the behaviour under test changed.
package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"trunion.io/pawl/internal/attest"
	"trunion.io/pawl/internal/claimlog"
	"trunion.io/pawl/internal/evidence"
	"trunion.io/pawl/internal/model"
	"trunion.io/pawl/internal/policy"
	"trunion.io/pawl/internal/resolve"
)

func git(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// newRepo is the equivalent of the pytest `repo` fixture. t.TempDir is cleaned
// up automatically, same as tmp_path.
func newRepo(t *testing.T) string {
	t.Helper()
	r := filepath.Join(t.TempDir(), "repo")
	mustMkdir(t, r)
	git(t, r, "init", "-q", "-b", "main")
	git(t, r, "config", "user.email", "pod@example.com")
	git(t, r, "config", "user.name", "pod")
	mustMkdir(t, filepath.Join(r, "src"))
	mustWrite(t, filepath.Join(r, "src", "auth.py"), "def noop():\n    return None\n")
	git(t, r, "add", "-A")
	git(t, r, "commit", "-qm", "base")
	return r
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const junitXML = `<?xml version="1.0"?>
<testsuites><testsuite name="pytest" tests="2">
<testcase classname="tests.test_auth" name="test_expiry"/>
<testcase classname="tests.test_auth" name="test_clock_skew"><failure>boom</failure></testcase>
</testsuite></testsuites>
`

const coverageXML = `<?xml version="1.0"?>
<coverage><packages><package><classes>
<class filename="src/auth.py"><lines>
<line number="3" hits="1"/><line number="4" hits="1"/><line number="5" hits="1"/>
<line number="6" hits="1"/><line number="7" hits="0"/><line number="8" hits="0"/>
</lines></class>
</classes></package></packages></coverage>
`

func writeFeature(t *testing.T, repo string) {
	t.Helper()
	mustWrite(t, filepath.Join(repo, "src", "auth.py"),
		"def noop():\n"+
			"    return None\n"+
			"\n"+
			"def verify_token(token, now):\n"+
			"    \"\"\"Reject expired tokens.\"\"\"\n"+
			"    return token.exp > now\n"+
			"\n"+
			"def refresh(token):\n"+
			"    return token\n")
}

func record(t *testing.T, repo string, opts claimlog.Options) model.Claim {
	t.Helper()
	c, err := claimlog.Record(repo, opts)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	return c
}

func testRef(ref string) []model.EvidenceRef {
	return []model.EvidenceRef{{Type: model.EvidenceTest, Ref: ref}}
}

func loadClaims(t *testing.T, repo string) []model.Claim {
	t.Helper()
	claims, err := claimlog.Load(repo)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return claims
}

func collect(t *testing.T, s evidence.Sources) *evidence.Evidence {
	t.Helper()
	ev, err := evidence.Collect(s)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	return ev
}

func build(t *testing.T, repo, base string, claims []model.Claim, ev *evidence.Evidence) model.ReadingList {
	t.Helper()
	rl, err := resolve.BuildReadingList(repo, base, claims, ev, "HEAD")
	if err != nil {
		t.Fatalf("build reading list: %v", err)
	}
	return rl
}

func writeJUnit(t *testing.T, repo string) string {
	t.Helper()
	p := filepath.Join(repo, "junit.xml")
	mustWrite(t, p, junitXML)
	return p
}

func writeCoverage(t *testing.T, repo string) string {
	t.Helper()
	p := filepath.Join(repo, "coverage.xml")
	mustWrite(t, p, coverageXML)
	return p
}

func TestVerifiedClaimCollapsesItsHunk(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind:       model.KindAssumption,
		Text:       "Token exp is a unix timestamp in the same clock domain as now.",
		Path:       "src/auth.py",
		StartLine:  4,
		EndLine:    6,
		VerifiedBy: testRef("tests.test_auth.test_expiry"),
	})
	record(t, repo, claimlog.Options{
		Kind:       model.KindAssumption,
		Text:       "Refresh is a passthrough until rotation lands.",
		Path:       "src/auth.py",
		StartLine:  8,
		EndLine:    9,
		VerifiedBy: testRef("tests.test_auth.test_clock_skew"),
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	ev := collect(t, evidence.Sources{
		JUnit:    []string{writeJUnit(t, repo)},
		Coverage: []string{writeCoverage(t, repo)},
	})
	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), ev)

	var passing, failing model.ResolvedClaim
	for _, rc := range rl.Claims {
		if strings.Contains(rc.Claim.Text, "unix timestamp") {
			passing = rc
		}
		if strings.Contains(rc.Claim.Text, "passthrough") {
			failing = rc
		}
	}

	if passing.Anchor != model.AnchorAnchored {
		t.Errorf("passing claim anchor = %q, want %q", passing.Anchor, model.AnchorAnchored)
	}
	if passing.Coverage != model.CoverageVerified {
		t.Errorf("passing claim coverage = %q, want %q", passing.Coverage, model.CoverageVerified)
	}
	if passing.NeedsHuman() {
		t.Error("a verified claim must not need a human")
	}

	// An asserted test that FAILED must not clear. This is the whole point.
	if failing.Coverage != model.CoverageUnverified {
		t.Errorf("failing claim coverage = %q, want %q", failing.Coverage, model.CoverageUnverified)
	}
	if !failing.NeedsHuman() {
		t.Error("a claim whose asserted test failed must need a human")
	}

	if !hasVerdict(rl, model.VerdictUnverified) {
		t.Error("expected at least one unverified span")
	}
	if rl.MustReadLines() >= rl.ChangedLines() {
		t.Errorf("must_read %d should be < changed %d", rl.MustReadLines(), rl.ChangedLines())
	}
}

func TestAssertedTestThatDoesNotExistIsNotCoverage(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind:       model.KindAssumption,
		Text:       "Claims a test that was never written.",
		Path:       "src/auth.py",
		StartLine:  4,
		EndLine:    6,
		VerifiedBy: testRef("tests.test_auth.test_imaginary"),
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), ev)

	rc := rl.Claims[0]
	if rc.Coverage != model.CoverageUnverified {
		t.Errorf("coverage = %q, want %q", rc.Coverage, model.CoverageUnverified)
	}
	if !containsSubstring(rc.CoverageDetail, "not found") {
		t.Errorf("expected a 'not found' detail, got %v", rc.CoverageDetail)
	}
	if !rc.NeedsHuman() {
		t.Error("an asserted-but-absent test must not clear")
	}
}

func TestUnclaimedChangeNeverClears(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	rl := build(t, repo, "HEAD~1", nil, collect(t, evidence.Sources{}))

	if len(rl.Spans) == 0 {
		t.Fatal("expected spans for an unclaimed change")
	}
	for _, s := range rl.Spans {
		if s.Verdict != model.VerdictUnclaimed {
			t.Errorf("span %d-%d verdict = %q, want unclaimed", s.StartLine, s.EndLine, s.Verdict)
		}
	}
	if rl.MustReadLines() != rl.ChangedLines() {
		t.Errorf("must_read %d should equal changed %d", rl.MustReadLines(), rl.ChangedLines())
	}
	if policy.Evaluate(rl, policy.Defaults()).Allowed {
		t.Error("gate must not allow a wholly unclaimed changeset")
	}
}

func TestClaimRelocatesWhenCodeMoves(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind:       model.KindAssumption,
		Text:       "exp is unix seconds",
		Path:       "src/auth.py",
		StartLine:  4,
		EndLine:    6,
		VerifiedBy: testRef("tests.test_auth.test_expiry"),
	})

	// Agent keeps working and inserts above the claimed span.
	authPath := filepath.Join(repo, "src", "auth.py")
	existing, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, authPath, "import time\n\n"+string(existing))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), ev)

	rc := rl.Claims[0]
	if rc.Anchor != model.AnchorRelocated {
		t.Errorf("anchor = %q, want %q", rc.Anchor, model.AnchorRelocated)
	}
	if rc.AnchoredStart == nil || *rc.AnchoredStart != 6 {
		t.Errorf("anchored_start = %v, want 6", rc.AnchoredStart)
	}
	if rc.Coverage != model.CoverageVerified {
		t.Errorf("coverage = %q, want %q", rc.Coverage, model.CoverageVerified)
	}
}

func TestDriftedClaimIsReportedNotSilentlyKept(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind:       model.KindAssumption,
		Text:       "exp is unix seconds",
		Path:       "src/auth.py",
		StartLine:  4,
		EndLine:    6,
		VerifiedBy: testRef("tests.test_auth.test_expiry"),
	})

	// The claimed code is then rewritten. The claim no longer describes it.
	mustWrite(t, filepath.Join(repo, "src", "auth.py"),
		"def noop():\n    return None\n\ndef verify_token(t, n):\n    return True\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), ev)

	rc := rl.Claims[0]
	if rc.Anchor != model.AnchorDrifted {
		t.Errorf("anchor = %q, want %q", rc.Anchor, model.AnchorDrifted)
	}
	if !rc.NeedsHuman() {
		t.Error("a drifted claim must not clear on a passing test")
	}
}

func TestImplicitCoverageFromLineHits(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind:      model.KindAssumption,
		Text:      "No named check asserted, but the span is exercised.",
		Path:      "src/auth.py",
		StartLine: 4,
		EndLine:   6,
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	ev := collect(t, evidence.Sources{Coverage: []string{writeCoverage(t, repo)}})
	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), ev)

	rc := rl.Claims[0]
	if rc.Coverage != model.CoverageImplicit {
		t.Errorf("coverage = %q, want %q", rc.Coverage, model.CoverageImplicit)
	}
	if rc.NeedsHuman() {
		t.Error("an implicitly covered claim should not need a human")
	}
}

func TestUndeterminedAlwaysEscalates(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind:       model.KindUndetermined,
		Text:       "Could not establish whether callers pass tz-aware datetimes.",
		Path:       "src/auth.py",
		StartLine:  4,
		EndLine:    6,
		VerifiedBy: testRef("tests.test_auth.test_expiry"),
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), ev)

	if rl.Claims[0].Coverage != model.CoverageVerified {
		t.Errorf("coverage = %q, want %q", rl.Claims[0].Coverage, model.CoverageVerified)
	}
	if !rl.Claims[0].NeedsHuman() {
		t.Error("undetermined outranks a passing test")
	}
	if policy.Evaluate(rl, policy.Defaults()).Allowed {
		t.Error("gate must block on an undetermined claim")
	}
}

func TestGateBlocksOversizedChangeset(t *testing.T) {
	repo := newRepo(t)
	var b strings.Builder
	for i := range 500 {
		b.WriteString("x" + strconv.Itoa(i) + " = " + strconv.Itoa(i) + "\n")
	}
	mustWrite(t, filepath.Join(repo, "src", "big.py"), b.String())
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "big")

	rl := build(t, repo, "HEAD~1", nil, collect(t, evidence.Sources{}))
	decision := policy.Evaluate(rl, policy.Defaults())

	if decision.Allowed {
		t.Error("gate must block an oversized changeset")
	}
	if !hasRule(decision, "changeset_size") {
		t.Errorf("expected a changeset_size violation, got %v", decision.Violations)
	}
}

func TestAttestationShape(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind:       model.KindAssumption,
		Text:       "exp is unix seconds",
		Path:       "src/auth.py",
		StartLine:  4,
		EndLine:    6,
		VerifiedBy: testRef("tests.test_auth.test_expiry"),
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), ev)

	stmt := attest.BuildStatement(rl, attest.Options{Ticket: "PROJ-1", PolicyPack: "pack@0.1"})

	// Assert on the wire shape, not the struct: the `_type` key is the part a
	// verifier downstream actually depends on.
	b, err := json.Marshal(stmt)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}

	if wire["_type"] != "https://in-toto.io/Statement/v1" {
		t.Errorf("_type = %v", wire["_type"])
	}
	predicateType, _ := wire["predicateType"].(string)
	if !strings.HasSuffix(predicateType, "assumption-trail/v0.1") {
		t.Errorf("predicateType = %v", predicateType)
	}

	// Subject is the tree, not a built artifact.
	subject := wire["subject"].([]any)[0].(map[string]any)
	if _, ok := subject["digest"].(map[string]any)["gitTree"]; !ok {
		t.Errorf("subject digest missing gitTree: %v", subject)
	}

	predicate := wire["predicate"].(map[string]any)
	claims := predicate["claims"].([]any)
	if got := claims[0].(map[string]any)["coverage"]; got != "verified" {
		t.Errorf("claims[0].coverage = %v, want verified", got)
	}
	breakdown := predicate["authorRoleBreakdown"].(map[string]any)
	agent := breakdown["agent"].(map[string]any)
	if got := agent["claims"].(float64); got != 1 {
		t.Errorf("authorRoleBreakdown.agent.claims = %v, want 1", got)
	}
}

// TestAttestationRecordsTheToolThatProducedIt covers PAWL-011 AC1, AC2 and AC5.
//
// Without this the predicate is anonymous: a signed trail cannot be traced to
// the verifier that cleared its lines, and pawl's verdicts change between
// versions.
func TestAttestationRecordsTheToolThatProducedIt(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind:       model.KindAssumption,
		Text:       "exp is unix seconds",
		Path:       "src/auth.py",
		StartLine:  4,
		EndLine:    6,
		VerifiedBy: testRef("tests.test_auth.test_expiry"),
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), ev)

	stmt := attest.BuildStatement(rl, attest.Options{
		Version: "1.2.3",
		Digest:  "sha256:abc123",
	})

	if got := stmt.Predicate.Tool.Name; got != "pawl" {
		t.Errorf("tool.name = %q, want pawl", got)
	}
	// AC4: the version recorded is the one it was given, not one re-derived
	// here. Two sources for a version is two things that can disagree.
	if got := stmt.Predicate.Tool.Version; got != "1.2.3" {
		t.Errorf("tool.version = %q, want 1.2.3", got)
	}
	if got := stmt.Predicate.Tool.Digest; got != "sha256:abc123" {
		t.Errorf("tool.digest = %q, want sha256:abc123", got)
	}

	// AC5: the predicate schema moves, the type URL does not.
	if got := stmt.Predicate.SchemaVersion; got != "0.2" {
		t.Errorf("schemaVersion = %q, want 0.2", got)
	}
	if got := stmt.PredicateType; got != model.ClaimPredicateType {
		t.Errorf("predicateType = %q, want it unchanged", got)
	}
}

// TestAttestationOmitsAnUndeterminableDigest covers PAWL-011 AC3. A placeholder
// digest is worse than an absent one because it looks like an answer.
func TestAttestationOmitsAnUndeterminableDigest(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	rl := build(t, repo, "HEAD~1", nil, collect(t, evidence.Sources{}))
	stmt := attest.BuildStatement(rl, attest.Options{Version: "dev", Digest: ""})

	b, err := json.Marshal(stmt)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	tool := wire["predicate"].(map[string]any)["tool"].(map[string]any)

	if _, present := tool["digest"]; present {
		t.Errorf("digest key must be absent when undeterminable, got %v", tool["digest"])
	}
	// AC6: a development build still identifies itself rather than going
	// unattributed.
	if tool["version"] != "dev" {
		t.Errorf("tool.version = %v, want dev", tool["version"])
	}
}

// TestClaimAndPredicateSchemasVersionIndependently guards the split made for
// PAWL-011 AC5. They were one constant; raising the predicate to 0.2 would have
// silently revved the on-disk claim log format too.
func TestClaimAndPredicateSchemasVersionIndependently(t *testing.T) {
	if model.ClaimSchemaVersion == model.PredicateSchemaVersion {
		t.Fatalf("claim and predicate schema versions are equal (%q) — if they "+
			"have been recoupled, two unrelated consumers now move together",
			model.ClaimSchemaVersion)
	}

	repo := newRepo(t)
	writeFeature(t, repo)
	c := record(t, repo, claimlog.Options{
		Kind:      model.KindAssumption,
		Text:      "claim log format is not the predicate format",
		Path:      "src/auth.py",
		StartLine: 4,
		EndLine:   6,
	})
	if c.SchemaVersion != model.ClaimSchemaVersion {
		t.Errorf("claim schema_version = %q, want %q", c.SchemaVersion, model.ClaimSchemaVersion)
	}
}

// TestPartialCollapseWithinASingleHunk is the case that broke hunk-granular
// verdicts: one hunk, two claims, one verified and one not. Only the unverified
// span may reach the human.
func TestPartialCollapseWithinASingleHunk(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind:       model.KindAssumption,
		Text:       "verified span",
		Path:       "src/auth.py",
		StartLine:  4,
		EndLine:    6,
		VerifiedBy: testRef("tests.test_auth.test_expiry"),
	})
	record(t, repo, claimlog.Options{
		Kind:       model.KindAssumption,
		Text:       "unverified span",
		Path:       "src/auth.py",
		StartLine:  8,
		EndLine:    9,
		VerifiedBy: testRef("tests.test_auth.test_clock_skew"),
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := build(t, repo, "HEAD~1", loadClaims(t, repo), ev)

	if rl.MustReadLines() >= rl.ChangedLines() {
		t.Errorf("must_read %d should be < changed %d", rl.MustReadLines(), rl.ChangedLines())
	}

	surfaced := false
	for _, s := range rl.MustRead() {
		if s.StartLine == 4 && s.EndLine == 6 {
			t.Error("verified span must collapse")
		}
		if s.StartLine >= 7 {
			surfaced = true
		}
	}
	if !surfaced {
		t.Error("unverified span must surface")
	}
}

func hasVerdict(rl model.ReadingList, v model.SpanVerdict) bool {
	for _, s := range rl.Spans {
		if s.Verdict == v {
			return true
		}
	}
	return false
}

func hasRule(d policy.Decision, rule string) bool {
	for _, v := range d.Violations {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

func containsSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
