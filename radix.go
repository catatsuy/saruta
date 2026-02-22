package saruta

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type node struct {
	staticChildren map[string]*node
	paramChild     *paramEdge
	catchAllChild  *paramEdge

	handlers map[string]http.Handler
	mount    http.Handler
}

type paramEdge struct {
	name    string
	expr    string
	prefix  string
	suffix  string
	matcher segmentMatcher
	next    *node
}

type pathParam struct {
	name  string
	value string
}

type routeMatch struct {
	leaf       *radixNode
	params     [8]pathParam
	paramCount int
}

type radixNode struct {
	staticEdges     []radixStaticEdge
	staticEdgeIndex [256]uint16 // index+1; 0 means none
	paramChild      *radixParamEdge
	catchAllChild   *radixParamEdge
	handlers        map[string]http.Handler
	mount           http.Handler
}

type radixStaticEdge struct {
	label string
	next  *radixNode
}

type radixParamEdge struct {
	name    string
	prefix  string
	suffix  string
	matcher segmentMatcher
	next    *radixNode
}

func newNode() *node {
	return &node{
		staticChildren: make(map[string]*node),
	}
}

func (n *node) insertRoute(method, pattern string, cp compiledPattern, h http.Handler) error {
	cur := n
	for _, seg := range cp.segments {
		switch seg.kind {
		case segmentStatic:
			next := cur.staticChildren[seg.literal]
			if next == nil {
				next = newNode()
				cur.staticChildren[seg.literal] = next
			}
			cur = next
		case segmentParam:
			if cur.paramChild == nil {
				cur.paramChild = &paramEdge{
					name:    seg.name,
					expr:    seg.expr,
					prefix:  seg.prefix,
					suffix:  seg.suffix,
					matcher: seg.matcher,
					next:    newNode(),
				}
			} else if cur.paramChild.name != seg.name || cur.paramChild.expr != seg.expr || cur.paramChild.prefix != seg.prefix || cur.paramChild.suffix != seg.suffix {
				return fmt.Errorf("route conflict: %s %s conflicts with existing parameter {%s}", method, pattern, cur.paramChild.name)
			}
			cur = cur.paramChild.next
		case segmentCatchAll:
			if cur.catchAllChild == nil {
				cur.catchAllChild = &paramEdge{
					name:    seg.name,
					expr:    seg.expr,
					matcher: seg.matcher,
					next:    newNode(),
				}
			} else if cur.catchAllChild.name != seg.name {
				return fmt.Errorf("route conflict: %s %s conflicts with existing catch-all {%s...}", method, pattern, cur.catchAllChild.name)
			}
			cur = cur.catchAllChild.next
		default:
			return fmt.Errorf("unknown segment kind")
		}
	}
	if cur.handlers == nil {
		cur.handlers = make(map[string]http.Handler)
	}
	if _, exists := cur.handlers[method]; exists {
		return fmt.Errorf("duplicate route: %s %s", method, pattern)
	}
	cur.handlers[method] = h
	return nil
}

func (n *node) insertMount(prefix string, cp compiledPattern, h http.Handler) error {
	cur := n
	for _, seg := range cp.segments {
		if seg.kind != segmentStatic {
			return fmt.Errorf("invalid mount prefix %q: prefix must be a static path", prefix)
		}
		next := cur.staticChildren[seg.literal]
		if next == nil {
			next = newNode()
			cur.staticChildren[seg.literal] = next
		}
		cur = next
	}
	if cur.mount != nil {
		return fmt.Errorf("duplicate mount: %s", prefix)
	}
	cur.mount = h
	return nil
}

func storeParam(params *[8]pathParam, count int, p pathParam) (int, bool) {
	if count >= len(params) {
		return count, false
	}
	params[count] = p
	return count + 1, true
}

func (pe *paramEdge) matchSegment(seg string) (string, bool) {
	return matchParamSegment(seg, pe.prefix, pe.suffix, pe.matcher)
}

func (pe *radixParamEdge) matchSegment(seg string) (string, bool) {
	return matchParamSegment(seg, pe.prefix, pe.suffix, pe.matcher)
}

func matchParamSegment(seg, prefix, suffix string, matcher segmentMatcher) (string, bool) {
	if len(seg) < len(prefix)+len(suffix) {
		return "", false
	}
	if prefix != "" && !strings.HasPrefix(seg, prefix) {
		return "", false
	}
	if suffix != "" && !strings.HasSuffix(seg, suffix) {
		return "", false
	}
	valueEnd := len(seg) - len(suffix)
	value := seg[len(prefix):valueEnd]
	if matcher != nil && !matcher.Match(value) {
		return "", false
	}
	return value, true
}

func allowHeaderValue(handlers map[string]http.Handler) string {
	if len(handlers) == 0 {
		return ""
	}
	methods := make([]string, 0, len(handlers))
	for method := range handlers {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return strings.Join(methods, ", ")
}

func buildRadix(root *node) *radixNode {
	if root == nil {
		return &radixNode{}
	}
	rt := buildRadixNode(root)
	finalizeRadix(rt)
	return rt
}

func buildRadixNode(src *node) *radixNode {
	dst := &radixNode{
		handlers: src.handlers,
		mount:    src.mount,
	}
	if src.paramChild != nil {
		dst.paramChild = &radixParamEdge{
			name:    src.paramChild.name,
			prefix:  src.paramChild.prefix,
			suffix:  src.paramChild.suffix,
			matcher: src.paramChild.matcher,
			next:    buildRadixNode(src.paramChild.next),
		}
	}
	if src.catchAllChild != nil {
		dst.catchAllChild = &radixParamEdge{
			name:    src.catchAllChild.name,
			matcher: src.catchAllChild.matcher,
			next:    buildRadixNode(src.catchAllChild.next),
		}
	}

	if len(src.staticChildren) == 0 {
		return dst
	}
	for seg, child := range src.staticChildren {
		label, end := compressStaticChain(seg, child)
		insertRadixStaticEdge(dst, label, buildRadixNode(end))
	}
	return dst
}

func compressStaticChain(firstSeg string, child *node) (string, *node) {
	label := "/" + firstSeg
	cur := child
	for {
		if cur == nil || cur.handlers != nil || cur.mount != nil || cur.paramChild != nil || cur.catchAllChild != nil || len(cur.staticChildren) != 1 {
			return label, cur
		}
		var nextSeg string
		var nextNode *node
		for s, n := range cur.staticChildren {
			nextSeg, nextNode = s, n
			break
		}
		label += "/" + nextSeg
		cur = nextNode
	}
}

func (n *radixNode) matchRoute(path string) (routeMatch, bool) {
	var params [8]pathParam
	if path == "/" {
		return routeMatch{leaf: n, params: params, paramCount: 0}, true
	}
	leaf, count, ok := n.matchPath(path, 0, &params, 0)
	if !ok {
		return routeMatch{}, false
	}
	return routeMatch{leaf: leaf, params: params, paramCount: count}, true
}

func (n *radixNode) matchPath(path string, pos int, params *[8]pathParam, paramCount int) (*radixNode, int, bool) {
	if pos == len(path) {
		return n, paramCount, true
	}

	if pos < len(path) {
		if edge := n.staticEdgeFor(path[pos]); edge != nil && strings.HasPrefix(path[pos:], edge.label) {
			if leaf, count, ok := edge.next.matchPath(path, pos+len(edge.label), params, paramCount); ok {
				return leaf, count, true
			}
		}
	}

	if pe := n.paramChild; pe != nil {
		if seg, nextPos, ok := nextSegmentAt(path, pos); ok {
			if value, ok := pe.matchSegment(seg); ok {
				nextCount, ok := storeParam(params, paramCount, pathParam{name: pe.name, value: value})
				if ok {
					if leaf, count, ok := pe.next.matchPath(path, nextPos, params, nextCount); ok {
						return leaf, count, true
					}
				}
			}
		}
	}

	if pe := n.catchAllChild; pe != nil {
		if rest, ok := catchAllAt(path, pos); ok {
			if value, ok := pe.matchSegment(rest); ok {
				nextCount, ok := storeParam(params, paramCount, pathParam{name: pe.name, value: value})
				if ok {
					return pe.next, nextCount, true
				}
			}
		}
	}

	return nil, 0, false
}

func nextSegmentAt(path string, pos int) (seg string, nextPos int, ok bool) {
	if pos >= len(path) || path[pos] != '/' {
		return "", 0, false
	}
	start := pos + 1
	for i := start; i < len(path); i++ {
		if path[i] == '/' {
			return path[start:i], i, true
		}
	}
	return path[start:], len(path), true
}

func catchAllAt(path string, pos int) (string, bool) {
	if pos >= len(path) || path[pos] != '/' {
		return "", false
	}
	return path[pos+1:], true
}

func (n *radixNode) findMount(path string) http.Handler {
	cur := n
	pos := 0
	var candidate http.Handler
	if cur.mount != nil {
		candidate = cur.mount
	}
	for {
		if pos == len(path) {
			return candidate
		}
		edge := cur.staticEdgeFor(path[pos])
		if edge == nil || !strings.HasPrefix(path[pos:], edge.label) {
			return candidate
		}
		cur = edge.next
		pos += len(edge.label)
		if cur.mount != nil && (pos == len(path) || (pos < len(path) && path[pos] == '/')) {
			candidate = cur.mount
		}
	}
}

func insertRadixStaticEdge(n *radixNode, label string, child *radixNode) {
	for i := range n.staticEdges {
		existing := &n.staticEdges[i]
		common := longestCommonPrefix(existing.label, label)
		if common == 0 {
			continue
		}
		if common == len(existing.label) && common == len(label) {
			existing.next = mergeRadixSubtree(existing.next, child)
			return
		}
		if common == len(existing.label) {
			insertRadixStaticEdge(existing.next, label[common:], child)
			return
		}
		// Split existing edge.
		split := &radixNode{}
		remainingExisting := existing.label[common:]
		remainingNew := label[common:]
		split.staticEdges = append(split.staticEdges, radixStaticEdge{
			label: remainingExisting,
			next:  existing.next,
		})
		existing.label = existing.label[:common]
		existing.next = split
		if remainingNew == "" {
			split = mergeRadixSubtree(split, child)
			existing.next = split
			return
		}
		insertRadixStaticEdge(split, remainingNew, child)
		return
	}
	n.staticEdges = append(n.staticEdges, radixStaticEdge{label: label, next: child})
}

func mergeRadixSubtree(dst, src *radixNode) *radixNode {
	if dst == nil {
		return src
	}
	if src == nil {
		return dst
	}
	if dst.handlers == nil {
		dst.handlers = src.handlers
	}
	if dst.mount == nil {
		dst.mount = src.mount
	}
	if dst.paramChild == nil {
		dst.paramChild = src.paramChild
	}
	if dst.catchAllChild == nil {
		dst.catchAllChild = src.catchAllChild
	}
	for _, e := range src.staticEdges {
		insertRadixStaticEdge(dst, e.label, e.next)
	}
	return dst
}

func longestCommonPrefix(a, b string) int {
	n := min(len(b), len(a))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

func finalizeRadix(n *radixNode) {
	if n == nil {
		return
	}
	if len(n.staticEdges) > 1 {
		sort.Slice(n.staticEdges, func(i, j int) bool {
			return n.staticEdges[i].label < n.staticEdges[j].label
		})
	}
	for i := range n.staticEdges {
		edge := &n.staticEdges[i]
		if edge.label != "" {
			n.staticEdgeIndex[edge.label[0]] = uint16(i + 1)
		}
		finalizeRadix(edge.next)
	}
	if n.paramChild != nil {
		finalizeRadix(n.paramChild.next)
	}
	if n.catchAllChild != nil {
		finalizeRadix(n.catchAllChild.next)
	}
}

func (n *radixNode) staticEdgeFor(first byte) *radixStaticEdge {
	idx := n.staticEdgeIndex[first]
	if idx == 0 {
		return nil
	}
	return &n.staticEdges[int(idx)-1]
}
