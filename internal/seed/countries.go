package seed

// map the values from Country-code.xlsx
var countryNames = map[int]string{
	1:   "India",
	14:  "Australia",
	30:  "Brazil",
	37:  "Canada",
	94:  "Indonesia",
	148: "New Zealand",
	162: "Philippines",
	166: "Qatar",
	184: "Singapore",
	189: "South Africa",
	191: "Sri Lanka",
	208: "Turkey",
	214: "United Arab Emirates",
	215: "United Kingdom",
	216: "United States",
}

func countryName(code int) (string, bool) {
	name, ok := countryNames[code]
	return name, ok
}
