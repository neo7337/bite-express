//go:build e2e

// Package e2e exercises the running API over real HTTP, with MongoDB behind it.
//
// Run `make test-e2e` (starts the stack, seeds the Zomato data, runs these tests), or point
// API_URL at an already running instance, e.g. API_URL=http://localhost:8082/api.
// The tests use only the public JSON contract, never internal packages.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

var (
	baseURL = envOr("API_URL", "http://localhost:8082/api")
	client  = &http.Client{Timeout: 10 * time.Second}
)

type restaurant struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Address       string   `json:"address"`
	City          string   `json:"city"`
	Country       string   `json:"country"`
	Cuisines      []string `json:"cuisines"`
	AverageRating *float64 `json:"averageRating"`
	Distance      *float64 `json:"distance"`
}

type listResponse struct {
	Items       []restaurant `json:"items"`
	HasMore     bool         `json:"hasMore"`
	Interpreted *struct {
		City     string   `json:"city"`
		Cuisines []string `json:"cuisines"`
		MaxCost  *int     `json:"maxCost"`
		Best     bool     `json:"best"`
		Nearest  bool     `json:"nearest"`
	} `json:"interpreted"`
}

type detail struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Address  string   `json:"address"`
	Locality string   `json:"locality"`
	City     string   `json:"city"`
	Country  string   `json:"country"`
	Cuisines []string `json:"cuisines"`
	Location *struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"location"`
	Pricing struct {
		CostForTwo *int   `json:"costForTwo"`
		Currency   string `json:"currency"`
	} `json:"pricing"`
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func TestMain(m *testing.M) {
	resp, err := client.Get(baseURL + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "API not reachable at %s (err=%v). Run `make test-e2e`.\n", baseURL, err)
		os.Exit(1)
	}
	resp.Body.Close()
	os.Exit(m.Run())
}

// do sends a request and returns status, headers and the raw body.
func do(t *testing.T, method, path, contentType, body string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, baseURL+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b
}

func get(t *testing.T, path string) (int, []byte) {
	t.Helper()
	status, _, b := do(t, http.MethodGet, path, "", "")
	return status, b
}

func getList(t *testing.T, path string) listResponse {
	t.Helper()
	status, b := get(t, path)
	if status != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", path, status, b)
	}
	var out listResponse
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode %s: %v\n%s", path, err, b)
	}
	return out
}

func createRestaurant(t *testing.T, body string) (int, http.Header, []byte) {
	t.Helper()
	return do(t, http.MethodPost, "/v1/restaurants", "application/json", body)
}

// needSeed skips tests that rely on the Zomato data being loaded (`make seed`).
func needSeed(t *testing.T) {
	t.Helper()
	if len(getList(t, "/v1/restaurants?city=Gurgaon&limit=1").Items) == 0 {
		t.Skip("no Gurgaon data; run `make seed` first")
	}
}

func uniqueName(prefix string) string { return fmt.Sprintf("%s %d", prefix, time.Now().UnixNano()) }

func containsFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- health

func TestHealth(t *testing.T) {
	for _, p := range []string{"/healthz", "/readyz"} {
		if status, b := get(t, p); status != http.StatusOK {
			t.Errorf("GET %s = %d: %s", p, status, b)
		}
	}
}

// ---------------------------------------------------------------- create + get

func TestCreateThenGetByID(t *testing.T) {
	name := uniqueName("E2E Dosa Corner")
	status, hdr, b := createRestaurant(t, fmt.Sprintf(`{
		"name": %q, "address": "12 MG Road", "locality": "Sector 29",
		"city": "Gurgaon", "country": "India", "countryCode": 1,
		"lat": 28.4595, "lng": 77.0266, "cuisines": ["South Indian", "south  indian", "Chinese"],
		"pricing": {"costForTwo": 600, "currency": "INR", "priceRange": 2}
	}`, name))
	if status != http.StatusCreated {
		t.Fatalf("POST = %d: %s", status, b)
	}
	var created detail
	if err := json.Unmarshal(b, &created); err != nil || created.ID == "" {
		t.Fatalf("bad create response (%v): %s", err, b)
	}
	if loc := hdr.Get("Location"); !strings.HasSuffix(loc, "/"+created.ID) {
		t.Errorf("Location = %q, want suffix /%s", loc, created.ID)
	}
	if len(created.Cuisines) != 2 {
		t.Errorf("cuisines should be de-duplicated, got %v", created.Cuisines)
	}

	status, b = get(t, "/v1/restaurants/"+created.ID)
	if status != http.StatusOK {
		t.Fatalf("GET by id = %d: %s", status, b)
	}
	var got detail
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != name || got.City != "Gurgaon" || got.Country != "India" || got.Locality != "Sector 29" {
		t.Errorf("unexpected detail: %+v", got)
	}
	if got.Location == nil || got.Location.Lat != 28.4595 || got.Location.Lng != 77.0266 {
		t.Errorf("location = %+v, want lat 28.4595 lng 77.0266", got.Location)
	}
	if got.Pricing.CostForTwo == nil || *got.Pricing.CostForTwo != 600 || got.Pricing.Currency != "INR" {
		t.Errorf("pricing = %+v", got.Pricing)
	}
	if strings.Contains(string(b), "tenant") {
		t.Errorf("tenant id leaked: %s", b)
	}
}

func TestCreateValidation(t *testing.T) {
	cases := []struct {
		name, ctype, body string
		want              int
	}{
		{"missing required fields", "application/json", `{"name":"x"}`, http.StatusBadRequest},
		{"no cuisines", "application/json", `{"name":"x","address":"a","city":"c","country":"India","cuisines":[]}`, http.StatusBadRequest},
		{"lat without lng", "application/json", `{"name":"x","address":"a","city":"c","country":"India","cuisines":["Cafe"],"lat":28.4}`, http.StatusBadRequest},
		{"lat out of range", "application/json", `{"name":"x","address":"a","city":"c","country":"India","cuisines":["Cafe"],"lat":95,"lng":77}`, http.StatusBadRequest},
		{"unknown field", "application/json", `{"name":"x","address":"a","city":"c","country":"India","cuisines":["Cafe"],"bogus":1}`, http.StatusBadRequest},
		{"malformed json", "application/json", `{"name":`, http.StatusBadRequest},
		{"two objects", "application/json", `{"name":"x"}{"name":"y"}`, http.StatusBadRequest},
		{"wrong content type", "text/plain", `{"name":"x"}`, http.StatusUnsupportedMediaType},
		{"body too large", "application/json", `{"name":"` + strings.Repeat("a", 1<<20+10) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _, b := do(t, http.MethodPost, "/v1/restaurants", c.ctype, c.body)
			if status != c.want {
				t.Fatalf("status = %d, want %d: %.200s", status, c.want, b)
			}
			if !strings.Contains(string(b), `"error"`) {
				t.Errorf("error body should contain an error field: %.200s", b)
			}
		})
	}

	t.Run("validation lists every problem", func(t *testing.T) {
		_, _, b := createRestaurant(t, `{}`)
		var out struct {
			Details []string `json:"details"`
		}
		if err := json.Unmarshal(b, &out); err != nil || len(out.Details) < 4 {
			t.Fatalf("want >=4 details, got %v (%v): %s", out.Details, err, b)
		}
	})
}

func TestGetByID_Errors(t *testing.T) {
	cases := []struct {
		name, id string
		want     int
	}{
		{"malformed id", "not-an-object-id", http.StatusBadRequest},
		{"unknown id", "000000000000000000000000", http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if status, b := get(t, "/v1/restaurants/"+c.id); status != c.want {
				t.Fatalf("status = %d, want %d: %s", status, c.want, b)
			}
		})
	}
}

// ---------------------------------------------------------------- list

func TestList_RequiredAttributes(t *testing.T) {
	needSeed(t)
	status, b := get(t, "/v1/restaurants?city=Gurgaon&limit=3&lat=28.4595&lng=77.0266")
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, b)
	}
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(b, &raw); err != nil || len(raw.Items) == 0 {
		t.Fatalf("no items (%v): %s", err, b)
	}
	for _, key := range []string{"id", "name", "address", "city", "country", "cuisines", "averageRating", "distance"} {
		if _, ok := raw.Items[0][key]; !ok {
			t.Errorf("list item is missing %q: %v", key, raw.Items[0])
		}
	}
}

func TestList_FilterByCityAndCuisine(t *testing.T) {
	needSeed(t)
	res := getList(t, "/v1/restaurants?city=Gurgaon&cuisine=north+indian&limit=20")
	if len(res.Items) == 0 {
		t.Fatal("expected Gurgaon north indian restaurants")
	}
	for _, r := range res.Items {
		if !strings.EqualFold(r.City, "Gurgaon") || !containsFold(r.Cuisines, "North Indian") {
			t.Errorf("filter violated: %+v", r)
		}
	}
}

func TestList_SortByRatingDescending(t *testing.T) {
	needSeed(t)
	res := getList(t, "/v1/restaurants?city=Gurgaon&sort=rating&order=desc&limit=30")
	var got []float64
	for _, r := range res.Items {
		if r.AverageRating != nil {
			got = append(got, *r.AverageRating)
		}
	}
	if len(got) < 2 || !sort.SliceIsSorted(got, func(i, j int) bool { return got[i] > got[j] }) {
		t.Errorf("ratings not descending: %v", got)
	}
}

func TestList_MinRating(t *testing.T) {
	needSeed(t)
	for _, r := range getList(t, "/v1/restaurants?city=Gurgaon&minRating=4.2&limit=30").Items {
		if r.AverageRating == nil || *r.AverageRating < 4.2 {
			t.Errorf("rating %v below minRating 4.2 for %s", r.AverageRating, r.Name)
		}
	}
}

func TestList_NearestFirstWithRadius(t *testing.T) {
	needSeed(t)
	res := getList(t, "/v1/restaurants?lat=28.4595&lng=77.0266&radius=5000&sort=distance&limit=30")
	if len(res.Items) < 2 {
		t.Skip("not enough nearby data")
	}
	prev := -1.0
	for _, r := range res.Items {
		if r.Distance == nil {
			t.Fatalf("%s has no distance", r.Name)
		}
		if *r.Distance > 5000 || *r.Distance < prev {
			t.Errorf("distance %v out of order or beyond radius (prev %v)", *r.Distance, prev)
		}
		prev = *r.Distance
	}
}

func TestList_Pagination(t *testing.T) {
	needSeed(t)
	page1 := getList(t, "/v1/restaurants?city=Gurgaon&sort=name&limit=2&offset=0")
	page2 := getList(t, "/v1/restaurants?city=Gurgaon&sort=name&limit=2&offset=2")
	if len(page1.Items) != 2 || !page1.HasMore || len(page2.Items) == 0 {
		t.Fatalf("page1=%d hasMore=%v page2=%d", len(page1.Items), page1.HasMore, len(page2.Items))
	}
	if page1.Items[0].ID == page2.Items[0].ID || page1.Items[1].ID == page2.Items[0].ID {
		t.Error("pages overlap")
	}
}

func TestList_BadParameters(t *testing.T) {
	for _, q := range []string{"lat=28.4", "lng=77.0", "limit=abc", "limit=-1", "sort=bogus", "minRating=high", "sort=distance"} {
		t.Run(q, func(t *testing.T) {
			if status, b := get(t, "/v1/restaurants?"+q); status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %.200s", status, b)
			}
		})
	}
}

// A new restaurant must show up in a list that was already served (and cached) before it existed.
func TestList_CacheIsInvalidatedByCreate(t *testing.T) {
	name := uniqueName("E2E Cache Probe")
	path := "/v1/restaurants?name=" + url.QueryEscape(name)

	if n := len(getList(t, path).Items); n != 0 {
		t.Fatalf("precondition: want 0 items, got %d", n)
	}
	getList(t, path) // second call is served from the cache

	status, _, b := createRestaurant(t, fmt.Sprintf(
		`{"name":%q,"address":"1 Test Rd","city":"Gurgaon","country":"India","cuisines":["Cafe"]}`, name))
	if status != http.StatusCreated {
		t.Fatalf("create = %d: %s", status, b)
	}
	if items := getList(t, path).Items; len(items) != 1 || items[0].Name != name {
		t.Fatalf("created restaurant not visible after create: %+v", items)
	}
}

// ---------------------------------------------------------------- search

func TestSearch_IsNotMistakenForAnID(t *testing.T) {
	status, b := get(t, "/v1/restaurants/search")
	if status != http.StatusOK || !strings.Contains(string(b), `"items"`) {
		t.Fatalf("GET /search = %d: %.200s", status, b)
	}
}

func TestSearch_BestRestaurantsInCityWithCuisine(t *testing.T) {
	needSeed(t)
	res := getList(t, "/v1/restaurants/search?q="+url.QueryEscape("Best restaurants in Gurgaon with South Indian Food"))
	if res.Interpreted == nil || !strings.EqualFold(res.Interpreted.City, "Gurgaon") ||
		!containsFold(res.Interpreted.Cuisines, "South Indian") || !res.Interpreted.Best {
		t.Fatalf("misinterpreted query: %+v", res.Interpreted)
	}
	if len(res.Items) == 0 {
		t.Fatal("expected results")
	}
	for _, r := range res.Items {
		if !strings.EqualFold(r.City, "Gurgaon") || !containsFold(r.Cuisines, "South Indian") {
			t.Errorf("result does not match the query: %+v", r)
		}
	}
}

func TestSearch_PriceForTwo(t *testing.T) {
	needSeed(t)
	res := getList(t, "/v1/restaurants/search?q="+url.QueryEscape("Restaurants in Gurgaon under Rs. 1000 for two"))
	if res.Interpreted == nil || res.Interpreted.MaxCost == nil || *res.Interpreted.MaxCost != 1000 ||
		!strings.EqualFold(res.Interpreted.City, "Gurgaon") {
		t.Fatalf("misinterpreted query: %+v", res.Interpreted)
	}
	if len(res.Items) == 0 {
		t.Fatal("expected results")
	}
}

func TestSearch_NearestWithLocation(t *testing.T) {
	needSeed(t)
	res := getList(t, "/v1/restaurants/search?lat=28.4595&lng=77.0266&q="+url.QueryEscape("Nearest place that serves Vada Pav"))
	if res.Interpreted == nil || !res.Interpreted.Nearest {
		t.Fatalf("expected the nearest intent: %+v", res.Interpreted)
	}
	prev := -1.0
	for _, r := range res.Items {
		if r.Distance == nil || *r.Distance < prev {
			t.Errorf("results not ordered by distance: %+v", r)
		}
		if r.Distance != nil {
			prev = *r.Distance
		}
	}
}

func TestSearch_NearestWithoutLocationIsRejected(t *testing.T) {
	needSeed(t)
	if status, b := get(t, "/v1/restaurants/search?q="+url.QueryEscape("Nearest place that serves Vada Pav")); status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %.200s", status, b)
	}
}

func TestSearch_ExplicitParametersWinOverTheQuery(t *testing.T) {
	needSeed(t)
	res := getList(t, "/v1/restaurants/search?city=Gurgaon&limit=3&q="+url.QueryEscape("best restaurants in Delhi"))
	for _, r := range res.Items {
		if !strings.EqualFold(r.City, "Gurgaon") {
			t.Errorf("explicit city=Gurgaon should win, got %s", r.City)
		}
	}
}

func TestSearch_AcceptsEveryListParameter(t *testing.T) {
	needSeed(t)
	res := getList(t, "/v1/restaurants/search?city=Gurgaon&cuisine=north+indian&minRating=3.5&sort=rating&limit=5")
	if len(res.Items) == 0 || len(res.Items) > 5 {
		t.Fatalf("got %d items", len(res.Items))
	}
}

func TestSearch_RejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"query too long": "q=" + strings.Repeat("a", 201),
		"bad limit":      "q=cafes&limit=abc",
		"bad sort":       "q=cafes&sort=bogus",
	}
	for name, q := range cases {
		t.Run(name, func(t *testing.T) {
			if status, b := get(t, "/v1/restaurants/search?"+q); status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %.200s", status, b)
			}
		})
	}
}

// ---------------------------------------------------------------- misc

func TestUnknownRouteAndMethod(t *testing.T) {
	if status, _ := get(t, "/v1/nope"); status != http.StatusNotFound {
		t.Errorf("unknown route = %d, want 404", status)
	}

	// The router has no route for DELETE. It either answers 4xx or drops the connection
	// (the golly server closes it without a response); either way the request must not succeed.
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/v1/restaurants/000000000000000000000000", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("DELETE: connection closed without a response (%v); the API has no 405 handling", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 400 {
		t.Errorf("DELETE should not succeed, got %d", resp.StatusCode)
	}
}
