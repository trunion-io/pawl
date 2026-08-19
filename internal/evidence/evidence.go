// Package evidence ingests the mechanical evidence a CI run already produces.
//
// Nothing here is novel and none of it should be. The point is to consume what
// the client's pipeline emits anyway — junit XML, Cobertura coverage, a
// typecheck report, an OPA decision log — and to refuse to take the agent's word
// for any of it. The agent asserts a check exists; this package decides whether
// it does.
package evidence

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"strings"
)

// Evidence is everything the verifier is allowed to treat as established fact.
type Evidence struct {
	// Tests maps test node id -> passed.
	Tests map[string]bool
	// CoveredLines maps path -> set of line numbers hit by the suite.
	CoveredLines map[string]map[int]bool
	// CleanTypecheck holds paths with no type errors.
	CleanTypecheck map[string]bool
	TypecheckRan   bool
	// PolicyResults maps rule name -> allowed.
	PolicyResults map[string]bool
	// SpecCriteria holds acceptance criterion ids present in a signed spec
	// attestation.
	SpecCriteria map[string]bool
}

func New() *Evidence {
	return &Evidence{
		Tests:          map[string]bool{},
		CoveredLines:   map[string]map[int]bool{},
		CleanTypecheck: map[string]bool{},
		PolicyResults:  map[string]bool{},
		SpecCriteria:   map[string]bool{},
	}
}

// TestPassed distinguishes absent from failed. The second return is false when
// the node id is not present in the evidence at all, which is not the same as a
// test that ran and failed — and neither of them clears.
func (e *Evidence) TestPassed(nodeID string) (passed bool, present bool) {
	p, ok := e.Tests[nodeID]
	return p, ok
}

func (e *Evidence) LinesCovered(path string, start, end int) bool {
	hit, ok := e.CoveredLines[path]
	if !ok || len(hit) == 0 {
		return false
	}
	for line := start; line <= end; line++ {
		if !hit[line] {
			return false
		}
	}
	return true
}

// findElements streams the document and decodes every element with the given
// local name, at any depth. Go's encoding/xml has no XPath, so this stands in
// for ElementTree's `.//name` — about twenty lines that the Python got from the
// standard library for free.
func findElements[T any](r io.Reader, name string) ([]T, error) {
	dec := xml.NewDecoder(r)
	var out []T
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != name {
			continue
		}
		var v T
		if err := dec.DecodeElement(&v, &se); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
}

type junitCase struct {
	Classname string    `xml:"classname,attr"`
	Name      string    `xml:"name,attr"`
	Failure   *struct{} `xml:"failure"`
	Error     *struct{} `xml:"error"`
	Skipped   *struct{} `xml:"skipped"`
}

// normaliseJUnitID handles pytest writing classname as a dotted module path.
// Emits both the dotted form and the pytest node id form so a claim can cite
// either.
func normaliseJUnitID(classname, name string) []string {
	if classname == "" {
		return []string{name}
	}
	ids := []string{classname + "." + name}
	parts := strings.Split(classname, ".")
	// tests.test_auth.TestThing -> tests/test_auth.py::TestThing::name
	for split := len(parts); split > 0; split-- {
		mod := strings.Join(parts[:split], "/")
		rest := strings.Join(parts[split:], "::")
		if rest != "" {
			ids = append(ids, mod+".py::"+rest+"::"+name)
		} else {
			ids = append(ids, mod+".py::"+name)
		}
	}
	return ids
}

func LoadJUnit(ev *Evidence, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	cases, err := findElements[junitCase](f, "testcase")
	if err != nil {
		return err
	}
	for _, c := range cases {
		if c.Skipped != nil {
			continue // a skipped test is not evidence of anything
		}
		failed := c.Failure != nil || c.Error != nil
		for _, id := range normaliseJUnitID(c.Classname, c.Name) {
			ev.Tests[id] = !failed
		}
	}
	return nil
}

type covClass struct {
	Filename string `xml:"filename,attr"`
	Lines    []struct {
		Number int `xml:"number,attr"`
		Hits   int `xml:"hits,attr"`
	} `xml:"lines>line"`
}

// LoadCoverageXML reads Cobertura format, as emitted by coverage.py and most JS
// tooling.
func LoadCoverageXML(ev *Evidence, path, stripPrefix string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	classes, err := findElements[covClass](f, "class")
	if err != nil {
		return err
	}
	for _, cls := range classes {
		filename := cls.Filename
		if stripPrefix != "" && strings.HasPrefix(filename, stripPrefix) {
			filename = strings.TrimLeft(filename[len(stripPrefix):], "/")
		}
		hits, ok := ev.CoveredLines[filename]
		if !ok {
			hits = map[int]bool{}
			ev.CoveredLines[filename] = hits
		}
		for _, line := range cls.Lines {
			if line.Hits > 0 {
				hits[line.Number] = true
			}
		}
	}
	return nil
}

// LoadTypecheck expects a JSON list of {"path": ..., "errors": n} or mypy
// --output=json lines. Files present in the changeset and absent from the error
// set are treated as clean.
func LoadTypecheck(ev *Evidence, path string, changedPaths map[string]bool) error {
	ev.TypecheckRan = true
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(b))
	errored := map[string]bool{}

	if strings.HasPrefix(text, "[") {
		var entries []map[string]any
		if err := json.Unmarshal([]byte(text), &entries); err != nil {
			return err
		}
		for _, entry := range entries {
			_, hasMessage := entry["message"]
			errCount, hasErrors := entry["errors"].(float64)
			if (hasErrors && errCount > 0) || (!hasErrors) || hasMessage {
				errored[firstString(entry, "path", "file")] = true
			}
		}
	} else {
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			errored[firstString(entry, "file", "path")] = true
		}
	}

	for p := range changedPaths {
		if !errored[p] {
			ev.CleanTypecheck[p] = true
		}
	}
	return nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

// LoadPolicy reads a flat JSON object of rule -> bool, or an OPA-style result
// document.
func LoadPolicy(ev *Evidence, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var data any
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	if obj, ok := data.(map[string]any); ok {
		if result, ok := obj["result"]; ok {
			data = result
		}
	}
	switch v := data.(type) {
	case []any:
		for _, item := range v {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			rule, ok := entry["rule"].(string)
			if !ok {
				continue
			}
			allowed, _ := entry["allowed"].(bool)
			ev.PolicyResults[rule] = allowed
		}
	case map[string]any:
		for rule, value := range v {
			b, _ := value.(bool)
			ev.PolicyResults[rule] = b
		}
	}
	return nil
}

// LoadSpec reads a signed spec attestation. Only criteria marked checkable count
// as evidence — an unverifiable criterion is a permanent tax on human attention
// and should never satisfy a claim mechanically.
func LoadSpec(ev *Evidence, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var data map[string]any
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	predicate := data
	if p, ok := data["predicate"].(map[string]any); ok {
		predicate = p
	}
	criteria, _ := predicate["criteria"].([]any)
	for _, c := range criteria {
		entry, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if checkable, _ := entry["checkable"].(bool); !checkable {
			continue
		}
		if id, ok := entry["id"].(string); ok {
			ev.SpecCriteria[id] = true
		}
	}
	return nil
}

// Sources names the evidence files a run was given.
type Sources struct {
	JUnit        []string
	Coverage     []string
	Typecheck    string
	Policy       string
	Spec         string
	ChangedPaths map[string]bool
	StripPrefix  string
}

func Collect(s Sources) (*Evidence, error) {
	ev := New()
	for _, p := range s.JUnit {
		if err := LoadJUnit(ev, p); err != nil {
			return nil, err
		}
	}
	for _, p := range s.Coverage {
		if err := LoadCoverageXML(ev, p, s.StripPrefix); err != nil {
			return nil, err
		}
	}
	if s.Typecheck != "" {
		if err := LoadTypecheck(ev, s.Typecheck, s.ChangedPaths); err != nil {
			return nil, err
		}
	}
	if s.Policy != "" {
		if err := LoadPolicy(ev, s.Policy); err != nil {
			return nil, err
		}
	}
	if s.Spec != "" {
		if err := LoadSpec(ev, s.Spec); err != nil {
			return nil, err
		}
	}
	return ev, nil
}
