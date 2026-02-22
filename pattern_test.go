package saruta

import "testing"

func TestCompilePatternValid(t *testing.T) {
	tests := []struct {
		pattern string
		kinds   []segmentKind
	}{
		{pattern: "/", kinds: nil},
		{pattern: "/users", kinds: []segmentKind{segmentStatic}},
		{pattern: "/users/{id}", kinds: []segmentKind{segmentStatic, segmentParam}},
		{pattern: `/users/{id:\d+}`, kinds: []segmentKind{segmentStatic, segmentParam}},
		{pattern: `/api/{name:[0-9]+}.json`, kinds: []segmentKind{segmentStatic, segmentParam}},
		{pattern: `/assets/pre-{id:[0-9]+}-v1`, kinds: []segmentKind{segmentStatic, segmentParam}},
		{pattern: "/files/{path...}", kinds: []segmentKind{segmentStatic, segmentCatchAll}},
		{pattern: "/users/", kinds: []segmentKind{segmentStatic, segmentStatic}},
	}
	for _, tc := range tests {
		cp, err := compilePattern(tc.pattern)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.pattern, err)
		}
		if len(cp.segments) != len(tc.kinds) {
			t.Fatalf("%s: segment len = %d, want %d", tc.pattern, len(cp.segments), len(tc.kinds))
		}
		for i, want := range tc.kinds {
			if cp.segments[i].kind != want {
				t.Fatalf("%s[%d]: kind = %v, want %v", tc.pattern, i, cp.segments[i].kind, want)
			}
		}
	}
}

func TestCompilePatternInvalid(t *testing.T) {
	tests := []string{
		"",
		"users",
		"/files/{path...}/x",
		"/users/{",
		"/users/}",
		"/users/{}",
		"/users/{...}",
		"/users/{id:[0-9+}",
		"/users/{id:}",
		"/files/{path...:[0-9]+}",
		"/api/{id:[0-9]+}{x}",
		"/api/x{id...}.json",
	}
	for _, pattern := range tests {
		if _, err := compilePattern(pattern); err == nil {
			t.Fatalf("%s: expected error", pattern)
		}
	}
}
