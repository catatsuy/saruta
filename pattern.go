package saruta

import (
	"fmt"
	"strings"
)

type segmentMatcher interface {
	Match(seg string) bool
}

type compiledPattern struct {
	segments []segment
}

type segmentKind int

const (
	segmentStatic segmentKind = iota
	segmentParam
	segmentCatchAll
)

type segment struct {
	kind    segmentKind
	literal string
	name    string
	expr    string
	prefix  string
	suffix  string
	matcher segmentMatcher
	tmpl    *segmentTemplate
}

type segmentTemplate struct {
	literals []string
	params   []templateParam
}

type templateParam struct {
	name    string
	expr    string
	matcher segmentMatcher
}

type byteClassMatcher struct {
	allow  [256]bool
	minLen int
}

func (m *byteClassMatcher) Match(seg string) bool {
	if len(seg) < m.minLen {
		return false
	}
	for i := 0; i < len(seg); i++ {
		if !m.allow[seg[i]] {
			return false
		}
	}
	return true
}

func compilePattern(pattern string) (compiledPattern, error) {
	if pattern == "" {
		return compiledPattern{}, fmt.Errorf("invalid pattern: empty pattern")
	}
	if pattern[0] != '/' {
		return compiledPattern{}, fmt.Errorf("invalid pattern: must start with '/'")
	}
	if pattern == "/" {
		return compiledPattern{}, nil
	}

	rawSegs := splitPathSegments(pattern)
	segments := make([]segment, 0, len(rawSegs))
	for i, raw := range rawSegs {
		seg, err := parseSegment(raw)
		if err != nil {
			return compiledPattern{}, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if seg.kind == segmentCatchAll && i != len(rawSegs)-1 {
			return compiledPattern{}, fmt.Errorf("invalid pattern %q: catch-all must be the last segment", pattern)
		}
		segments = append(segments, seg)
	}

	return compiledPattern{segments: segments}, nil
}

func parseSegment(raw string) (segment, error) {
	if raw == "" {
		return segment{kind: segmentStatic, literal: ""}, nil
	}
	if !strings.Contains(raw, "{") && !strings.Contains(raw, "}") {
		return segment{kind: segmentStatic, literal: raw}, nil
	}

	var literals []string
	var params []templateParam
	last := 0
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '}':
			return segment{}, fmt.Errorf("invalid segment syntax %q", raw)
		case '{':
			j := strings.IndexByte(raw[i+1:], '}')
			if j < 0 {
				return segment{}, fmt.Errorf("invalid segment syntax %q", raw)
			}
			j = i + 1 + j
			if strings.Contains(raw[i+1:j], "{") {
				return segment{}, fmt.Errorf("invalid segment syntax %q", raw)
			}
			literals = append(literals, raw[last:i])
			body := raw[i+1 : j]
			if body == "" {
				return segment{}, fmt.Errorf("empty parameter name")
			}
			if strings.HasSuffix(body, "...") {
				if len(params) > 0 || i != 0 || j != len(raw)-1 {
					return segment{}, fmt.Errorf("catch-all cannot have static prefix/suffix in segment")
				}
				return parseParamBody(body, "", "")
			}
			p, err := parseSegmentParam(body)
			if err != nil {
				return segment{}, err
			}
			params = append(params, p)
			i = j
			last = j + 1
		}
	}
	literals = append(literals, raw[last:])
	if len(params) == 0 {
		return segment{}, fmt.Errorf("invalid segment syntax %q", raw)
	}
	for i := 1; i < len(literals)-1; i++ {
		if literals[i] == "" {
			return segment{}, fmt.Errorf("adjacent parameters in one segment are not supported")
		}
	}
	tmpl := &segmentTemplate{literals: literals, params: params}
	seg := segment{
		kind: segmentParam,
		tmpl: tmpl,
	}
	if len(params) == 1 {
		seg.name = params[0].name
		seg.expr = params[0].expr
		seg.matcher = params[0].matcher
		seg.prefix = literals[0]
		seg.suffix = literals[1]
	}
	return seg, nil
}

func parseParamBody(body, prefix, suffix string) (segment, error) {
	if strings.HasSuffix(body, "...") {
		if prefix != "" || suffix != "" {
			return segment{}, fmt.Errorf("catch-all cannot have static prefix/suffix in segment")
		}
		if strings.Contains(body, ":") {
			return segment{}, fmt.Errorf("regex catch-all parameters are not supported yet")
		}
		name := strings.TrimSuffix(body, "...")
		if err := validateParamName(name); err != nil {
			return segment{}, err
		}
		return segment{kind: segmentCatchAll, name: name}, nil
	}

	name := body
	expr := ""
	var matcher segmentMatcher
	p, err := parseSegmentParam(body)
	if err != nil {
		return segment{}, err
	}
	name = p.name
	expr = p.expr
	matcher = p.matcher
	return segment{
		kind:    segmentParam,
		name:    name,
		expr:    expr,
		prefix:  prefix,
		suffix:  suffix,
		matcher: matcher,
		tmpl: &segmentTemplate{
			literals: []string{prefix, suffix},
			params:   []templateParam{p},
		},
	}, nil
}

func parseSegmentParam(body string) (templateParam, error) {
	name := body
	expr := ""
	var matcher segmentMatcher
	if before, after, ok := strings.Cut(body, ":"); ok {
		name = before
		expr = after
		if expr == "" {
			return templateParam{}, fmt.Errorf("empty parameter expression")
		}
		var err error
		matcher, err = compileSegmentExpr(expr)
		if err != nil {
			return templateParam{}, fmt.Errorf("invalid matcher for parameter %q: %w", name, err)
		}
	}
	if err := validateParamName(name); err != nil {
		return templateParam{}, err
	}
	return templateParam{name: name, expr: expr, matcher: matcher}, nil
}

func compileSegmentExpr(expr string) (segmentMatcher, error) {
	if expr == `\d` {
		return newByteClassMatcher([]byte("0123456789"), 1), nil
	}
	if after, ok := strings.CutPrefix(expr, `\d`); ok {
		switch after {
		case "+":
			return newByteClassMatcher([]byte("0123456789"), 1), nil
		case "*":
			return newByteClassMatcher([]byte("0123456789"), 0), nil
		}
	}

	if len(expr) < 2 || expr[0] != '[' {
		return nil, fmt.Errorf("unsupported expression %q", expr)
	}
	end := strings.IndexByte(expr, ']')
	if end <= 0 {
		return nil, fmt.Errorf("unterminated character class")
	}
	if end != len(expr)-1 && end != len(expr)-2 {
		return nil, fmt.Errorf("unsupported expression %q", expr)
	}

	minLen := 1
	if end == len(expr)-2 {
		switch expr[len(expr)-1] {
		case '+':
			minLen = 1
		case '*':
			minLen = 0
		default:
			return nil, fmt.Errorf("unsupported quantifier %q", string(expr[len(expr)-1]))
		}
	}

	classBytes, err := parseByteClass(expr[1:end])
	if err != nil {
		return nil, err
	}
	return newByteClassMatcher(classBytes, minLen), nil
}

func newByteClassMatcher(chars []byte, minLen int) *byteClassMatcher {
	m := &byteClassMatcher{minLen: minLen}
	for _, c := range chars {
		m.allow[c] = true
	}
	return m
}

func parseByteClass(class string) ([]byte, error) {
	if class == "" {
		return nil, fmt.Errorf("empty character class")
	}
	var out []byte
	for i := 0; i < len(class); {
		cur, next, err := readClassAtom(class, i)
		if err != nil {
			return nil, err
		}
		i = next
		if i+1 < len(class) && class[i] == '-' {
			endCh, endNext, err := readClassAtom(class, i+1)
			if err != nil {
				return nil, err
			}
			if cur > endCh {
				return nil, fmt.Errorf("invalid range %q-%q", string(cur), string(endCh))
			}
			for c := cur; c <= endCh; c++ {
				out = append(out, c)
			}
			i = endNext
			continue
		}
		out = append(out, cur)
	}
	return out, nil
}

func readClassAtom(s string, i int) (byte, int, error) {
	if i >= len(s) {
		return 0, i, fmt.Errorf("unexpected end of character class")
	}
	if s[i] != '\\' {
		return s[i], i + 1, nil
	}
	if i+1 >= len(s) {
		return 0, i, fmt.Errorf("dangling escape in character class")
	}
	switch s[i+1] {
	case 'd':
		return 0, i, fmt.Errorf(`\d is not supported inside character class`)
	default:
		return s[i+1], i + 2, nil
	}
}

func validateParamName(name string) error {
	if name == "" {
		return fmt.Errorf("empty parameter name")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (i > 0 && c >= '0' && c <= '9') {
			continue
		}
		return fmt.Errorf("invalid parameter name %q", name)
	}
	return nil
}

func splitPathSegments(path string) []string {
	if path == "/" {
		return nil
	}
	return strings.Split(path[1:], "/")
}
