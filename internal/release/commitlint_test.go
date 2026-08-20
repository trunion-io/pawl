package release

// PAWL-027 — the Go checker and commitlint must accept the same types.
//
// Two implementations of one rule is the drift risk the spec names. The local
// hook uses the Go parser because a Go developer should not need node_modules to
// write a commit; CI uses commitlint because that is what was asked for. This
// test is what makes the duplication safe: change one list and it fails until
// the other matches.

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestGoTypesMatchCommitlintConfig(t *testing.T) {
	b, err := os.ReadFile("../../commitlint.config.js")
	if err != nil {
		t.Fatalf("cannot read commitlint config: %v", err)
	}

	// Pull the type-enum array out of the config.
	re := regexp.MustCompile(`(?s)'type-enum':\s*\[\s*2,\s*'always',\s*\[(.*?)\]`)
	m := re.FindSubmatch(b)
	if m == nil {
		t.Fatal("could not find a type-enum rule in commitlint.config.js; if the rule moved, this test must move with it")
	}

	var fromJS []string
	for _, tok := range strings.Split(string(m[1]), ",") {
		tok = strings.TrimSpace(strings.Trim(strings.TrimSpace(tok), "'\""))
		if tok != "" {
			fromJS = append(fromJS, tok)
		}
	}

	fromGo := append([]string(nil), Types()...)
	sort.Strings(fromGo)
	sort.Strings(fromJS)

	if strings.Join(fromGo, ",") != strings.Join(fromJS, ",") {
		t.Errorf("commit types disagree:\n  go:         %v\n  commitlint: %v", fromGo, fromJS)
	}
}

// Every type commitlint accepts must also be a type BumpFor has an opinion
// about, even if that opinion is "no bump". A type nobody mapped would silently
// contribute nothing to the version.
func TestEveryTypeIsKnown(t *testing.T) {
	for _, ty := range Types() {
		if !KnownType(ty) {
			t.Errorf("%q is offered as a type but KnownType rejects it", ty)
		}
	}
}
