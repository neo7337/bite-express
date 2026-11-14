package nlq

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Restaurants in Gurgaon under Rs. 1000 for two": "restaurants in gurgaon under rs 1000 for two",
		"under ₹1,000":          "under rs 1000",
		"under ₹800":            "under rs 800",
		"budget 1,00,000":       "budget 100000",
		"rated 4.5+ stars!":     "rated 4.5+ stars",
		"Vada-Pav":              "vada pav",
		"  MIXED   Case  ":      "mixed case",
		"rs.500":                "rs 500",
		"a, b":                  "a b",
		"version 1.2.3":         "version 1.2.3",
		"Nearest place, please": "nearest place please",
		"":                      "",
	} {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaxCostRe(t *testing.T) {
	for in, want := range map[string]string{
		"restaurants in gurgaon under rs 1000 for two": "1000",
		"below 500":              "500",
		"less than rs 800":       "800",
		"within 300 rs":          "300",
		"up to 750":              "750",
		"under rs1000":           "1000",
		"under 1000 rupees":      "1000",
		"budget 600":             "600",
		"cafes at most inr 450 ": "450",
	} {
		m := maxCostRe.FindStringSubmatch(in)
		if m == nil || m[1] != want {
			t.Errorf("%q: got %v, want %s", in, m, want)
		}
	}
	for _, in := range []string{"top 5 restaurants", "best cafes", "under the bridge", "rated above 4"} {
		if maxCostRe.MatchString(in) {
			t.Errorf("%q should not match", in)
		}
	}
}

func TestForTwoRe(t *testing.T) {
	for _, in := range []string{"under rs 1000 for two", "per couple", "for 2", "for two people"} {
		if !forTwoRe.MatchString(in) {
			t.Errorf("%q should match", in)
		}
	}
	if forTwoRe.MatchString("for twelve") {
		t.Error("for twelve should not match")
	}
}

func TestRatingRes(t *testing.T) {
	for in, want := range map[string]string{"rated above 4": "4", "at least 4.5 stars": "4.5", "over 3": "3", "min 2": "2"} {
		if m := ratingAboveRe.FindStringSubmatch(in); m == nil || m[1] != want {
			t.Errorf("above %q: got %v want %s", in, m, want)
		}
	}
	for in, want := range map[string]string{"4+ stars": "4", "4.5+": "4.5", "cafes 3+ in gurgaon": "3"} {
		if m := ratingPlusRe.FindStringSubmatch(in); m == nil || m[1] != want {
			t.Errorf("plus %q: got %v want %s", in, m, want)
		}
	}
	if ratingPlusRe.MatchString("a+ grade") {
		t.Error("a+ should not match")
	}
}

func TestBestAndNearestRe(t *testing.T) {
	if got := bestRe.FindString("top rated cafes"); got != "top rated" {
		t.Errorf("want the longer phrase, got %q", got)
	}
	for _, in := range []string{"best restaurants", "highly rated", "top places"} {
		if !bestRe.MatchString(in) {
			t.Errorf("%q should be a best query", in)
		}
	}
	if bestRe.MatchString("bestow") || bestRe.MatchString("stop") {
		t.Error("must match whole words only")
	}
	for _, in := range []string{"nearest place", "near me", "closest cafe", "nearby"} {
		if !nearestRe.MatchString(in) {
			t.Errorf("%q should be a nearest query", in)
		}
	}
}

func TestWordLists(t *testing.T) {
	for w := range stopwords {
		if softWords[w] {
			t.Errorf("%q is in both stopwords and softWords", w)
		}
	}
	if isStopword("cafe") {
		t.Error("cafe is a cuisine and must not be a stopword")
	}
	for _, w := range []string{"restaurants", "the", "serves"} {
		if !isStopword(w) {
			t.Errorf("%q should be a stopword", w)
		}
	}
	if !isSoftWord("cheap") || isSoftWord("pizza") {
		t.Error("soft word lookup wrong")
	}
}

func TestDishPhrases(t *testing.T) {
	if len(dishPhrases) != len(dishCuisine) {
		t.Fatalf("phrases %d != entries %d", len(dishPhrases), len(dishCuisine))
	}
	idx := map[string]int{}
	for i, p := range dishPhrases {
		idx[p] = i
		if i > 0 && len(dishPhrases[i-1]) < len(p) {
			t.Errorf("not longest-first at %q", p)
		}
	}
	if idx["vada pav"] > idx["vada"] || idx["medu vada"] > idx["vada"] {
		t.Error(`"vada pav" and "medu vada" must be tried before "vada"`)
	}
	for dish, cuisine := range dishCuisine {
		if dish != strings.ToLower(dish) || cuisine != strings.ToLower(cuisine) || cuisine == "" {
			t.Errorf("entry %q -> %q must be lowercase and non-empty", dish, cuisine)
		}
		if dish != normalize(dish) {
			t.Errorf("dish %q is not in normalised form (%q)", dish, normalize(dish))
		}
		if isStopword(dish) {
			t.Errorf("dish %q is also a stopword", dish)
		}
	}
	if dishCuisine["vada pav"] != "street food" || dishCuisine["dosa"] != "south indian" {
		t.Error("spec examples must map")
	}
}
