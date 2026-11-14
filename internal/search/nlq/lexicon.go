package nlq

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// normalize lowercases the query and flattens punctuation so the regexes below can stay
// simple. After normalisation: "₹" and "Rs." become the word "rs", thousands separators
// are gone ("1,00,000" -> "100000"), decimals survive ("4.5"), "+" survives ("4+"),
// and every other symbol is a single space.
func normalize(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "₹", " rs ")
	in := []rune(s)
	out := make([]rune, 0, len(in))
	digit := func(i int) bool { return i >= 0 && i < len(in) && unicode.IsDigit(in[i]) }
	for i, r := range in {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+':
			out = append(out, r)
		case r == '.' && digit(i-1) && digit(i+1): // keep decimals
			out = append(out, r)
		case r == ',' && digit(i-1) && digit(i+1): // drop thousands separators
		default:
			out = append(out, ' ')
		}
	}
	return strings.Join(strings.Fields(string(out)), " ")
}

// ---- patterns (all run on normalize()d text) -----------------------------------------

var (
	// "under rs 1000", "below 500", "less than 800 rupees", "up to 750"
	maxCostRe = regexp.MustCompile(
		`\b(?:under|below|less than|lesser than|within|upto|up to|max|maximum|at most|not more than|budget)` +
			`\s+(?:of\s+)?(?:(?:rs|inr)\s*)?(\d+)(?:\s+(?:rs|rupees|inr))?\b`)

	// "for two", "per couple", "for 2": consumed so it is not left over as free text
	forTwoRe = regexp.MustCompile(`\b(?:for|per)\s+(?:two|2|couple|couples|two people|2 people)\b`)

	// "rated above 4", "at least 4.5 stars"
	ratingAboveRe = regexp.MustCompile(`\b(?:rated\s+)?(?:above|over|at least|atleast|minimum|min)\s+(\d(?:\.\d)?)(?:\s+stars?)?\b`)

	// "4+ stars", "4.5+"
	ratingPlusRe = regexp.MustCompile(`\b(\d(?:\.\d)?)\+(?:\s*stars?)?`)

	// "best", "top rated"; longer phrases first so "top rated" wins over "top"
	bestRe = regexp.MustCompile(`\b(?:highest rated|highly rated|top rated|best rated|well rated|best|top)\b`)

	// "nearest", "closest", "near me"
	nearestRe = regexp.MustCompile(`\b(?:nearest|closest|near me|nearby|near by|close to me|around me)\b`)
)

// ---- word lists ---------------------------------------------------------------------

func setOf(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// stopwords carry no filtering meaning and are dropped silently.
// "cafe" is deliberately absent: it is a cuisine in the data.
var stopwords = setOf(
	"a", "an", "the", "and", "or", "of", "in", "at", "on", "to", "for", "with",
	"that", "which", "who", "serves", "serve", "serving", "has", "have", "having", "offers", "offering",
	"find", "show", "me", "my", "give", "get", "list", "some", "any", "looking", "want", "need", "i", "is", "are",
	"restaurant", "restaurants", "place", "places", "spot", "spots", "joint", "joints",
	"eatery", "eateries", "outlet", "outlets", "food", "foods", "cuisine", "cuisines",
	"dish", "dishes", "eat", "eating", "dine", "dining", "near",
)

// softWords are qualifiers we cannot act on yet. They are reported back as unmatched
// terms but never become a free-text filter, which would wrongly empty the results.
var softWords = setOf(
	"cheap", "inexpensive", "affordable", "good", "great", "nice", "decent", "amazing", "awesome",
	"lovely", "famous", "popular", "tasty", "delicious", "authentic",
)

func isStopword(w string) bool { return stopwords[w] }
func isSoftWord(w string) bool { return softWords[w] }

// ---- dish -> cuisine ----------------------------------------------------------------

// dishCuisine maps a dish to the single cuisine most likely to list it. The dataset has
// no menu data, so this is a curated bridge. The parser only applies an entry when the
// target cuisine exists in the live Vocabulary; otherwise the words stay as free text.
var dishCuisine = map[string]string{
	"vada pav": "street food", "pav bhaji": "street food", "pani puri": "street food",
	"golgappa": "street food", "chaat": "street food", "bhel puri": "street food",
	"dosa": "south indian", "idli": "south indian", "medu vada": "south indian",
	"vada": "south indian", "uttapam": "south indian", "sambar": "south indian",
	"biryani": "biryani", "kebab": "mughlai", "kebabs": "mughlai", "korma": "mughlai",
	"butter chicken": "north indian", "dal makhani": "north indian", "paneer tikka": "north indian",
	"naan": "north indian", "chole bhature": "north indian", "paratha": "north indian",
	"noodles": "chinese", "manchurian": "chinese", "fried rice": "chinese", "momos": "momos",
	"pasta": "italian", "lasagna": "italian", "pizza": "pizza", "burger": "burger", "burgers": "burger",
	"cake": "bakery", "pastry": "bakery", "pastries": "bakery", "brownie": "desserts",
	"ice cream": "ice cream", "coffee": "cafe", "sandwich": "fast food", "sandwiches": "fast food",
}

// dishPhrases are the dishCuisine keys, longest first, so "vada pav" is tried before "vada"
// and "medu vada" before "vada".
var dishPhrases = func() []string {
	p := make([]string, 0, len(dishCuisine))
	for k := range dishCuisine {
		p = append(p, k)
	}
	sort.Slice(p, func(a, b int) bool {
		if len(p[a]) != len(p[b]) {
			return len(p[a]) > len(p[b])
		}
		return p[a] < p[b]
	})
	return p
}()
