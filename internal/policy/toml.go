package policy

// A deliberately small TOML reader, covering exactly what .pawl/policy.toml
// uses: comments, one level of [table], and scalar or string-array values.
//
// This file is the honest cost of the port. Python 3.11 has tomllib in the
// standard library, so the original needed none of it; Go does not, and the
// alternatives were a third-party module or this. It is here rather than in
// go.mod because a zero-dependency build is worth more to C-6 than a complete
// TOML implementation is — the policy file is one we ship the template for.
//
// What it does NOT support, and will reject or ignore rather than silently
// misread: nested tables, arrays of tables, inline tables, multi-line strings,
// dates, and heterogeneous arrays. If a client's policy file ever needs those,
// replace this with github.com/BurntSushi/toml and take the dependency.

import (
	"fmt"
	"strconv"
	"strings"
)

// parseTOML returns table name -> key -> value. The root table is keyed "".
// Values are int64, float64, bool, string, or []string.
func parseTOML(src string) (map[string]map[string]any, error) {
	tables := map[string]map[string]any{"": {}}
	current := ""

	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") {
			end := strings.Index(line, "]")
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated table header", i+1)
			}
			current = strings.TrimSpace(line[1:end])
			if _, ok := tables[current]; !ok {
				tables[current] = map[string]any{}
			}
			continue
		}

		eq := strings.Index(line, "=")
		if eq < 0 {
			return nil, fmt.Errorf("line %d: expected key = value", i+1)
		}
		key := strings.TrimSpace(line[:eq])
		rest := strings.TrimSpace(line[eq+1:])

		// An array may span lines; accumulate until the brackets balance.
		if strings.HasPrefix(rest, "[") {
			buf := rest
			for strings.Count(buf, "[") > strings.Count(buf, "]") {
				i++
				if i >= len(lines) {
					return nil, fmt.Errorf("line %d: unterminated array", i)
				}
				buf += " " + strings.TrimSpace(lines[i])
			}
			values, err := parseStringArray(buf)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			tables[current][key] = values
			continue
		}

		value, err := parseScalar(stripComment(rest))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		tables[current][key] = value
	}
	return tables, nil
}

// stripComment removes a trailing # comment, respecting a quoted string.
func stripComment(s string) string {
	inString := false
	for i, r := range s {
		switch r {
		case '"':
			inString = !inString
		case '#':
			if !inString {
				return strings.TrimSpace(s[:i])
			}
		}
	}
	return strings.TrimSpace(s)
}

func parseScalar(s string) (any, error) {
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) && len(s) >= 2 {
		return strings.Trim(s, `"`), nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}
	return nil, fmt.Errorf("unsupported value %q", s)
}

func parseStringArray(s string) ([]string, error) {
	open := strings.Index(s, "[")
	shut := strings.LastIndex(s, "]")
	if open < 0 || shut < open {
		return nil, fmt.Errorf("malformed array %q", s)
	}
	body := s[open+1 : shut]

	var out []string
	for _, part := range strings.Split(body, ",") {
		part = stripComment(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		if !strings.HasPrefix(part, `"`) || !strings.HasSuffix(part, `"`) {
			return nil, fmt.Errorf("array elements must be quoted strings, got %q", part)
		}
		out = append(out, strings.Trim(part, `"`))
	}
	return out, nil
}
