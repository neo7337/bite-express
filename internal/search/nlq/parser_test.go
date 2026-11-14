package nlq

import (
	"reflect"
	"testing"
)

var testVocab = NewVocabulary(
	[]string{"Gurgaon", "Delhi", "New Delhi", "Mumbai"},
	[]string{"South Indian", "North Indian", "Indian", "Street Food", "Chinese", "Cafe", "Pizza", "Biryani"},
)

func TestParse_SpecExamples(t *testing.T) {
	t.Run("best in city with cuisine", func(t *testing.T) {
		in := Parse("Best restaurants in Gurgaon with South Indian Food", testVocab)
		if in.City != "gurgaon" || !reflect.DeepEqual(in.Cuisines, []string{"south indian"}) {
			t.Errorf("city/cuisine: %+v", in)
		}
		if !in.Best || in.MinVotes == nil || *in.MinVotes != defaultMinVotes {
			t.Errorf("best/minVotes: %+v", in)
		}
		if in.Text != "" || in.Nearest || in.MaxCost != nil {
			t.Errorf("nothing else should be set: %+v", in)
		}
	})
	t.Run("under price for two", func(t *testing.T) {
		in := Parse("Restaurants in Gurgaon under Rs. 1000 for two", testVocab)
		if in.City != "gurgaon" || in.MaxCost == nil || *in.MaxCost != 1000 || in.Currency != "INR" {
			t.Errorf("%+v", in)
		}
		if in.Text != "" || in.Best || len(in.Cuisines) != 0 {
			t.Errorf("nothing else should be set: %+v", in)
		}
	})
	t.Run("nearest dish", func(t *testing.T) {
		in := Parse("Nearest place that serves Vada Pav", testVocab)
		if !in.Nearest || !reflect.DeepEqual(in.Cuisines, []string{"street food"}) {
			t.Errorf("%+v", in)
		}
		if in.Text != "" || in.City != "" {
			t.Errorf("dish words must be consumed, not left as text: %+v", in)
		}
	})
}

func TestParse_Variants(t *testing.T) {
	cases := []struct {
		name  string
		q     string
		check func(Intent) bool
	}{
		{"rupee symbol", "cafes under ₹800", func(i Intent) bool {
			return i.MaxCost != nil && *i.MaxCost == 800 && reflect.DeepEqual(i.Cuisines, []string{"cafe"})
		}},
		{"indian digit grouping", "below Rs 1,00,000", func(i Intent) bool { return i.MaxCost != nil && *i.MaxCost == 100000 }},
		{"longest city wins", "pizza in new delhi", func(i Intent) bool { return i.City == "new delhi" && i.Text == "" }},
		{"plain delhi still works", "pizza in delhi", func(i Intent) bool { return i.City == "delhi" }},
		{"longest cuisine wins", "top rated north indian near me", func(i Intent) bool {
			return i.Best && i.Nearest && reflect.DeepEqual(i.Cuisines, []string{"north indian"})
		}},
		{"min rating", "rated above 4.5 pizza", func(i Intent) bool {
			return i.MinRating != nil && *i.MinRating == 4.5 && reflect.DeepEqual(i.Cuisines, []string{"pizza"})
		}},
		{"plus stars", "chinese 4+ stars", func(i Intent) bool { return i.MinRating != nil && *i.MinRating == 4 }},
		{"under stars is not a price", "under 4 stars", func(i Intent) bool { return i.MaxCost == nil }},
		{"absurd rating ignored", "above 1000 pizza", func(i Intent) bool { return i.MinRating == nil }},
		{"soft words reported, not filtered", "cheap cafes in gurgaon", func(i Intent) bool {
			return reflect.DeepEqual(i.Unmatched, []string{"cheap"}) && i.Text == "" && i.City == "gurgaon"
		}},
		{"dish maps to cuisine", "dosa in gurgaon", func(i Intent) bool { return reflect.DeepEqual(i.Cuisines, []string{"south indian"}) }},
		{"cuisine and dish dedupe", "south indian dosa", func(i Intent) bool { return reflect.DeepEqual(i.Cuisines, []string{"south indian"}) }},
		{"dish without known cuisine stays text", "momos in gurgaon", func(i Intent) bool { return i.Text == "momos" && len(i.Cuisines) == 0 }},
		{"unknown words become text", "dominos in gurgaon", func(i Intent) bool { return i.Text == "dominos" }},
		{"case and punctuation", "BEST Restaurants, in GURGAON!!", func(i Intent) bool { return i.Best && i.City == "gurgaon" && i.Text == "" }},
		{"two cuisines", "chinese and street food", func(i Intent) bool {
			return reflect.DeepEqual(i.Cuisines, []string{"street food", "chinese"}) || reflect.DeepEqual(i.Cuisines, []string{"chinese", "street food"})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if in := Parse(c.q, testVocab); !c.check(in) {
				t.Errorf("%q -> %+v", c.q, in)
			}
		})
	}
}

func TestParse_EmptyAndNoise(t *testing.T) {
	for _, q := range []string{"", "   ", "the restaurants in", "?!"} {
		if in := Parse(q, testVocab); !in.IsZero() {
			t.Errorf("%q should yield nothing, got %+v", q, in)
		}
	}
}

func TestParse_EmptyVocabularyStillParsesNumbers(t *testing.T) {
	in := Parse("best cafes under rs 500", NewVocabulary(nil, nil))
	if !in.Best || in.MaxCost == nil || *in.MaxCost != 500 || in.Text != "cafes" {
		t.Errorf("%+v", in)
	}
}
