package config

import (
	"fmt"
	"strconv"
	"strings"
)

type yamlLine struct {
	indent int
	text   string
	line   int
}

// parseYAML implements the deliberately small YAML subset used by SigWatch's
// MVP configuration: indented mappings, sequences, quoted/unquoted scalars,
// numbers, booleans, null, and comments. Anchors, aliases, flow collections,
// multiline scalars, and tags are intentionally rejected/not supported.
func parseYAML(input string) (map[string]any, error) {
	lines, err := lexYAML(input)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return map[string]any{}, nil
	}
	if strings.HasPrefix(lines[0].text, "- ") || lines[0].text == "-" {
		return nil, fmt.Errorf("line %d: top-level YAML must be a mapping", lines[0].line)
	}
	v, next, err := parseMap(lines, 0, lines[0].indent)
	if err != nil {
		return nil, err
	}
	if next != len(lines) {
		return nil, fmt.Errorf("line %d: could not parse YAML", lines[next].line)
	}
	return v, nil
}

func lexYAML(input string) ([]yamlLine, error) {
	var out []yamlLine
	for i, raw := range strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n") {
		lineNo := i + 1
		if strings.ContainsRune(raw, '\t') {
			return nil, fmt.Errorf("line %d: tabs are not allowed for YAML indentation", lineNo)
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		text, err := stripYAMLComment(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		text = strings.TrimSpace(text)
		if text == "" || text == "---" || text == "..." {
			continue
		}
		out = append(out, yamlLine{indent: indent, text: text, line: lineNo})
	}
	return out, nil
}

func stripYAMLComment(s string) (string, error) {
	var quote rune
	escaped := false
	for i, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '#' {
			if i == 0 || (i > 0 && (s[i-1] == ' ' || s[i-1] == '\t')) {
				return s[:i], nil
			}
		}
	}
	if quote != 0 {
		return "", fmt.Errorf("unterminated quoted scalar")
	}
	return s, nil
}

func parseBlock(lines []yamlLine, idx, indent int) (any, int, error) {
	if idx >= len(lines) {
		return nil, idx, nil
	}
	if lines[idx].indent != indent {
		return nil, idx, fmt.Errorf("line %d: unexpected indentation", lines[idx].line)
	}
	if lines[idx].text == "-" || strings.HasPrefix(lines[idx].text, "- ") {
		return parseList(lines, idx, indent)
	}
	return parseMap(lines, idx, indent)
}

func parseMap(lines []yamlLine, idx, indent int) (map[string]any, int, error) {
	m := map[string]any{}
	for idx < len(lines) {
		ln := lines[idx]
		if ln.indent < indent {
			break
		}
		if ln.indent > indent {
			return nil, idx, fmt.Errorf("line %d: unexpected indentation", ln.line)
		}
		if ln.text == "-" || strings.HasPrefix(ln.text, "- ") {
			break
		}
		key, raw, ok := splitKeyValue(ln.text)
		if !ok || key == "" {
			return nil, idx, fmt.Errorf("line %d: expected 'key: value'", ln.line)
		}
		if _, exists := m[key]; exists {
			return nil, idx, fmt.Errorf("line %d: duplicate key %q", ln.line, key)
		}
		idx++
		if raw != "" {
			v, err := parseScalar(raw)
			if err != nil {
				return nil, idx, fmt.Errorf("line %d: %w", ln.line, err)
			}
			m[key] = v
			continue
		}
		if idx >= len(lines) || lines[idx].indent <= indent {
			m[key] = nil
			continue
		}
		childIndent := lines[idx].indent
		v, next, err := parseBlock(lines, idx, childIndent)
		if err != nil {
			return nil, idx, err
		}
		m[key] = v
		idx = next
	}
	return m, idx, nil
}

func parseList(lines []yamlLine, idx, indent int) ([]any, int, error) {
	var out []any
	for idx < len(lines) {
		ln := lines[idx]
		if ln.indent < indent {
			break
		}
		if ln.indent != indent || !(ln.text == "-" || strings.HasPrefix(ln.text, "- ")) {
			break
		}
		rest := strings.TrimSpace(strings.TrimPrefix(ln.text, "-"))
		idx++
		if rest == "" {
			if idx >= len(lines) || lines[idx].indent <= indent {
				out = append(out, nil)
				continue
			}
			v, next, err := parseBlock(lines, idx, lines[idx].indent)
			if err != nil {
				return nil, idx, err
			}
			out = append(out, v)
			idx = next
			continue
		}

		if key, raw, ok := splitKeyValue(rest); ok {
			item := map[string]any{}
			if key == "" {
				return nil, idx, fmt.Errorf("line %d: list mapping key is empty", ln.line)
			}
			if raw != "" {
				v, err := parseScalar(raw)
				if err != nil {
					return nil, idx, fmt.Errorf("line %d: %w", ln.line, err)
				}
				item[key] = v
			} else if idx < len(lines) && lines[idx].indent > indent {
				childIndent := lines[idx].indent
				v, next, err := parseBlock(lines, idx, childIndent)
				if err != nil {
					return nil, idx, err
				}
				item[key] = v
				idx = next
			} else {
				item[key] = nil
			}

			// Additional keys belonging to this list item are one indentation
			// level deeper than the dash. Parse and merge them.
			if idx < len(lines) && lines[idx].indent > indent {
				moreIndent := lines[idx].indent
				if lines[idx].text == "-" || strings.HasPrefix(lines[idx].text, "- ") {
					return nil, idx, fmt.Errorf("line %d: unexpected nested list", lines[idx].line)
				}
				more, next, err := parseMap(lines, idx, moreIndent)
				if err != nil {
					return nil, idx, err
				}
				for k, v := range more {
					if _, exists := item[k]; exists {
						return nil, idx, fmt.Errorf("line %d: duplicate key %q in list item", lines[idx].line, k)
					}
					item[k] = v
				}
				idx = next
			}
			out = append(out, item)
			continue
		}

		v, err := parseScalar(rest)
		if err != nil {
			return nil, idx, fmt.Errorf("line %d: %w", ln.line, err)
		}
		out = append(out, v)
	}
	return out, idx, nil
}

func splitKeyValue(s string) (string, string, bool) {
	var quote rune
	escaped := false
	for i, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ':' {
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
		}
	}
	return "", "", false
}

func parseScalar(s string) (any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "{") || s == "|" || s == ">" || strings.Contains(s, " &") || strings.HasPrefix(s, "*") {
		return nil, fmt.Errorf("unsupported YAML feature in %q", s)
	}
	if s[0] == '"' {
		v, err := strconv.Unquote(s)
		if err != nil {
			return nil, fmt.Errorf("invalid quoted string: %w", err)
		}
		return v, nil
	}
	if s[0] == '\'' {
		if len(s) < 2 || s[len(s)-1] != '\'' {
			return nil, fmt.Errorf("invalid single-quoted string")
		}
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'"), nil
	}
	switch strings.ToLower(s) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null", "~":
		return nil, nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && strings.ContainsAny(s, ".eE") {
		return f, nil
	}
	return s, nil
}
