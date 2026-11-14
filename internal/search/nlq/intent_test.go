package nlq

import (
	"reflect"
	"testing"
)

func TestNewVocabulary_LongestFirstDedupedLowercase(t *testing.T) {
	v := NewVocabulary([]string{"Delhi", "New  Delhi", "delhi", " ", "Gurgaon"}, []string{"Indian", "South Indian", "indian"})
	if want := []string{"new delhi", "gurgaon", "delhi"}; !reflect.DeepEqual(v.Cities, want) {
		t.Errorf("cities = %v, want %v", v.Cities, want)
	}
	if want := []string{"south indian", "indian"}; !reflect.DeepEqual(v.Cuisines, want) {
		t.Errorf("cuisines = %v, want %v", v.Cuisines, want)
	}
}

func TestNewVocabulary_Empty(t *testing.T) {
	v := NewVocabulary(nil, nil)
	if v.Cities == nil || v.Cuisines == nil || len(v.Cities)+len(v.Cuisines) != 0 {
		t.Errorf("want empty non-nil slices, got %#v", v)
	}
}

func TestIntent_IsZero(t *testing.T) {
	if !(Intent{}).IsZero() {
		t.Error("zero intent should be zero")
	}
	cost := 0
	for name, in := range map[string]Intent{
		"city": {City: "x"}, "best": {Best: true}, "nearest": {Nearest: true},
		"zero cost still counts": {MaxCost: &cost}, "text": {Text: "x"}, "unmatched": {Unmatched: []string{"x"}},
	} {
		if in.IsZero() {
			t.Errorf("%s: should not be zero", name)
		}
	}
}
