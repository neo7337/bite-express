package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/neo7337/bite-express/internal/repository"
	"github.com/neo7337/bite-express/internal/search/nlq"
)

func TestToSearchResponse_Shape(t *testing.T) {
	cost, votes, rating := 800, 50, 4.5
	b, _ := json.Marshal(toSearchResponse(repository.ListResult{HasMore: true}, nlq.Intent{
		City: "gurgaon", Cuisines: []string{"cafe"}, MaxCost: &cost, MinVotes: &votes, MinRating: &rating,
		Best: true, Nearest: true, Text: "pizza hut", Unmatched: []string{"cheap"},
	}))
	s := string(b)
	for _, want := range []string{
		`"items":[]`, `"hasMore":true`, `"city":"gurgaon"`, `"cuisines":["cafe"]`, `"maxCost":800`,
		`"minVotes":50`, `"minRating":4.5`, `"best":true`, `"nearest":true`, `"text":"pizza hut"`, `"unmatchedTerms":["cheap"]`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
}

func TestToSearchResponse_EmptyIntentIsAnEmptyObject(t *testing.T) {
	b, _ := json.Marshal(toSearchResponse(repository.ListResult{}, nlq.Intent{}))
	if string(b) != `{"items":[],"hasMore":false,"interpreted":{}}` {
		t.Fatalf("got %s", b)
	}
}
