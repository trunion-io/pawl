package release

// PAWL-027 AC8 — a release candidate is tagged when the *full* check suite
// passes on trunk.
//
// rc.yml names the checks it waits for, and the ruleset names the checks that
// are actually required. Nothing connected the two, and they drifted: rc.yml
// listed four while the ruleset required seven, so a candidate could have been
// tagged with three checks — including both commit-message checks — still
// unreported. Worse, the drift was invisible because rc.yml also carried a
// second, unused copy of the list.
//
// The same fix as TestGoTypesMatchCommitlintConfig: hold the two together
// mechanically rather than by remembering.

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestRCChecksMatchRuleset(t *testing.T) {
	// The checks the ruleset requires.
	b, err := os.ReadFile("../../.github/_setup/ruleset-main.json")
	if err != nil {
		t.Fatalf("cannot read the ruleset: %v", err)
	}
	var rs struct {
		Rules []struct {
			Type       string `json:"type"`
			Parameters struct {
				RequiredStatusChecks []struct {
					Context string `json:"context"`
				} `json:"required_status_checks"`
			} `json:"parameters"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(b, &rs); err != nil {
		t.Fatalf("ruleset is not valid JSON: %v", err)
	}
	var fromRuleset []string
	for _, r := range rs.Rules {
		if r.Type == "required_status_checks" {
			for _, c := range r.Parameters.RequiredStatusChecks {
				fromRuleset = append(fromRuleset, c.Context)
			}
		}
	}
	if len(fromRuleset) == 0 {
		t.Fatal("no required_status_checks found in the ruleset; if the shape changed, this test must change with it")
	}

	// The checks rc.yml waits for, read from its NAMES heredoc.
	y, err := os.ReadFile("../../.github/workflows/rc.yml")
	if err != nil {
		t.Fatalf("cannot read rc.yml: %v", err)
	}
	m := regexp.MustCompile(`(?s)<<'NAMES'\n(.*?)\n\s*NAMES`).FindSubmatch(y)
	if m == nil {
		t.Fatal("could not find the NAMES heredoc in rc.yml; if it moved, this test must move with it")
	}
	var fromRC []string
	for _, line := range strings.Split(string(m[1]), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			fromRC = append(fromRC, s)
		}
	}

	sort.Strings(fromRuleset)
	sort.Strings(fromRC)
	if strings.Join(fromRuleset, "|") != strings.Join(fromRC, "|") {
		t.Errorf("rc.yml and the ruleset disagree about the required checks:\n  ruleset: %v\n  rc.yml:  %v",
			fromRuleset, fromRC)
	}
}
