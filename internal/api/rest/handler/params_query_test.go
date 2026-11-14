package handler

import (
	"errors"
	"strings"
	"testing"
)

func TestParseQuery(t *testing.T) {
	for in, want := range map[string]string{
		"":                        "",
		"   ":                     "",
		"best cafes":              "best cafes",
		"  best    cafes \t in\n": "best cafes in",
	} {
		got, err := parseQuery(in)
		if err != nil || got != want {
			t.Errorf("parseQuery(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestParseQuery_LengthLimit(t *testing.T) {
	if _, err := parseQuery(strings.Repeat("a", maxQueryLen)); err != nil {
		t.Errorf("exactly %d characters should pass: %v", maxQueryLen, err)
	}
	if _, err := parseQuery(strings.Repeat("a", maxQueryLen+1)); !errors.Is(err, errBadParam) {
		t.Errorf("one over the limit should be errBadParam, got %v", err)
	}
	// the limit counts characters, not bytes
	if _, err := parseQuery(strings.Repeat("é", maxQueryLen)); err != nil {
		t.Errorf("multi-byte characters must be counted as one each: %v", err)
	}
}
