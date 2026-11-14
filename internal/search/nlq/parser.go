package nlq

import (
	"regexp"
	"strconv"
	"strings"
)

const defaultMinVotes = 50 // so "best" is not won by a 5.0 with two votes

// Parse reads a natural language query into an Intent. It never fails: whatever it cannot
// explain is left in Text (acted on as a name/cuisine search) or Unmatched (ignored).
//
// Each step removes the words it understood, so later steps (and the leftover Text) only
// see what is still unexplained. Order matters: price and rating first, so their numbers
// are not mistaken for text; cuisines before dishes, so "south indian dosa" yields one
// cuisine.
func Parse(q string, v Vocabulary) Intent {
	var in Intent
	s := normalize(q)

	// price: "under rs 1000 for two"
	if n, rest, ok := takeMaxCost(s); ok {
		in.MaxCost, in.Currency, s = &n, "INR", rest
	}
	s = forTwoRe.ReplaceAllString(s, " ")

	// rating: "above 4", "4+ stars"
	if r, rest, ok := takeMinRating(s); ok {
		in.MinRating, s = &r, rest
	}

	// intent words
	if bestRe.MatchString(s) {
		votes := defaultMinVotes
		in.Best, in.MinVotes = true, &votes
		s = bestRe.ReplaceAllString(s, " ")
	}
	if nearestRe.MatchString(s) {
		in.Nearest = true
		s = nearestRe.ReplaceAllString(s, " ")
	}

	// city: first (longest) known city found in the text
	for _, c := range v.Cities {
		if rest, ok := removeTerm(s, c, false); ok {
			in.City, s = c, rest
			break
		}
	}

	// cuisines: every known cuisine mentioned ("cafes" matches "cafe")
	for _, c := range v.Cuisines {
		if rest, ok := removeTerm(s, c, true); ok {
			in.Cuisines, s = appendUnique(in.Cuisines, c), rest
		}
	}

	// dishes: only when their cuisine really exists in the data, else they stay as text
	known := make(map[string]bool, len(v.Cuisines))
	for _, c := range v.Cuisines {
		known[c] = true
	}
	for _, dish := range dishPhrases {
		cuisine := dishCuisine[dish]
		if !known[cuisine] {
			continue
		}
		if rest, ok := removeTerm(s, dish, false); ok {
			in.Cuisines, s = appendUnique(in.Cuisines, cuisine), rest
		}
	}

	// leftovers
	var text []string
	for _, w := range strings.Fields(s) {
		switch {
		case isStopword(w):
		case isSoftWord(w):
			in.Unmatched = append(in.Unmatched, w)
		default:
			text = append(text, w)
		}
	}
	in.Text = strings.Join(text, " ")
	return in
}

// takeMaxCost finds "under rs 1000"-style phrases, skipping "under 4 stars".
func takeMaxCost(s string) (n int, rest string, ok bool) {
	for _, m := range maxCostRe.FindAllStringSubmatchIndex(s, -1) {
		if strings.HasPrefix(s[m[1]:], " star") {
			continue
		}
		v, err := strconv.Atoi(s[m[2]:m[3]])
		if err != nil {
			continue
		}
		return v, s[:m[0]] + " " + s[m[1]:], true
	}
	return 0, s, false
}

// takeMinRating finds "above 4" or "4+ stars"; only values in (0, 5] count.
func takeMinRating(s string) (r float64, rest string, ok bool) {
	for _, re := range []*regexp.Regexp{ratingAboveRe, ratingPlusRe} {
		for _, m := range re.FindAllStringSubmatchIndex(s, -1) {
			v, err := strconv.ParseFloat(s[m[2]:m[3]], 64)
			if err != nil || v <= 0 || v > 5 {
				continue
			}
			return v, s[:m[0]] + " " + s[m[1]:], true
		}
	}
	return 0, s, false
}

// removeTerm removes the first whole-word occurrence of term from s. With plural set it
// also accepts "terms"/"termes" ("cafes" for "cafe").
func removeTerm(s, term string, plural bool) (string, bool) {
	padded := " " + s + " "
	candidates := []string{term}
	if plural {
		candidates = append(candidates, term+"s", term+"es")
	}
	for _, c := range candidates {
		if i := strings.Index(padded, " "+c+" "); i >= 0 {
			rest := padded[:i] + " " + padded[i+len(c)+2:]
			return strings.Join(strings.Fields(rest), " "), true
		}
	}
	return s, false
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}
