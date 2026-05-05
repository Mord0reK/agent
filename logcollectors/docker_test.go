package logcollectors

import (
	"testing"
)

func TestDockerCollectorWildcardPatternDetection(t *testing.T) {
	tests := []struct {
		pattern string
		isWild  bool
	}{
		{"app", false},
		{"redis", false},
		{"app-*", true},
		{"app*", true},
		{"*-web", true},
		{"*", true},
		{"app-?", true},
	}

	for _, tt := range tests {
		result := isWildcard(tt.pattern)
		if result != tt.isWild {
			t.Errorf("isWildcard(%q): got %v, want %v", tt.pattern, result, tt.isWild)
		}
	}
}

func TestDockerCollectorExactMatch(t *testing.T) {
	// Test that exact match still works (backward compatibility)
	c := NewDockerCollector("testhost", "app")
	if c.pattern != "app" {
		t.Errorf("expected pattern 'app', got %q", c.pattern)
	}
	if isWildcard(c.pattern) {
		t.Error("'app' should not be detected as wildcard")
	}
}

func TestDockerCollectorWildcardPattern(t *testing.T) {
	// Test wildcard pattern
	c := NewDockerCollector("testhost", "app-*")
	if c.pattern != "app-*" {
		t.Errorf("expected pattern 'app-*', got %q", c.pattern)
	}
	if !isWildcard(c.pattern) {
		t.Error("'app-*' should be detected as wildcard")
	}
}
