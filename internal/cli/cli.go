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
	"strconv"
	"strings"

	"trunion.io/pawl/internal/attest"
	"trunion.io/pawl/internal/claimlog"
	"trunion.io/pawl/internal/evidence"
	"trunion.io/pawl/internal/gitutil"
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
		path       = fs.String("path", "", "File the claim is about.")
		lines      = fs.String("lines", "", "Line range, e.g. 40-58.")
		kind       = fs.String("kind", string(model.KindAssumption), "Claim kind.")
		role       = fs.String("role", string(model.RoleAgent), "Author role.")
		harness    = fs.String("harness", "", "e.g. claude-code")
		modelName  = fs.String("model", "", "Model identifier.")
		identity   = fs.String("identity", "", "Human identity for expert/client roles.")
		session    = fs.String("session", "", "Session identifier.")
		ticket     = fs.String("ticket", "", "e.g. PROJ-1234")
		repo       = fs.String("repo", ".", "Repository root.")
		verifiedBy stringSlice
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
	var text string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		text, args = args[0], args[1:]
	}
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
			Harness:  *harness,
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
		path     = fs.String("path", "", "File the acknowledgement is about.")
		lines    = fs.String("lines", "", "Line range, e.g. 40-58.")
		role     = fs.String("role", string(model.RoleAgent), "Author role.")
		harness  = fs.String("harness", "", "e.g. claude-code")
		modelStr = fs.String("model", "", "Model identifier.")
		identity = fs.String("identity", "", "Human identity for expert/client roles.")
		session  = fs.String("session", "", "Session identifier.")
		repo     = fs.String("repo", ".", "Repository root.")
	)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: pawl ack --path <file> --lines <a-b> [options]")
		fmt.Fprintln(fs.Output(), "\nRecords that a changed span carries nothing to assume.")
		fmt.Fprintln(fs.Output(), "It is not a claim, and it does not clear a span on evidence —")
		fmt.Fprintln(fs.Output(), "acknowledged spans are sampled for review.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
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
			Harness:  *harness,
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
