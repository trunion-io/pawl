// Package e2e runs end-to-end over a real git repo. No mocks: the failure modes
// worth catching here are all in the git and anchoring layers (C-9).
//
// These are the same ten cases as the Python suite, in the same order, asserting
// the same things. Where a test reads differently it is because Go's testing
// package works differently, not because the behaviour under test changed.
package e2e

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"trunion.io/pawl/internal/attest"
	"trunion.io/pawl/internal/calibrate"
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

func ack(t *testing.T, repo string, opts claimlog.AckOptions) model.Acknowledgement {
	t.Helper()
	a, err := claimlog.RecordAck(repo, opts)
	if err != nil {
		t.Fatalf("record ack: %v", err)
	}
	return a
}

func buildWithAcks(t *testing.T, repo, base string, claims []model.Claim,
	acks []model.Acknowledgement, ev *evidence.Evidence) model.ReadingList {
	t.Helper()
	rl, err := resolve.BuildReadingListWithAcks(repo, base, claims, acks, ev, "HEAD")
	if err != nil {
		t.Fatalf("build reading list: %v", err)
	}
	return rl
}

// TestAcknowledgedSpanCollapsesButIsNotAClaim covers PAWL-008 AC2 and AC4.
//
// The span must clear, and it must remain distinguishable from a claim-cleared
// span — the gate and the sampler both need to know which code was evidenced
// and which was waved through.
func TestAcknowledgedSpanCollapsesButIsNotAClaim(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 4, EndLine: 9})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		t.Fatal(err)
	}
	rl := buildWithAcks(t, repo, "HEAD~1", nil, acks, collect(t, evidence.Sources{}))

	if !hasVerdict(rl, model.VerdictAcknowledged) {
		t.Fatalf("expected an acknowledged span, got %v", verdicts(rl))
	}
	if hasVerdict(rl, model.VerdictClear) {
		t.Error("an acknowledgement must not read as `clear` — it is not evidence")
	}
	if rl.MustReadLines() != 0 {
		t.Errorf("acknowledged spans should collapse, %d lines still need a human", rl.MustReadLines())
	}
	// AC2: it is not a claim, anywhere.
	if len(rl.Claims) != 0 {
		t.Errorf("acknowledgement leaked into the claim corpus: %d claims", len(rl.Claims))
	}
	if rl.Summary().Claims != 0 {
		t.Error("acknowledgement inflated the claim count shown to clients")
	}
}

// TestAcknowledgementIsNotSilence covers PAWL-008 AC4's other half: a span with
// no record at all must still reach a human (C-3).
func TestAcknowledgementIsNotSilence(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	// Acknowledge only part of the change.
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 4, EndLine: 6})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		t.Fatal(err)
	}
	rl := buildWithAcks(t, repo, "HEAD~1", nil, acks, collect(t, evidence.Sources{}))

	if !hasVerdict(rl, model.VerdictUnclaimed) {
		t.Error("the unacknowledged remainder must still be unclaimed")
	}
	if rl.Summary().UnclaimedLines == 0 {
		t.Error("unclaimed lines must still be counted; the gate blocks on them")
	}
	if policy.Evaluate(rl, policy.Defaults()).Allowed {
		t.Error("gate must still block when part of the change carries no record")
	}
}

// TestClaimOutranksAcknowledgementOnOverlap covers the C-8 interaction. An
// acknowledgement must never soften a claim that needs a human.
func TestClaimOutranksAcknowledgementOnOverlap(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind:       model.KindAssumption,
		Text:       "cites a test that does not exist",
		Path:       "src/auth.py",
		StartLine:  4,
		EndLine:    6,
		VerifiedBy: testRef("tests.test_auth.test_imaginary"),
	})
	// Acknowledge the same span. It must not rescue the failing claim.
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 4, EndLine: 6})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		t.Fatal(err)
	}
	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := buildWithAcks(t, repo, "HEAD~1", loadClaims(t, repo), acks, ev)

	for _, s := range rl.MustRead() {
		if s.StartLine <= 4 && 4 <= s.EndLine {
			return // correct: the span still reaches a human
		}
	}
	t.Error("an acknowledgement must not clear a span whose claim needs a human")
}

// TestDriftedAcknowledgementStopsAccountingForItsSpan covers C-4 applied to the
// new record type. If the code changed under it, it no longer describes
// delivered code and the span falls back to unaccounted.
func TestDriftedAcknowledgementStopsAccountingForItsSpan(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 4, EndLine: 6})

	// Rewrite the acknowledged code.
	mustWrite(t, filepath.Join(repo, "src", "auth.py"),
		"def noop():\n    return None\n\ndef verify_token(t, n):\n    return True\n")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		t.Fatal(err)
	}
	rl := buildWithAcks(t, repo, "HEAD~1", nil, acks, collect(t, evidence.Sources{}))

	if hasVerdict(rl, model.VerdictAcknowledged) {
		t.Error("a drifted acknowledgement must not keep accounting for its span")
	}
	if rl.MustReadLines() == 0 {
		t.Error("the span must fall back to needing a human")
	}
}

// TestAcknowledgementRatioIsReported covers PAWL-008 AC6 — the early signal that
// claiming has decayed into box-ticking, available before the sampler has data.
func TestAcknowledgementRatioIsReported(t *testing.T) {
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
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 8, EndLine: 9})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		t.Fatal(err)
	}
	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := buildWithAcks(t, repo, "HEAD~1", loadClaims(t, repo), acks, ev)
	s := rl.Summary()

	if s.AcknowledgedLines == 0 {
		t.Fatal("acknowledged lines not counted")
	}
	// Ratio is over accounted code, not over all changed code: it measures
	// claiming quality, not how much unclaimed noise was in the diff.
	if s.AcknowledgementRatio <= 0 || s.AcknowledgementRatio >= 1 {
		t.Errorf("ratio = %v, want a fraction between 0 and 1 with both records present",
			s.AcknowledgementRatio)
	}
}

// TestAcknowledgementsAreStoredApartFromClaims covers PAWL-008 AC2 at the
// storage layer. Two files makes conflation structurally impossible.
func TestAcknowledgementsAreStoredApartFromClaims(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 4, EndLine: 6})

	claims, err := claimlog.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 0 {
		t.Errorf("acknowledgement appeared in the claim log: %d claims", len(claims))
	}
	// Stored apart from claims. PAWL-018 moved this from a sibling file to a
	// sibling directory; PAWL-008 AC2's requirement — that nothing reading
	// claims can encounter an acknowledgement — is unchanged and still what is
	// being asserted.
	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(acks) != 1 {
		t.Errorf("acknowledgement not stored: got %d", len(acks))
	}
}

// sampleFrom builds a reviewable sample from a changeset with a cleared span.
func sampleFrom(t *testing.T, repo string) calibrate.Sample {
	t.Helper()
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "exp is unix seconds",
		Path: "src/auth.py", StartLine: 4, EndLine: 6,
		VerifiedBy: testRef("tests.test_auth.test_expiry"),
	})
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 8, EndLine: 9})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		t.Fatal(err)
	}
	ev := collect(t, evidence.Sources{JUnit: []string{writeJUnit(t, repo)}})
	rl := buildWithAcks(t, repo, "HEAD~1", loadClaims(t, repo), acks, ev)

	s := calibrate.FromReadingList(rl, "0.1.0", policy.Defaults(), "sample01", time.Now())
	if len(s.Spans) == 0 {
		t.Fatal("no cleared spans to sample")
	}
	return s
}

// TestSamplingIsDeterministicPerChangeset covers PAWL-007 AC1.
//
// Deriving the decision from the tree rather than fresh randomness is what
// stops a changeset being opted out of review by re-running until it says no.
func TestSamplingIsDeterministicPerChangeset(t *testing.T) {
	tree := "e0845caa150e2a091739fb08d98cf146d8cb75c0"
	first := calibrate.Selected(tree, 0.5)
	for range 20 {
		if calibrate.Selected(tree, 0.5) != first {
			t.Fatal("the same changeset must always get the same decision; " +
				"otherwise re-running is another attempt at not being sampled")
		}
	}
	if !calibrate.Selected(tree, 1.0) {
		t.Error("rate 1.0 must always sample")
	}
	if calibrate.Selected(tree, 0) {
		t.Error("rate 0 must never sample")
	}
}

// TestClaimsStayHiddenUntilEverySpanIsJudged is PAWL-007 AC7, and the criterion
// that turns blinding from an aspiration into a mechanical check.
func TestClaimsStayHiddenUntilEverySpanIsJudged(t *testing.T) {
	repo := newRepo(t)
	s := sampleFrom(t, repo)

	if s.MayRevealClaims() {
		t.Fatal("claims must not be revealed before any span is judged")
	}

	// Attributing a cause early must be refused, not merely discouraged.
	sp := s.Spans[0]
	err := s.RecordCause(sp.Path, sp.StartLine, sp.EndLine, "someclaim", calibrate.CauseClaimFalse)
	if !errors.Is(err, calibrate.ErrPhase1Incomplete) {
		t.Errorf("phase 2 must be refused before phase 1 completes, got %v", err)
	}

	// Judge every span; only then do claims become visible.
	for _, sp := range s.Spans {
		if err := s.RecordVerdict(sp.Path, sp.StartLine, sp.EndLine,
			calibrate.VerdictCorrect, "rich", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if !s.MayRevealClaims() {
		t.Error("claims should be revealed once every span is judged")
	}
}

// TestCauseOnlyAttachesToAFalseClear — a cause explains a false clear, and
// attaching one elsewhere would put noise in the only corpus that matters.
func TestCauseOnlyAttachesToAFalseClear(t *testing.T) {
	repo := newRepo(t)
	s := sampleFrom(t, repo)
	for _, sp := range s.Spans {
		if err := s.RecordVerdict(sp.Path, sp.StartLine, sp.EndLine,
			calibrate.VerdictCorrect, "rich", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	sp := s.Spans[0]
	if err := s.RecordCause(sp.Path, sp.StartLine, sp.EndLine, "c1", calibrate.CauseAnchorWrong); err == nil {
		t.Error("a cause must not attach to a span judged correct")
	}
}

// TestFalseClearRateCountsOnlyReviewedSpans covers AC4.
//
// An unreviewed span is evidence that nobody looked, not that clearing was
// right. Counting it as correct would let the number improve by sampling more
// and reviewing less.
func TestFalseClearRateCountsOnlyReviewedSpans(t *testing.T) {
	repo := newRepo(t)
	s := sampleFrom(t, repo)

	// Judge exactly one span, leave the rest pending.
	sp := s.Spans[0]
	if err := s.RecordVerdict(sp.Path, sp.StartLine, sp.EndLine,
		calibrate.VerdictFalseClear, "rich", time.Now()); err != nil {
		t.Fatal(err)
	}

	rep := calibrate.Summarise([]calibrate.Sample{s})
	if rep.ReviewedSpans != 1 {
		t.Errorf("reviewed spans = %d, want 1", rep.ReviewedSpans)
	}
	if rep.FalseClearRate != 1.0 {
		t.Errorf("rate = %v, want 1.0 over the single reviewed span", rep.FalseClearRate)
	}
	if rep.PendingReview != 1 {
		t.Errorf("pending review = %d, want the sample counted as incomplete", rep.PendingReview)
	}
}

// TestReportBreaksDownByRoleAndCause covers AC5 and AC6 — "improve the agents"
// and "fix the anchoring" are different projects and one rate cannot tell them
// apart.
func TestReportBreaksDownByRoleAndCause(t *testing.T) {
	repo := newRepo(t)
	s := sampleFrom(t, repo)
	for _, sp := range s.Spans {
		if err := s.RecordVerdict(sp.Path, sp.StartLine, sp.EndLine,
			calibrate.VerdictFalseClear, "rich", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	// Attribute one to a pawl defect.
	for _, sp := range s.Spans {
		if len(sp.ClaimIDs) > 0 {
			if err := s.RecordCause(sp.Path, sp.StartLine, sp.EndLine,
				sp.ClaimIDs[0], calibrate.CauseAnchorWrong); err != nil {
				t.Fatal(err)
			}
			break
		}
	}

	rep := calibrate.Summarise([]calibrate.Sample{s})
	if rep.ByCause[string(calibrate.CauseAnchorWrong)] != 1 {
		t.Errorf("cause breakdown = %v, want one anchor_wrong", rep.ByCause)
	}
	if len(rep.ByRole) == 0 {
		t.Error("role breakdown is the handover curve; it must not be empty")
	}
	// An acknowledged span carries no claim, so waving code through must still
	// be visible rather than vanishing from the breakdown.
	if _, ok := rep.ByRole["acknowledged"]; !ok {
		t.Errorf("acknowledged spans missing from the role breakdown: %v", rep.ByRole)
	}
}

// TestSamplesRoundTripThroughDisk — the corpus has to survive the engagement.
func TestSamplesRoundTripThroughDisk(t *testing.T) {
	repo := newRepo(t)
	s := sampleFrom(t, repo)
	if err := calibrate.Save(repo, s); err != nil {
		t.Fatal(err)
	}
	all, err := calibrate.LoadAll(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != s.ID {
		t.Fatalf("expected the sample back, got %d", len(all))
	}
	if all[0].ToolVersion != "0.1.0" {
		t.Errorf("tool version not persisted: %q — AC8 needs it to qualify the rate", all[0].ToolVersion)
	}
	if all[0].Policy.MaxMustReadRatio != policy.Defaults().MaxMustReadRatio {
		t.Error("policy snapshot not persisted; a rate mixing thresholds is not a rate")
	}
}

// TestRuleAcknowledgesAMatchingPath covers PAWL-017 AC1 and AC6/AC7 — the
// record must say a rule produced it, and which one.
func TestRuleAcknowledgesAMatchingPath(t *testing.T) {
	repo := newRepo(t)
	mustMkdir(t, filepath.Join(repo, "vendor"))
	mustWrite(t, filepath.Join(repo, "vendor", "dep.go"), "package dep\n\nfunc X() {}\n")
	writeFeature(t, repo)

	acc := policy.Accounting{AcknowledgePaths: []string{"vendor/"}}
	matches := resolve.AutoAck(repo, pending(t, repo), acc)

	if len(matches) == 0 {
		t.Fatal("rule matched nothing")
	}
	for _, m := range matches {
		if !strings.HasPrefix(m.Path, "vendor/") {
			t.Errorf("rule matched outside its path: %s", m.Path)
		}
		if m.Rule != "path:vendor/" {
			t.Errorf("rule not named on the match: %q", m.Rule)
		}
	}

	// And the record it produces says so.
	m := matches[0]
	path, start, end, origin, rule := resolve.RuleAcknowledgement(m)
	a := ack(t, repo, claimlog.AckOptions{
		Path: path, StartLine: start, EndLine: end, Origin: origin, Rule: rule,
	})
	if a.Origin != model.OriginRule {
		t.Errorf("origin = %q, want rule", a.Origin)
	}
	if a.Rule == "" {
		t.Error("a rule-produced record must name the rule that produced it")
	}
}

// TestARuleCannotProduceAClaim is PAWL-017 AC3, the boundary the spec turns on.
// A rule may say there was nothing to assume; it can never say what was
// assumed, because it does not know.
func TestARuleCannotProduceAClaim(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)

	_, err := claimlog.Record(repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "a rule should not be able to say this",
		Path: "src/auth.py", StartLine: 4, EndLine: 6,
		Origin: model.OriginRule,
	})
	if err == nil {
		t.Fatal("a rule must not be able to produce a claim")
	}
}

// TestFormattingOnlyRuleUsesTheFingerprintNormalisation covers AC2's claim that
// the rule is provable rather than heuristic. It reuses the same normalisation
// as claim fingerprints; if the two ever disagreed, a span could be
// acknowledged as unchanged while its claim reported drift.
func TestFormattingOnlyRuleUsesTheFingerprintNormalisation(t *testing.T) {
	acc := policy.Accounting{AcknowledgeFormattingOnly: true}

	before := []string{"func x() {", "    return 1", "}"}
	reindented := []string{"func x() {", "\t\treturn 1", "}"}
	renamed := []string{"func y() {", "    return 1", "}"}

	if !acc.FormattingOnly(before, reindented) {
		t.Error("a pure reindent must read as formatting-only")
	}
	if acc.FormattingOnly(before, renamed) {
		t.Error("a rename is a real change and must not be acknowledged by rule")
	}

	off := policy.Accounting{AcknowledgeFormattingOnly: false}
	if off.FormattingOnly(before, reindented) {
		t.Error("the rule must do nothing when the client has not enabled it")
	}
}

// TestGateIgnoresRecordedCost is PAWL-017 AC10, and load-bearing. If a verdict
// ever depended on cost, an agent would be paid to claim less.
func TestGateIgnoresRecordedCost(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "expensive claim",
		Path: "src/auth.py", StartLine: 4, EndLine: 9,
		Cost: &model.Cost{Tokens: 999999, Scope: "claim_text"},
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	withCost := build(t, repo, "HEAD~1", loadClaims(t, repo), collect(t, evidence.Sources{}))

	// The same reading list with cost stripped must decide identically.
	stripped := withCost
	stripped.Claims = append([]model.ResolvedClaim(nil), withCost.Claims...)
	for i := range stripped.Claims {
		stripped.Claims[i].Claim.Cost = nil
	}

	a := policy.Evaluate(withCost, policy.Defaults())
	b := policy.Evaluate(stripped, policy.Defaults())
	if a.Allowed != b.Allowed || len(a.Violations) != len(b.Violations) {
		t.Errorf("cost changed the gate decision: %+v vs %+v", a, b)
	}
}

// TestSurfacingCacheSuppressesAnUnchangedRepeat covers AC14, and AC17 — the
// cache decides only whether to speak, never what pawl concludes.
func TestSurfacingCacheSuppressesAnUnchangedRepeat(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	spans := pending(t, repo)
	if len(spans) == 0 {
		t.Fatal("nothing pending to surface")
	}

	if resolve.AlreadySurfaced(repo, "src/auth.py", spans) {
		t.Error("nothing has been surfaced yet")
	}
	resolve.MarkSurfaced(repo, "src/auth.py", spans)
	if !resolve.AlreadySurfaced(repo, "src/auth.py", spans) {
		t.Error("an unchanged repeat must be suppressed")
	}

	// A changed set speaks again.
	changed := append([]resolve.PendingSpan(nil), spans...)
	changed = append(changed, resolve.PendingSpan{Path: "src/auth.py", StartLine: 99, EndLine: 99})
	if resolve.AlreadySurfaced(repo, "src/auth.py", changed) {
		t.Error("a changed pending set must surface")
	}

	// AC17: deleting the cache changes nothing pawl concludes.
	before := build(t, repo, "HEAD", nil, collect(t, evidence.Sources{}))
	if err := os.RemoveAll(filepath.Join(repo, ".pawl", ".cache")); err != nil {
		t.Fatal(err)
	}
	after := build(t, repo, "HEAD", nil, collect(t, evidence.Sources{}))
	if before.Summary() != after.Summary() {
		t.Error("deleting the cache changed a reading list; it must never influence a verdict")
	}
	// AC18: and an absent cache reads as not-surfaced, so it speaks again.
	if resolve.AlreadySurfaced(repo, "src/auth.py", spans) {
		t.Error("an absent cache must fail toward speaking, not toward silence")
	}
}

// TestRecordsOnTwoBranchesMergeWithoutConflict is PAWL-018 AC3, and the reason
// the spec exists.
//
// The previous shared-JSONL layout conflicted here on the second merge, because
// every branch appended at the same end of the same file. Against real git, per
// C-9 — a mock would have hidden the exact behaviour under test.
func TestRecordsOnTwoBranchesMergeWithoutConflict(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "feature")

	// Branch A records a claim.
	git(t, repo, "checkout", "-q", "-b", "feature-a")
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "branch A assumption",
		Path: "src/auth.py", StartLine: 4, EndLine: 6,
	})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "A")

	// Branch B, from the same base, records its own.
	git(t, repo, "checkout", "-q", "main")
	git(t, repo, "checkout", "-q", "-b", "feature-b")
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "branch B assumption",
		Path: "src/auth.py", StartLine: 8, EndLine: 9,
	})
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 1, EndLine: 2})
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "B")

	// Merge both, in sequence — the merge-queue case.
	git(t, repo, "checkout", "-q", "main")
	git(t, repo, "merge", "-q", "feature-a")

	cmd := exec.Command("git", "merge", "--no-edit", "feature-b")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("second merge conflicted, which is the whole failure this "+
			"layout exists to prevent:\n%s", out)
	}

	// And every record from both branches survived.
	claims := loadClaims(t, repo)
	texts := map[string]bool{}
	for _, c := range claims {
		texts[c.Text] = true
	}
	if !texts["branch A assumption"] || !texts["branch B assumption"] {
		t.Errorf("merge lost records: got %d claims, %v", len(claims), texts)
	}
	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(acks) != 1 {
		t.Errorf("acknowledgement lost in merge: got %d", len(acks))
	}
}

// TestLegacyJSONLIsStillRead covers AC6. A repository mid-adoption has records
// in the old shared log, and losing evidence is the one thing this component
// may never do.
func TestLegacyJSONLIsStillRead(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)

	// A record in the new per-file layout.
	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "new layout",
		Path: "src/auth.py", StartLine: 4, EndLine: 6,
	})
	// And one left behind in the legacy shared log.
	legacy := `{"schema_version":"0.1","id":"legacy01","ts":"2026-01-01T00:00:00Z",` +
		`"kind":"assumption","text":"old layout","path":"src/auth.py",` +
		`"start_line":8,"end_line":9,"fingerprint":"sha256:x","verified_by":[],` +
		`"author":{"role":"agent"}}` + "\n"
	mustWrite(t, claimlog.LogPath(repo), legacy)

	claims := loadClaims(t, repo)
	seen := map[string]bool{}
	for _, c := range claims {
		seen[c.Text] = true
	}
	if !seen["new layout"] || !seen["old layout"] {
		t.Errorf("expected records from both layouts, got %v", seen)
	}
}

// TestRecordFilesAreWriteOnce covers AC4. Append-only was the property worth
// keeping; write-once holds it more strongly because there is no edit path.
func TestRecordFilesAreWriteOnce(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	c := record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "first",
		Path: "src/auth.py", StartLine: 4, EndLine: 6,
	})

	// Re-writing the same id must be refused rather than silently overwriting.
	if _, err := claimlog.Append(repo, c); err == nil {
		t.Error("re-writing an existing record must fail; records are never modified")
	}
}

// TestPruneRemovesAttestedRecords covers AC7.
func TestPruneRemovesAttestedRecords(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	keep := record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "keep",
		Path: "src/auth.py", StartLine: 4, EndLine: 6,
	})
	drop := record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "drop",
		Path: "src/auth.py", StartLine: 8, EndLine: 9,
	})

	removed, skipped, err := claimlog.Prune(repo, []string{drop.ID, "nosuchid"})
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if len(skipped) != 1 || skipped[0] != "nosuchid" {
		t.Errorf("skipped = %v, want [nosuchid]", skipped)
	}

	claims := loadClaims(t, repo)
	if len(claims) != 1 || claims[0].ID != keep.ID {
		t.Errorf("prune removed the wrong record: %d left", len(claims))
	}
}

func pending(t *testing.T, repo string, paths ...string) []resolve.PendingSpan {
	t.Helper()
	claims, err := claimlog.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		t.Fatal(err)
	}
	spans, err := resolve.Pending(repo, claims, acks, paths)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	return spans
}

// TestPendingReportsUncommittedUnaccountedSpans covers PAWL-016 AC1 and AC2.
// The edit is deliberately NOT committed — that is the moment a hook fires.
func TestPendingReportsUncommittedUnaccountedSpans(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo) // uncommitted

	spans := pending(t, repo)
	if len(spans) == 0 {
		t.Fatal("expected pending spans for an uncommitted, unaccounted edit")
	}
	for _, s := range spans {
		if s.Path != "src/auth.py" {
			t.Errorf("unexpected path %q", s.Path)
		}
	}
}

// TestPendingClearsWhenAccounted covers AC1 from the other side: both record
// types remove a span from the pending list, and neither needs evidence (AC3).
func TestPendingClearsWhenAccounted(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)

	before := len(pending(t, repo))
	if before == 0 {
		t.Fatal("nothing pending to begin with")
	}

	record(t, repo, claimlog.Options{
		Kind: model.KindAssumption, Text: "exp is unix seconds",
		Path: "src/auth.py", StartLine: 4, EndLine: 6,
	})
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 8, EndLine: 9})

	after := pending(t, repo)
	for _, s := range after {
		for line := s.StartLine; line <= s.EndLine; line++ {
			if (4 <= line && line <= 6) || (8 <= line && line <= 9) {
				t.Errorf("line %d is accounted for but still pending", line)
			}
		}
	}
	if len(after) >= before {
		t.Errorf("pending did not shrink: %d -> %d", before, len(after))
	}
}

// TestPendingReturnsWhenARecordDrifts applies C-4 at edit time: a record whose
// fingerprint no longer matches has stopped describing this code, so its span
// is pending again.
func TestPendingReturnsWhenARecordDrifts(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 4, EndLine: 6})

	if covered := pending(t, repo); spansCover(covered, 4) {
		t.Fatal("acknowledged span should not be pending yet")
	}

	// Rewrite the acknowledged code.
	mustWrite(t, filepath.Join(repo, "src", "auth.py"),
		"def noop():\n    return None\n\ndef verify_token(t, n):\n    return True\n")

	if !spansCover(pending(t, repo), 4) {
		t.Error("a drifted acknowledgement must leave its span pending again")
	}
}

// TestPendingRespectsPathFilter — a hook firing on one edit does not want the
// whole tree.
func TestPendingRespectsPathFilter(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	mustWrite(t, filepath.Join(repo, "src", "other.py"), "x = 1\ny = 2\n")

	all := pending(t, repo)
	filtered := pending(t, repo, "src/other.py")

	if len(filtered) == 0 {
		t.Fatal("expected pending spans for the filtered path")
	}
	for _, s := range filtered {
		if s.Path != "src/other.py" {
			t.Errorf("filter leaked %q", s.Path)
		}
	}
	if len(filtered) >= len(all) {
		t.Error("filtered result should be a subset")
	}
}

// TestPendingExcludesTheClaimLog — the records must not generate work for
// themselves, or accounting never terminates.
func TestPendingExcludesTheClaimLog(t *testing.T) {
	repo := newRepo(t)
	writeFeature(t, repo)
	ack(t, repo, claimlog.AckOptions{Path: "src/auth.py", StartLine: 4, EndLine: 9})

	for _, s := range pending(t, repo) {
		if strings.HasPrefix(s.Path, ".pawl/") {
			t.Errorf("the record log is itself pending: %s", s.Path)
		}
	}
}

func spansCover(spans []resolve.PendingSpan, line int) bool {
	for _, s := range spans {
		if s.StartLine <= line && line <= s.EndLine {
			return true
		}
	}
	return false
}

func verdicts(rl model.ReadingList) []model.SpanVerdict {
	var out []model.SpanVerdict
	for _, s := range rl.Spans {
		out = append(out, s.Verdict)
	}
	return out
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
