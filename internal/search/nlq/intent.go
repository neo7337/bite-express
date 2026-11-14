package nlq

import (
	"sort"
	"strings"
)

type Intent struct {
	City      string
	Cuisines  []string
	MaxCost   *int
	Currency  string
	MinRating *float64
	MinVotes  *int
	Best      bool
	Nearest   bool
	Text      string
	Unmatched []string
}

// IsZero reports whether the query yielded no structure at all.
func (i Intent) IsZero() bool {
	return i.City == "" && len(i.Cuisines) == 0 && i.MaxCost == nil && i.Currency == "" &&
		i.MinRating == nil && i.MinVotes == nil && !i.Best && !i.Nearest &&
		i.Text == "" && len(i.Unmatched) == 0
}

// Vocabulary is the set of values that exist in the data, used to recognise cities and
// cuisines instead of guessing. Build it with NewVocabulary so matching is longest-first.
type Vocabulary struct {
	Cities   []string
	Cuisines []string
}

// NewVocabulary lowercases, trims and de-duplicates the terms and sorts them longest
// first, so "new delhi" is tried before "delhi" and "south indian" before "indian".
func NewVocabulary(cities, cuisines []string) Vocabulary {
	return Vocabulary{Cities: normalizeTerms(cities), Cuisines: normalizeTerms(cuisines)}
}

func normalizeTerms(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.Join(strings.Fields(strings.ToLower(t)), " ")
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	sort.SliceStable(out, func(a, b int) bool {
		if len(out[a]) != len(out[b]) {
			return len(out[a]) > len(out[b])
		}
		return out[a] < out[b]
	})
	return out
}
