package pkg

import "testing"

func TestNormalizeCuisines(t *testing.T) {
	names, keys := NormalizeCuisines([]string{" North  Indian", "north indian", "", "Cafe", "  "})
	if len(names) != 2 || names[0] != "North Indian" || names[1] != "Cafe" {
		t.Fatalf("names = %v", names)
	}
	if len(keys) != 2 || keys[0] != "north indian" || keys[1] != "cafe" {
		t.Fatalf("keys = %v", keys)
	}
}

func TestNormalizeCuisines_KeepsFirstSpelling(t *testing.T) {
	names, _ := NormalizeCuisines([]string{"CAFE", "cafe"})
	if len(names) != 1 || names[0] != "CAFE" {
		t.Fatalf("names = %v", names)
	}
}

func TestNormalizeCuisines_EmptyIsNotNil(t *testing.T) {
	for _, in := range [][]string{nil, {}, {" ", ""}} {
		n, k := NormalizeCuisines(in)
		if n == nil || k == nil || len(n) != 0 || len(k) != 0 {
			t.Errorf("%v: want empty non-nil slices, got %#v %#v", in, n, k)
		}
	}
}

func TestNormalizeCurrency(t *testing.T) {
	for in, want := range map[string]string{
		"Indian Rupees(Rs.)": "INR",
		" inr ":              "INR",
		"usd":                "USD",
		"":                   "",
	} {
		if got := NormalizeCurrency(in); got != want {
			t.Errorf("NormalizeCurrency(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewGeoPoint_IsLngLat(t *testing.T) {
	p := NewGeoPoint(28.4, 77.0)
	if p.Type != "Point" || len(p.Coordinates) != 2 || p.Coordinates[0] != 77.0 || p.Coordinates[1] != 28.4 {
		t.Fatalf("%+v", p)
	}
}
