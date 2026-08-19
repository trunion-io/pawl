// Package cli is pawl's command line.
//
// Four commands, matching the four things the kit has to do in a client repo:
//
//	pawl claim    emit a claim at the moment of the edit (harness hook)
//	pawl verify   resolve claims against evidence, print the reading list
//	pawl attest   emit the in-toto Statement for signing with cosign
//	pawl gate     evaluate the policy pack and exit non-zero on violation
//
// No daemon, no database, no hosted service. It installs into a repo and a CI
// job.
//
// Built on the standard library's flag package. Typer gave the Python version
// subcommands, enum validation, help text and repeatable options from type
// annotations alone; here each of those is a few explicit lines. This file is
// where most of the port's extra length lives, and it is the fairest single
// place to judge whether the trade is worth it.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"trunion.io/pawl/internal/calibrate"

	"trunion.io/pawl/internal/attest"
	"trunion.io/pawl/internal/claimlog"
	"trunion.io/pawl/internal/evidence"
	"trunion.io/pawl/internal/gitutil"
	"trunion.io/pawl/internal/harness"
	"trunion.io/pawl/internal/model"
	"trunion.io/pawl/internal/policy"
	"trunion.io/pawl/internal/resolve"
)

const usage = `pawl — Provenance of Agent-Written Lines.

Edit-time claim capture and changeset verification for agentic delivery.

Commands:
  claim    Record a claim against a span of source. Called from a harness hook.
  ack      Account for a changed span that carries nothing to assume.
  pending  List changed spans in the working tree that carry no record yet.
  prune    Remove record files for a changeset that has been attested.
  sample   Select this changeset for calibration review, at the configured rate.
  review   Review a sampled changeset. Two phases: verdict, then cause.
  calibrate  Report the false-clear rate over reviewed samples.
  setup    Install pawl's hook into your harness settings.
  hook     Harness hook entry point. Not for interactive use.
  verify   Resolve claims against evidence and print the reading list.
  attest   Emit the in-toto Statement. Sign it with ` + "`cosign attest-blob`" + `.
  gate     Evaluate the policy pack. Exit 1 on violation.
  version  Print the version of this binary.

Run "pawl <command> -h" for the options of a command.
`

// stringSlice makes a flag repeatable, which typer.Option gave the Python
// version by declaring the parameter as list[Path].
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// evidenceFlags is the option set shared by verify, attest and gate.
type evidenceFlags struct {
	repo        string
	base        string
	junit       stringSlice
	coverage    stringSlice
	typecheck   string
	policy      string
	spec        string
	stripPrefix string
}

func (e *evidenceFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&e.repo, "repo", ".", "Repository root.")
	fs.StringVar(&e.base, "base", "origin/main", "Base ref for the changeset.")
	fs.Var(&e.junit, "junit", "junit XML file. Repeatable.")
	fs.Var(&e.coverage, "coverage", "Cobertura XML file. Repeatable.")
	fs.StringVar(&e.typecheck, "typecheck", "", "Typecheck report (JSON).")
	fs.StringVar(&e.policy, "policy-results", "", "Policy decision log (JSON).")
	fs.StringVar(&e.spec, "spec", "", "Signed spec attestation (JSON).")
	fs.StringVar(&e.stripPrefix, "strip-prefix", "", "Prefix to strip from coverage paths.")
}

// Run dispatches a subcommand. version is injected at build time and reported
// by `pawl version`; a client pinning pawl in CI needs to be able to see which
// binary they actually got.
func Run(args []string, version string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stdout, usage)
		return 0
	}
	switch args[0] {
	case "claim":
		return cmdClaim(args[1:])
	case "ack":
		return cmdAck(args[1:])
	case "pending":
		return cmdPending(args[1:])
	case "prune":
		return cmdPrune(args[1:])
	case "sample":
		return cmdSample(args[1:], version)
	case "review":
		return cmdReview(args[1:])
	case "calibrate":
		return cmdCalibrate(args[1:])
	case "setup":
		return cmdSetup(args[1:])
	case "hook":
		return cmdHook(args[1:])
	case "verify":
		return cmdVerify(args[1:])
	case "attest":
		return cmdAttest(args[1:], version)
	case "gate":
		return cmdGate(args[1:])
	case "version", "--version", "-version":
		fmt.Printf("pawl %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

// takeLeadingPositional pulls a leading non-flag argument off the front.
//
// The standard flag package stops parsing at the first non-flag argument, so
// `pawl review <id> --span x` would silently parse zero flags. typer and click
// intersperse positionals and options; flag does not. Every command here that
// takes a leading positional needs this, and forgetting it has now produced the
// same silent no-op twice — once in `claim`, once in `review`.
func takeLeadingPositional(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "error:", err)
	return 2
}

// parseEvidenceRefs accepts type:ref, e.g. test:tests/test_auth.py::test_expiry
func parseEvidenceRefs(values []string) ([]model.EvidenceRef, error) {
	refs := []model.EvidenceRef{}
	for _, value := range values {
		kind, ref, found := strings.Cut(value, ":")
		if !found {
			names := make([]string, 0, len(model.EvidenceTypes()))
			for _, t := range model.EvidenceTypes() {
				names = append(names, string(t))
			}
			return nil, fmt.Errorf("expected type:ref, got %q. Types: %s",
				value, strings.Join(names, ", "))
		}
		etype := model.EvidenceType(kind)
		if !etype.Valid() {
			return nil, fmt.Errorf("unknown evidence type %q", kind)
		}
		refs = append(refs, model.EvidenceRef{Type: etype, Ref: ref})
	}
	return refs, nil
}

func cmdClaim(args []string) int {
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	var (
		path        = fs.String("path", "", "File the claim is about.")
		lines       = fs.String("lines", "", "Line range, e.g. 40-58.")
		kind        = fs.String("kind", string(model.KindAssumption), "Claim kind.")
		role        = fs.String("role", string(model.RoleAgent), "Author role.")
		harnessFlag = fs.String("harness", "", "e.g. claude-code")
		modelName   = fs.String("model", "", "Model identifier.")
		identity    = fs.String("identity", "", "Human identity for expert/client roles.")
		session     = fs.String("session", "", "Session identifier.")
		ticket      = fs.String("ticket", "", "e.g. PROJ-1234")
		repo        = fs.String("repo", ".", "Repository root.")
		verifiedBy  stringSlice
	)
	fs.Var(&verifiedBy, "verified-by", "type:ref, repeatable.")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `usage: pawl claim "<text>" --path <file> --lines <a-b> [options]`)
		fs.PrintDefaults()
	}

	// The standard flag package stops parsing at the first non-flag argument,
	// so `pawl claim "text" --path x` would silently parse zero flags. typer
	// and click intersperse positionals and options; flag does not. The claim
	// text leads the command in every example we publish, so pull it off the
	// front by hand rather than making users write the flags first.
	text, args := takeLeadingPositional(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if text == "" {
		// Also accept it trailing the flags, which is what a shell user who
		// forgot the order will try next.
		if fs.NArg() != 1 {
			fs.Usage()
			return 2
		}
		text = fs.Arg(0)
	} else if fs.NArg() != 0 {
		fs.Usage()
		return 2
	}

	claimKind := model.ClaimKind(*kind)
	if !claimKind.Valid() {
		return fail(fmt.Errorf("unknown claim kind %q", *kind))
	}
	authorRole := model.AuthorRole(*role)
	if !authorRole.Valid() {
		return fail(fmt.Errorf("unknown author role %q", *role))
	}
	if *path == "" || *lines == "" {
		fs.Usage()
		return 2
	}

	start, end, err := parseLineRange(*lines)
	if err != nil {
		return fail(err)
	}
	refs, err := parseEvidenceRefs(verifiedBy)
	if err != nil {
		return fail(err)
	}

	recorded, err := claimlog.Record(*repo, claimlog.Options{
		Kind:       claimKind,
		Text:       text,
		Path:       *path,
		StartLine:  start,
		EndLine:    end,
		VerifiedBy: refs,
		Author: &model.Author{
			Role:     authorRole,
			Harness:  *harnessFlag,
			Model:    *modelName,
			Identity: *identity,
		},
		Session: *session,
		Ticket:  *ticket,
	})
	if err != nil {
		return fail(err)
	}
	fmt.Printf("%s  %s  %s:%d-%d\n", recorded.ID, recorded.Kind, *path, start, end)
	return 0
}

// cmdAck records an acknowledgement. Note the absence of a text argument:
// PAWL-008 AC3 requires that accounting for a trivial span costs an agent no
// prose, so there is nowhere to put any.
func cmdAck(args []string) int {
	fs := flag.NewFlagSet("ack", flag.ContinueOnError)
	var (
		path        = fs.String("path", "", "File the acknowledgement is about.")
		lines       = fs.String("lines", "", "Line range, e.g. 40-58.")
		role        = fs.String("role", string(model.RoleAgent), "Author role.")
		harnessFlag = fs.String("harness", "", "e.g. claude-code")
		modelStr    = fs.String("model", "", "Model identifier.")
		identity    = fs.String("identity", "", "Human identity for expert/client roles.")
		session     = fs.String("session", "", "Session identifier.")
		repo        = fs.String("repo", ".", "Repository root.")
		auto        = fs.Bool("auto", false, "Apply deterministic rules from .pawl/policy.toml instead of taking --path/--lines.")
		dryRun      = fs.Bool("dry-run", false, "With --auto: report what the rules match, record nothing.")
	)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: pawl ack --path <file> --lines <a-b> [options]")
		fmt.Fprintln(fs.Output(), "       pawl ack --auto [--dry-run]")
		fmt.Fprintln(fs.Output(), "\nRecords that a changed span carries nothing to assume.")
		fmt.Fprintln(fs.Output(), "It is not a claim, and it does not clear a span on evidence —")
		fmt.Fprintln(fs.Output(), "acknowledged spans are sampled for review.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *auto {
		return cmdAckAuto(*repo, *dryRun, *harnessFlag, *modelStr, *session)
	}
	if *path == "" || *lines == "" {
		fs.Usage()
		return 2
	}
	authorRole := model.AuthorRole(*role)
	if !authorRole.Valid() {
		return fail(fmt.Errorf("unknown author role %q", *role))
	}
	start, end, err := parseLineRange(*lines)
	if err != nil {
		return fail(err)
	}

	recorded, err := claimlog.RecordAck(*repo, claimlog.AckOptions{
		Path:      *path,
		StartLine: start,
		EndLine:   end,
		Author: &model.Author{
			Role:     authorRole,
			Harness:  *harnessFlag,
			Model:    *modelStr,
			Identity: *identity,
		},
		Session: *session,
	})
	if err != nil {
		return fail(err)
	}
	fmt.Printf("%s  acknowledged  %s:%d-%d\n", recorded.ID, *path, start, end)
	return 0
}

// cmdPending is what a harness hook calls after an edit (PAWL-016).
//
// It reports unaccounted spans in the working tree and always exits 0 — a hook
// must never fail an edit loop because the accounting tool had an opinion
// (AC9). Enforcement is the gate's job, not this command's.
func cmdPending(args []string) int {
	fs := flag.NewFlagSet("pending", flag.ContinueOnError)
	var (
		repo   = fs.String("repo", ".", "Repository root.")
		asJSON = fs.Bool("json", false, "Emit pending spans as JSON.")
		quiet  = fs.Bool("quiet", false, "Print nothing; exit 0 regardless.")
		once   = fs.Bool("once", false, "Stay silent if this exact pending set was already surfaced.")
	)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: pawl pending [--repo .] [--json] [<file>...]")
		fmt.Fprintln(fs.Output(), "\nLists changed spans in the working tree carrying neither a")
		fmt.Fprintln(fs.Output(), "claim nor an acknowledgement. Needs no evidence files.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	claims, err := claimlog.Load(*repo)
	if err != nil {
		return pendingSoftFail(err, *quiet)
	}
	acks, err := claimlog.LoadAcks(*repo)
	if err != nil {
		return pendingSoftFail(err, *quiet)
	}
	spans, err := resolve.Pending(*repo, claims, acks, fs.Args())
	if err != nil {
		return pendingSoftFail(err, *quiet)
	}

	// AC14: an agent editing the same file repeatedly should not be told the
	// same thing each time. The cache decides only whether to speak; it never
	// changes what is reported (AC17).
	if *once && len(fs.Args()) == 1 {
		file := fs.Args()[0]
		if resolve.AlreadySurfaced(*repo, file, spans) {
			return 0
		}
		resolve.MarkSurfaced(*repo, file, spans)
	}

	if *quiet {
		return 0
	}
	if *asJSON {
		b, err := json.MarshalIndent(spans, "", "  ")
		if err != nil {
			return pendingSoftFail(err, *quiet)
		}
		fmt.Println(string(b))
		return 0
	}

	if len(spans) == 0 {
		fmt.Println("nothing pending: every changed span carries a claim or an acknowledgement")
		return 0
	}
	total := 0
	for _, s := range spans {
		total += s.Lines()
	}
	fmt.Printf("%d span(s), %d line(s) with no record yet:\n", len(spans), total)
	for _, s := range spans {
		fmt.Printf("  %s:%d-%d\n", s.Path, s.StartLine, s.EndLine)
	}
	fmt.Println()
	fmt.Println("Record one of:")
	fmt.Println("  pawl claim \"<what you assumed>\" --path <file> --lines <a-b> [--verified-by test:<id>]")
	fmt.Println("  pawl ack --path <file> --lines <a-b>      # nothing to assume here")
	return 0
}

// pendingSoftFail reports to stderr and still exits 0. A hook that breaks the
// edit loop when pawl misbehaves will be uninstalled the first time it does,
// and would deserve to be (PAWL-016 AC9).
func pendingSoftFail(err error, quiet bool) int {
	if !quiet {
		fmt.Fprintln(os.Stderr, "pawl pending:", err)
	}
	return 0
}

// cmdPrune removes the record files named by an attestation (PAWL-018 AC7).
//
// Records are working state for an unmerged changeset; the signed attestation
// embeds every one of them and git history keeps them regardless. Pruning only
// what a trail provably names is what makes this safe — it will never remove a
// record that is not already preserved somewhere durable.
func cmdPrune(args []string) int {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	var (
		repo     = fs.String("repo", ".", "Repository root.")
		attested = fs.String("attested", "", "Attestation whose records to remove. Required.")
		dryRun   = fs.Bool("dry-run", false, "Report what would be removed, remove nothing.")
	)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: pawl prune --attested <trail.intoto.json> [--repo .] [--dry-run]")
		fmt.Fprintln(fs.Output(), "\nRemoves record files the given attestation already embeds.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *attested == "" {
		fs.Usage()
		return 2
	}

	b, err := os.ReadFile(*attested)
	if err != nil {
		return fail(err)
	}
	var stmt struct {
		Predicate struct {
			Claims []struct {
				ID string `json:"id"`
			} `json:"claims"`
		} `json:"predicate"`
	}
	if err := json.Unmarshal(b, &stmt); err != nil {
		return fail(fmt.Errorf("%s: not a readable attestation: %w", *attested, err))
	}
	if len(stmt.Predicate.Claims) == 0 {
		fmt.Println("attestation names no records; nothing to prune")
		return 0
	}

	ids := make([]string, 0, len(stmt.Predicate.Claims))
	for _, c := range stmt.Predicate.Claims {
		ids = append(ids, c.ID)
	}

	if *dryRun {
		fmt.Printf("would remove %d record(s) named by %s\n", len(ids), *attested)
		for _, id := range ids {
			fmt.Printf("  %s\n", id)
		}
		return 0
	}

	removed, skipped, err := claimlog.Prune(*repo, ids)
	if err != nil {
		return fail(err)
	}
	fmt.Printf("removed %d record(s)\n", removed)
	if len(skipped) > 0 {
		// Legacy JSONL entries land here: removing one line would rewrite a
		// shared append-only file, which is the edit write-once storage exists
		// to prevent.
		fmt.Printf("%d not present as record files (legacy log entries are left alone)\n", len(skipped))
	}
	return 0
}

// cmdAckAuto applies the client's deterministic rules (PAWL-017 AC1).
//
// Rules can only ever acknowledge — say there was nothing to assume. They can
// never claim (AC3). Every record it writes carries the rule that produced it,
// so a rule that turns out to be wrong is traceable to its records (AC7).
func cmdAckAuto(repo string, dryRun bool, harness, modelName, session string) int {
	pol, err := policy.Load(repo)
	if err != nil {
		return fail(err)
	}
	if pol.Accounting.Empty() {
		fmt.Println("no acknowledgement rules configured; add an [accounting] table to .pawl/policy.toml")
		return 0
	}

	claims, err := claimlog.Load(repo)
	if err != nil {
		return fail(err)
	}
	acks, err := claimlog.LoadAcks(repo)
	if err != nil {
		return fail(err)
	}
	spans, err := resolve.Pending(repo, claims, acks, nil)
	if err != nil {
		return fail(err)
	}

	matches := resolve.AutoAck(repo, spans, pol.Accounting)
	if len(matches) == 0 {
		fmt.Println("no pending spans matched a rule")
		return 0
	}

	if dryRun {
		fmt.Printf("%d span(s) would be acknowledged by rule:\n", len(matches))
		for _, m := range matches {
			fmt.Printf("  %s:%d-%d  [%s]\n", m.Path, m.StartLine, m.EndLine, m.Rule)
		}
		return 0
	}

	byRule := map[string]int{}
	for _, m := range matches {
		path, start, end, origin, rule := resolve.RuleAcknowledgement(m)
		if _, err := claimlog.RecordAck(repo, claimlog.AckOptions{
			Path: path, StartLine: start, EndLine: end,
			Author:  &model.Author{Role: model.RoleAgent, Harness: harness, Model: modelName},
			Session: session,
			Origin:  origin,
			Rule:    rule,
		}); err != nil {
			return fail(err)
		}
		byRule[rule]++
	}

	fmt.Printf("acknowledged %d span(s) by rule:\n", len(matches))
	for rule, n := range byRule {
		fmt.Printf("  %-24s %d\n", rule, n)
	}
	fmt.Println()
	fmt.Println("These were recorded, not skipped — they are sampled like any other")
	fmt.Println("acknowledgement, so a wrong rule surfaces as a false clear.")
	return 0
}

// cmdSample selects a changeset for calibration review (PAWL-007 AC1).
func cmdSample(args []string, version string) int {
	fs := flag.NewFlagSet("sample", flag.ContinueOnError)
	e := &evidenceFlags{}
	e.register(fs)
	rate := fs.Float64("rate", 0.05, "Fraction of cleared changesets to sample.")
	force := fs.Bool("force", false, "Sample regardless of the rate.")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rl, err := resolveReadingList(e)
	if err != nil {
		return fail(err)
	}
	if !*force && !calibrate.Selected(rl.Tree, *rate) {
		fmt.Printf("not sampled (rate %.2f)\n", *rate)
		return 0
	}

	pol, err := policy.Load(e.repo)
	if err != nil {
		return fail(err)
	}
	id := rl.Tree
	if len(id) > 12 {
		id = id[:12]
	}
	s := calibrate.FromReadingList(rl, version, pol, id, time.Now())
	if len(s.Spans) == 0 {
		fmt.Println("nothing cleared in this changeset; nothing to review")
		return 0
	}
	if err := calibrate.Save(e.repo, s); err != nil {
		return fail(err)
	}
	fmt.Printf("sampled %s — %d cleared span(s) awaiting review\n", s.ID, len(s.Spans))
	fmt.Printf("  pawl review %s\n", s.ID)
	return 0
}

// cmdReview drives the two-phase review (AC2, AC3, AC7).
func cmdReview(args []string) int {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	var (
		repo     = fs.String("repo", ".", "Repository root.")
		list     = fs.Bool("list", false, "List samples awaiting review.")
		span     = fs.String("span", "", "Span to judge, as path:start-end.")
		verdict  = fs.String("verdict", "", "correct | false_clear")
		claimID  = fs.String("claim", "", "Claim to attribute a cause to.")
		cause    = fs.String("cause", "", "claim_false | claim_incomplete | anchor_wrong | evidence_hollow")
		reviewer = fs.String("reviewer", "", "Who reviewed this.")
	)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: pawl review --list")
		fmt.Fprintln(fs.Output(), "       pawl review <id>")
		fmt.Fprintln(fs.Output(), "       pawl review <id> --span <path:a-b> --verdict <v> --reviewer <who>")
		fmt.Fprintln(fs.Output(), "       pawl review <id> --span <path:a-b> --claim <id> --cause <c>")
		fs.PrintDefaults()
	}
	id, args := takeLeadingPositional(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *list {
		samples, err := calibrate.LoadAll(*repo)
		if err != nil {
			return fail(err)
		}
		if len(samples) == 0 {
			fmt.Println("no samples")
			return 0
		}
		for _, s := range samples {
			state := "reviewed"
			if !s.Phase1Complete() {
				state = fmt.Sprintf("%d span(s) pending", len(s.Pending()))
			}
			fmt.Printf("  %s  %s  %s\n", s.ID, s.TS.Format("2006-01-02"), state)
		}
		return 0
	}

	if id == "" {
		id = fs.Arg(0)
	}
	if id == "" {
		fs.Usage()
		return 2
	}
	s, err := calibrate.Load(*repo, id)
	if err != nil {
		return fail(err)
	}

	switch {
	case *span != "" && *verdict != "":
		path, start, end, err := parseSpan(*span)
		if err != nil {
			return fail(err)
		}
		if err := s.RecordVerdict(path, start, end, calibrate.Verdict(*verdict), *reviewer, time.Now()); err != nil {
			return fail(err)
		}
	case *span != "" && *cause != "":
		path, start, end, err := parseSpan(*span)
		if err != nil {
			return fail(err)
		}
		if err := s.RecordCause(path, start, end, *claimID, calibrate.Cause(*cause)); err != nil {
			return fail(err)
		}
	default:
		return showSample(s)
	}

	if err := calibrate.Save(*repo, s); err != nil {
		return fail(err)
	}
	return showSample(s)
}

// showSample renders the review. Claim ids are withheld until every span has a
// verdict (AC7) — a reviewer who has read the claim cannot then judge the code
// independently of it.
func showSample(s calibrate.Sample) int {
	fmt.Printf("sample %s  tree %s  pawl %s\n", s.ID, s.Tree[:min(12, len(s.Tree))], s.ToolVersion)
	fmt.Println()

	if !s.MayRevealClaims() {
		fmt.Println("PHASE 1 — read each span and judge it WITHOUT seeing what was claimed.")
		fmt.Println("Ask only: did anything here need a human?")
		fmt.Println()
	}
	for _, sp := range s.Spans {
		state := string(sp.Reviewed)
		if sp.Reviewed == calibrate.VerdictPending {
			state = "pending"
		}
		fmt.Printf("  %s:%d-%d  [%s] %s\n", sp.Path, sp.StartLine, sp.EndLine, sp.Verdict, state)
		if s.MayRevealClaims() && len(sp.ClaimIDs) > 0 {
			fmt.Printf("      claims: %s\n", strings.Join(sp.ClaimIDs, ", "))
		}
		for _, c := range sp.Causes {
			fmt.Printf("      cause: %s (%s)\n", c.Cause, c.ClaimID)
		}
	}
	fmt.Println()
	if !s.MayRevealClaims() {
		fmt.Printf("%d span(s) still need a verdict. Claims stay hidden until they have one.\n", len(s.Pending()))
		fmt.Println("  pawl review " + s.ID + " --span <path:a-b> --verdict correct|false_clear --reviewer <who>")
	} else if len(s.FalseClears()) > 0 {
		fmt.Println("PHASE 2 — claims revealed. Attribute each false clear:")
		fmt.Println("  pawl review " + s.ID + " --span <path:a-b> --claim <id> --cause <cause>")
	} else {
		fmt.Println("review complete: no false clears")
	}
	return 0
}

func parseSpan(s string) (string, int, int, error) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", 0, 0, fmt.Errorf("expected path:start-end, got %q", s)
	}
	start, end, err := parseLineRange(s[i+1:])
	if err != nil {
		return "", 0, 0, err
	}
	return s[:i], start, end, nil
}

// cmdCalibrate reports the rate (AC4-AC6).
func cmdCalibrate(args []string) int {
	fs := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	repo := fs.String("repo", ".", "Repository root.")
	asJSON := fs.Bool("json", false, "Emit the report as JSON.")
	since := fs.String("since", "", "Only samples on or after this date (YYYY-MM-DD).")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	samples, err := calibrate.LoadAll(*repo)
	if err != nil {
		return fail(err)
	}
	if *since != "" {
		cutoff, err := time.Parse("2006-01-02", *since)
		if err != nil {
			return fail(fmt.Errorf("bad --since date: %w", err))
		}
		var kept []calibrate.Sample
		for _, s := range samples {
			if !s.TS.Before(cutoff) {
				kept = append(kept, s)
			}
		}
		samples = kept
	}

	rep := calibrate.Summarise(samples)
	if *asJSON {
		b, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return fail(err)
		}
		fmt.Println(string(b))
		return 0
	}

	fmt.Printf("%d sample(s), %d reviewed span(s), %d pending review\n",
		rep.Samples, rep.ReviewedSpans, rep.PendingReview)
	if rep.ReviewedSpans == 0 {
		fmt.Println("\nno reviewed spans yet; there is no rate to report")
		return 0
	}
	fmt.Printf("false-clear rate: %.1f%% (%d of %d)\n",
		rep.FalseClearRate*100, rep.FalseClears, rep.ReviewedSpans)

	if len(rep.ByRole) > 0 {
		fmt.Println("\nBY AUTHOR ROLE")
		for _, role := range sortedKeys(rep.ByRole) {
			e := rep.ByRole[role]
			fmt.Printf("  %-14s %.1f%%  (%d of %d)\n", role, e.Rate*100, e.FalseClears, e.Spans)
		}
	}
	if len(rep.ByCause) > 0 {
		fmt.Println("\nBY CAUSE")
		for _, c := range sortedKeysInt(rep.ByCause) {
			fmt.Printf("  %-20s %d\n", c, rep.ByCause[c])
		}
	}
	if len(rep.ToolVersions) > 1 {
		fmt.Printf("\nNOTE: samples span %d pawl versions (%s). Verdicts change between\n",
			len(rep.ToolVersions), strings.Join(rep.ToolVersions, ", "))
		fmt.Println("versions, so this rate mixes verifiers and needs qualifying.")
	}
	return 0
}

func sortedKeys(m map[string]calibrate.RoleRate) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysInt(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cmdHook is the harness entry point (PAWL-019). Never fails the edit loop.
func cmdHook(args []string) int {
	harnessName, _ := takeLeadingPositional(args)
	switch harnessName {
	case "claude-code":
		_ = harness.ClaudeCodeHook(os.Stdin, os.Stdout)
		return 0
	case "":
		fmt.Fprintln(os.Stderr, "usage: pawl hook claude-code")
		return 2
	default:
		fmt.Fprintf(os.Stderr, "unknown harness %q; known: claude-code\n", harnessName)
		return 2
	}
}

// cmdSetup installs the hook into the user's harness settings (AC1-AC6).
func cmdSetup(args []string) int {
	harnessName, args := takeLeadingPositional(args)
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "Show the resulting settings; write nothing.")
	uninstall := fs.Bool("uninstall", false, "Remove pawl's hook, leaving everything else.")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: pawl setup claude [--dry-run] [--uninstall]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if harnessName != "claude" && harnessName != "claude-code" {
		fs.Usage()
		return 2
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fail(err)
	}

	var p harness.Plan
	if *uninstall {
		p, err = harness.Uninstall(home)
	} else {
		p, err = harness.Install(home)
	}
	if err != nil {
		return fail(err)
	}

	if p.AlreadySet {
		if *uninstall {
			fmt.Printf("pawl's hook is not in %s; nothing to remove\n", p.Path)
		} else {
			fmt.Printf("already installed in %s\n", p.Path)
		}
		return 0
	}

	if *dryRun {
		fmt.Printf("would write %s:\n\n%s\n", p.Path, p.Result)
		return 0
	}
	if err := p.Apply(); err != nil {
		return fail(err)
	}

	if *uninstall {
		fmt.Printf("removed pawl's hook from %s\n", p.Path)
	} else {
		fmt.Printf("installed pawl's hook in %s\n", p.Path)
	}
	if p.Backup != "" {
		fmt.Printf("previous settings backed up to %s\n", p.Backup)
	}
	if !*uninstall {
		fmt.Println()
		fmt.Println("Open /hooks once, or restart, for the harness to pick it up.")
		fmt.Println("It stays silent in repositories that have no .pawl directory.")
	}
	return 0
}

func parseLineRange(s string) (int, int, error) {
	startS, endS, found := strings.Cut(s, "-")
	start, err := strconv.Atoi(strings.TrimSpace(startS))
	if err != nil {
		return 0, 0, fmt.Errorf("bad line range %q", s)
	}
	if !found || strings.TrimSpace(endS) == "" {
		return start, start, nil
	}
	end, err := strconv.Atoi(strings.TrimSpace(endS))
	if err != nil {
		return 0, 0, fmt.Errorf("bad line range %q", s)
	}
	return start, end, nil
}

func resolveReadingList(e *evidenceFlags) (model.ReadingList, error) {
	claims, err := claimlog.Load(e.repo)
	if err != nil {
		return model.ReadingList{}, err
	}
	acks, err := claimlog.LoadAcks(e.repo)
	if err != nil {
		return model.ReadingList{}, err
	}

	baseRef, err := gitutil.MergeBase(e.repo, e.base, "HEAD")
	if err != nil {
		// No merge base — a shallow clone or a ref that is already a commit.
		// Use what we were given rather than failing the run.
		baseRef = e.base
	}

	changedPaths := map[string]bool{}
	for _, c := range claims {
		changedPaths[c.Path] = true
	}

	ev, err := evidence.Collect(evidence.Sources{
		JUnit:        e.junit,
		Coverage:     e.coverage,
		Typecheck:    e.typecheck,
		Policy:       e.policy,
		Spec:         e.spec,
		ChangedPaths: changedPaths,
		StripPrefix:  e.stripPrefix,
	})
	if err != nil {
		return model.ReadingList{}, err
	}
	return resolve.BuildReadingListWithAcks(e.repo, baseRef, claims, acks, ev, "HEAD")
}

func render(w io.Writer, rl model.ReadingList) {
	s := rl.Summary()
	commit := rl.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}

	fmt.Fprintf(w, "tree     %s\n", rl.Tree)
	fmt.Fprintf(w, "commit   %s\n", commit)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%d changed lines, %d need a human (%s%% collapsed)\n",
		s.ChangedLines, s.MustReadLines, trimFloat(s.ReductionPct))
	fmt.Fprintf(w, "%d claims, %d unresolved, %d unclaimed lines\n",
		s.Claims, s.ClaimsNeedingHuman, s.UnclaimedLines)
	if s.AcknowledgedLines > 0 {
		fmt.Fprintf(w, "%d acknowledged lines (%.0f%% of accounted code waved through)\n",
			s.AcknowledgedLines, s.AcknowledgementRatio*100)
	}

	mustRead := rl.MustRead()
	if len(mustRead) == 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "reading list: empty")
		return
	}

	byClaim := map[string]model.ResolvedClaim{}
	for _, rc := range rl.Claims {
		byClaim[rc.Claim.ID] = rc
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "READING LIST")
	for _, hr := range mustRead {
		marker := "?"
		if hr.Verdict == model.VerdictUnclaimed {
			marker = "!"
		}
		fmt.Fprintf(w, "  %s %s:%d-%d  [%s]\n",
			marker, hr.Path, hr.StartLine, hr.EndLine, hr.Verdict)
		if hr.Verdict == model.VerdictUnclaimed {
			fmt.Fprintln(w, "      no claim over this span")
		}
		for _, cid := range hr.ClaimIDs {
			rc, ok := byClaim[cid]
			if !ok || !rc.NeedsHuman() {
				continue
			}
			fmt.Fprintf(w, "      %s: %s\n", rc.Claim.Kind, rc.Claim.Text)
			for _, d := range rc.CoverageDetail {
				fmt.Fprintf(w, "        - %s\n", d)
			}
		}
	}
}

// trimFloat prints 37.5 as "37.5" and 50.0 as "50.0", matching Python's repr of
// a rounded float in the summary line.
func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}

type annotation struct {
	Path            string `json:"path"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	AnnotationLevel string `json:"annotation_level"`
	Title           string `json:"title"`
	Message         string `json:"message"`
}

func annotations(rl model.ReadingList) []annotation {
	byClaim := map[string]model.ResolvedClaim{}
	for _, rc := range rl.Claims {
		byClaim[rc.Claim.ID] = rc
	}

	out := []annotation{}
	for _, hr := range rl.MustRead() {
		var message string
		if hr.Verdict == model.VerdictUnclaimed {
			message = "No claim recorded over this change. Read it."
		} else {
			var reasons []string
			for _, cid := range hr.ClaimIDs {
				rc, ok := byClaim[cid]
				if !ok || !rc.NeedsHuman() {
					continue
				}
				reasons = append(reasons, fmt.Sprintf("%s (%s)",
					rc.Claim.Text, strings.Join(rc.CoverageDetail, "; ")))
			}
			message = strings.Join(reasons, " | ")
			if message == "" {
				message = "Unresolved claim."
			}
		}
		out = append(out, annotation{
			Path:            hr.Path,
			StartLine:       hr.StartLine,
			EndLine:         hr.EndLine,
			AnnotationLevel: "warning",
			Title:           "pawl: " + string(hr.Verdict),
			Message:         message,
		})
	}
	return out
}

func cmdVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	e := &evidenceFlags{}
	e.register(fs)
	asJSON := fs.Bool("json", false, "Emit the reading list as JSON.")
	annotationsOut := fs.String("annotations", "", "Write GitHub check annotations JSON here.")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rl, err := resolveReadingList(e)
	if err != nil {
		return fail(err)
	}

	if *annotationsOut != "" {
		b, err := json.MarshalIndent(annotations(rl), "", "  ")
		if err != nil {
			return fail(err)
		}
		if err := os.WriteFile(*annotationsOut, b, 0o644); err != nil {
			return fail(err)
		}
	}

	if *asJSON {
		b, err := json.MarshalIndent(rl, "", "  ")
		if err != nil {
			return fail(err)
		}
		fmt.Println(string(b))
	} else {
		render(os.Stdout, rl)
	}
	return 0
}

// selfDigest returns the SHA-256 of the running binary, or "" when it cannot be
// determined.
//
// Hashing the executable at run time is the honest source: it describes the
// binary that actually produced this statement, and a build-time constant could
// not observe tampering. It can fail — an unreadable or deleted executable — and
// PAWL-011 AC3 says to omit the digest when it does rather than emit a
// placeholder, because a zero digest looks like an answer.
func selfDigest() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func cmdAttest(args []string, version string) int {
	fs := flag.NewFlagSet("attest", flag.ContinueOnError)
	e := &evidenceFlags{}
	e.register(fs)
	ticket := fs.String("ticket", "", "e.g. PROJ-1234")
	policyPack := fs.String("policy-pack", "", "Policy pack identifier.")
	out := fs.String("out", "", "Write the statement here instead of stdout.")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rl, err := resolveReadingList(e)
	if err != nil {
		return fail(err)
	}

	statement := attest.BuildStatement(rl, attest.Options{
		Repository: gitutil.RemoteURL(e.repo),
		Ticket:     *ticket,
		PolicyPack: *policyPack,
		Version:    version,
		Digest:     selfDigest(),
	})
	b, err := json.MarshalIndent(statement, "", "  ")
	if err != nil {
		return fail(err)
	}

	if *out != "" {
		if err := os.WriteFile(*out, b, 0o644); err != nil {
			return fail(err)
		}
		fmt.Printf("wrote %s\n", *out)
	} else {
		fmt.Println(string(b))
	}
	return 0
}

func cmdGate(args []string) int {
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	e := &evidenceFlags{}
	e.register(fs)
	asJSON := fs.Bool("json", false, "Emit the decision as JSON.")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rl, err := resolveReadingList(e)
	if err != nil {
		return fail(err)
	}
	p, err := policy.Load(e.repo)
	if err != nil {
		return fail(err)
	}
	decision := policy.Evaluate(rl, p)

	if *asJSON {
		b, err := json.MarshalIndent(decision, "", "  ")
		if err != nil {
			return fail(err)
		}
		fmt.Println(string(b))
	} else {
		render(os.Stdout, rl)
		fmt.Println()
		if decision.Allowed {
			fmt.Println("gate: pass")
		} else {
			fmt.Println("gate: FAIL")
			for _, v := range decision.Violations {
				fmt.Printf("  [%s] %s\n", v.Rule, v.Detail)
			}
		}
	}

	if decision.Allowed {
		return 0
	}
	return 1
}
