# bite-express

Restaurant discovery API for **BiteExpress**. Go + MongoDB, with filtered listing, geo-distance sorting, and a natural-language search endpoint.

## How this maps to BiteExpress's goals

| Goal | What the MVP does about it | What comes next |
|---|---|---|
| **Peak-hour reliability and performance (6-12 months), so traffic spikes do not cost revenue** | Read-through cache in front of MongoDB, with single-flight so identical concurrent requests share one query. Distance, filtering and sorting run in one `$geoNear` aggregation backed by compound indexes. Bounded page sizes (max 100). `/readyz` checks MongoDB so a load balancer can drain a sick instance. Input limits and validation protect the API. The cache can be switched off to measure its effect (see *Load test*). | Shared Redis cache, `secondaryPreferred` reads, request timeouts, rate limiting, metrics. |
| **Conversion: help hungry customers find food fast** | Natural-language search with an `interpreted` echo the app can show and correct. "Best" requires a minimum vote count, so a 5.0 from two votes cannot win. | Atlas Search for fuzzy text and autocomplete, locality recognition. |
| **Cloud-native, 2-3 years** | Stateless, containerised API. Small interfaces (`RestaurantsRepository`, `cache.Provider`) so the cache and datastore can be swapped without touching handlers. | MongoDB Atlas, Kubernetes, secrets from a manager. |
| **Coexist with the on-prem relational monolith** | Idempotent seed keyed on `(tenantId, sourceId)` shows the ingestion shape: re-runnable, no duplicates. | CDC from the relational database into MongoDB, then phased traffic cut-over by endpoint. |
| **International expansion and global data regulation** | `country`, `countryCode` and `currency` are first-class fields. Prices are normalised to ISO currency codes. | Zone sharding by region for data residency, field-level encryption for PII, per-region config. |
| **General purpose: usable by several similar businesses** | Multi-tenant from day one: `tenantId` leads every index and every query and is forced server-side on create. A free-form `attributes` field holds per-business fields without migrations. | Per-request tenant from an API key or JWT, per-tenant configuration. |

Deliberately out of scope for the MVP, and how they would be handled: authentication and
per-request tenancy (tenant comes from config today), PII and compliance controls, and
production observability. They are listed in *Scaling and roadmap*.

## Toolchain with mise (optional)

[mise](https://mise.jdx.dev) installs the exact tool versions this project needs, scoped to
the project directory, so nothing has to be installed globally:

```bash
brew install mise            # or: curl https://mise.run | sh
mise trust && mise install   # Go, golangci-lint v2 and k6, pinned in mise.toml
mise run test                # run a task, or use `mise exec -- make test`
mise run test-e2e
mise run loadtest
```

To get the tools on your `PATH` in an interactive shell, add `eval "$(mise activate zsh)"`
(or bash/fish) to your shell profile; then plain `make seed`, `make lint` and `make loadtest`
work inside the project directory.

Docker is the one prerequisite mise does not provide: the stack (`make up`), `make seed`,
`make test-integration` and `make test-e2e` need a running Docker daemon.

## Quick Start (Docker)

Requires Docker and, for the seed step, Go (see `go.mod`).

```bash
make up      # builds the API image, starts MongoDB, then the API on http://localhost:8082
make seed    # loads data/zomato.csv into MongoDB (cleaned, idempotent, safe to re-run)
```

`make up` starts `mongodb` and waits until it is healthy, then starts `api`. The seed is a
separate command because it runs on your machine against the MongoDB port that Compose
publishes (`localhost:27017`). It reads the CSV, cleans it (see *Seed / cleanup rules*) and
upserts the restaurants.

Check it is up:

```bash
curl localhost:8082/api/healthz
curl "localhost:8082/api/v1/restaurants?city=Gurgaon&limit=3"
```

> All routes are served under the `/api` prefix, e.g. `/api/v1/restaurants`.

> **Seed before you search.** The natural-language search learns the known cities and
> cuisines from the data the first time it is used and keeps them until the API restarts.
> If you searched before seeding, run `docker compose restart api`.

```bash
make help    # list all make targets
make down    # stop everything (data is kept in the mongo_data volume)
```

Optional: `mongo-express` (a MongoDB browser) is part of the Compose file on
http://localhost:8081.

> **Demo credentials only.** `secretpassword` (MongoDB root) and the mongo-express login in
> `docker-compose.yml`, `config-local.yml` and `config-docker.yml` are throwaway values for
> local demos. Never reuse them. In a real deployment, inject credentials from a secret
> manager or environment variables, enable TLS, and do not publish port 27017.

### Run the API without Docker

```bash
docker compose up -d mongodb   # just the database
make seed
make server                    # APP_ENV=local, uses config-local.yml, listens on :8082
```

Config is read from `config-<APP_ENV>.yml` in the working directory (`local` by default,
`docker` inside Compose). `tenantId` in the config is stamped on every document and applied
to every query.

## API

Base path: `/api`. All responses are JSON; errors look like `{"error": "..."}`.

### `POST /v1/restaurants`

```bash
curl -i -X POST localhost:8082/api/v1/restaurants \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Dosa Corner", "address": "12 MG Road", "locality": "Sector 29",
    "city": "Gurgaon", "country": "India", "countryCode": 1,
    "lat": 28.4595, "lng": 77.0266,
    "cuisines": ["South Indian"],
    "pricing": {"costForTwo": 600, "currency": "INR", "priceRange": 2},
    "flags": {"hasOnlineDelivery": true}
  }'
```

Returns `201` with the created entity and a `Location` header. Required: `name`, `address`,
`city`, `country`, at least one cuisine; `lat`/`lng` must be given together. Validation errors
return `400` with a `details` list; `409` on conflict; `415` if not `application/json`;
`413` above 1 MB. Unknown JSON fields are rejected.

### `GET /v1/restaurants/{id}`

```bash
curl localhost:8082/api/v1/restaurants/<id>
```

Full detail: address, locality, location, pricing, rating (average, votes, text), flags.
`400` for a malformed id, `404` if not found (scoped to the tenant).

### `GET /v1/restaurants`

```bash
# top-rated South Indian places in Gurgaon
curl "localhost:8082/api/v1/restaurants?city=Gurgaon&cuisine=south%20indian&sort=rating&limit=5"

# nearest places within 3 km, sorted by distance
curl "localhost:8082/api/v1/restaurants?lat=28.4595&lng=77.0266&radius=3000&sort=distance"

# under Rs. 1000 for two, rated 4+
curl "localhost:8082/api/v1/restaurants?city=Gurgaon&maxCost=1000&minRating=4"
```

| Param | Meaning |
|---|---|
| `name`, `city` | text filters (name: contains, case-insensitive; city: exact, case-insensitive) |
| `cuisine` | repeatable and/or comma-separated; a restaurant must serve **all** listed |
| `minRating`, `maxCost`, `currency` | numeric bounds; `maxCost` is cost for two, INR unless `currency` set |
| `lat`, `lng`, `radius` | location (both required together); `radius` in metres, optional |
| `sort`, `order` | `rating` \| `distance` \| `name` \| `cost`; `asc` \| `desc` (rating defaults to desc) |
| `limit`, `offset` | pagination; default 20, max 100 |

Response: `{"items": [...], "hasMore": bool}`. Each item has `id`, `name`, `address`, `city`,
`country`, `cuisines`, `averageRating` (`null` if unrated) and `distance` in metres (only when
`lat`/`lng` were given).

### `GET /v1/restaurants/search`

Accepts everything the list endpoint does, plus `q` (natural language, max 200 chars).
Explicit parameters always win over what is inferred from `q`.

```bash
curl "localhost:8082/api/v1/restaurants/search?q=Best+restaurants+in+Gurgaon+with+South+Indian+food"
curl "localhost:8082/api/v1/restaurants/search?q=Restaurants+in+Gurgaon+under+Rs.+1000+for+two"
curl "localhost:8082/api/v1/restaurants/search?q=Nearest+place+that+serves+Vada+Pav&lat=28.4595&lng=77.0266"
```

The response adds an `interpreted` object showing how the query was understood (city,
cuisines, max cost, min rating, best/nearest, leftover text, ignored terms) so clients can
display or correct it. A "nearest" query without `lat`/`lng` returns `400`.

## Data model

One collection, `restaurants`, one document per restaurant:

```
tenantId, sourceId?, name,
address{line, locality, city, countryCode, country},
location (GeoJSON Point, [lng, lat]),
cuisines[] (display), cuisineKeys[] (lower-cased, for matching),
pricing{costForTwo?, currency, priceRange}, rating{average?, votes, text},
flags{hasTableBooking, hasOnlineDelivery, isDeliveringNow},
attributes{} (free-form, for per-tenant extension), source{dataset}, createdAt, updatedAt
```

Indexes: `2dsphere` on `location`; unique partial `(tenantId, sourceId)`;
`(tenantId, address.city, rating.average desc)`; `(tenantId, cuisineKeys, rating.average desc)`.

### Seed / cleanup rules (`cmd/seed`)

Idempotent upserts keyed on `(tenantId, sourceId)`. The raw CSV is cleaned as follows: rows
with an invalid id, no name or a duplicate id are skipped (counts are printed); a UTF-8 BOM
and non-UTF-8 bytes are handled; coordinates that are unparseable, out of range or `0,0` leave
the restaurant without a `location` (it still appears in non-geo results); a non-positive cost
becomes `null`; "Not rated" becomes a `null` average instead of 0; cuisines are split,
trimmed and de-duplicated; country names come from the country code; currency strings such as
`Indian Rupees(Rs.)` are normalised to codes.

## Caching

List, search and by-id reads go through an **in-memory cache** that sits between the service
and MongoDB (`internal/repository/cached`, backed by `internal/cache`).

| Behaviour | Detail |
|---|---|
| What is cached | List/search results (per exact filter set and tenant) and restaurants by id. Errors and "not found" are never cached. |
| Freshness | Entries expire after `ttl` seconds (default 60). Creating a restaurant clears the cached lists. |
| Size | At most `maxEntries` list entries (default 1000) and ten times that many by-id entries; least recently used entries are evicted first. |
| Bursts | Identical requests that arrive together share one MongoDB query instead of each running their own. |
| Visibility | A line such as `cache stats: list hit_ratio=87% hits=... misses=...` is logged once a minute when there was traffic. |

Configure it in `config-<env>.yml`:

```yaml
cache:
  enabled: true
  type: "memory"      # the only supported type for now
  config:
    ttl: 60           # seconds
    maxEntries: 1000
```

Switch it off for a comparison run without editing files:

```bash
BITE_CACHE_ENABLED=false make server                    # local
BITE_CACHE_ENABLED=false docker compose up -d api       # Docker (recreates the api container)
```

Limits to know about:

- **Per instance.** Each API instance has its own cache. After a create, other instances catch
  up when their `ttl` runs out.
- **Results can be up to `ttl` old** (a new rating, "delivering now").
- **Nearby searches rarely hit.** The key contains the exact `lat`/`lng`, so only identical
  coordinates share an entry. City and cuisine listings hit well.
- **Memory is bounded by entry count, not bytes.** A list entry holds up to 100 restaurants;
  1000 typical 20-item entries are roughly 40 MB. Size `maxEntries` with that in mind.
- **A shared cache such as Redis is not implemented.** The code depends on a small
  `cache.Provider` interface, so a Redis-backed implementation can replace the in-memory one
  without changes to the repository or service.

## Design decisions and trade-offs

- **MongoDB as the operational store, one rich document per restaurant.** Reads for discovery
  need no joins; the document maps directly to the response shape and `attributes` allows
  per-business fields without migrations.
- **`$geoNear` for distance.** Distance is computed in the database (spherical, metres),
  combined with the other filters in one aggregation, and is only added when a location is given.
- **Multi-tenant from day one.** `tenantId` leads every index and every query, so the same
  codebase and cluster can serve several similar businesses. For the MVP it comes from config,
  not from the request.
- **Layered code** (`handler` → `service` → `repository` interface → `mongodb`), with the
  handler and service depending on small interfaces so they can be tested with fakes and the
  datastore can be swapped.
- **Rule-based NLQ instead of an LLM.** Deterministic, free, fast, testable and explainable
  (`interpreted`). Cities and cuisines come from the data itself, so the parser only
  recognises values that can actually match.
- **"Best" needs evidence.** "Best" sorts by rating but requires a minimum vote count
  (default 50), so a 5.0 from two votes cannot win.
- **Offset pagination with a `hasMore` flag** (fetch `limit + 1`). Simple and enough for a
  mobile list; it gets slower at deep offsets. Keyset pagination is the upgrade.
- **Regex for name/text matching.** Simple and correct, but case-insensitive "contains"
  regexes cannot use an index. See next steps.
- **Read-through in-memory cache.** Simple, no extra service to run, and enough to absorb
  bursts of identical queries. It is per instance; Redis is the planned replacement when there
  are several instances (see *Caching*).

## NLQ parser: limits and what comes next

How it works: the query is normalised, then each step removes the words it understood
(price → "for two" → rating → best/nearest → city → cuisines → dishes). Whatever is left
becomes a name/cuisine text match; filler words are ignored and reported in `unmatchedTerms`.

Known limits:

- English only, no typo tolerance or spelling variants beyond simple plurals ("cafes").
- Neighbourhood/locality names are not recognised, only cities and cuisines.
- Dish names map to a cuisine only through a small hand-written lexicon.
- No negation ("not spicy"), OR logic, or relative phrases beyond the supported patterns.
- Price is read as INR; other currencies need the explicit `currency` parameter.
- "Nearest" and "near me" need `lat`/`lng` from the client.
- The city/cuisine vocabulary is loaded once per process and cached. Newly created cities or
  cuisines are recognised only after a restart, and each instance holds its own copy.
- Leftover text uses regex matching, which scans the collection and has no relevance ranking.

Next steps:

1. Atlas Search (text, fuzzy matching, autocomplete, relevance) to replace regex matching.
2. Refresh the vocabulary on a TTL or via change streams.
3. Optional LLM-assisted parsing with this rule-based parser as the fallback, keeping the
   `interpreted` echo for transparency.
4. Locality recognition and a richer dish/synonym lexicon.

## Scaling and roadmap

- **Peak-hour reliability:** the in-memory read cache is in place. Still to do: a shared
  Redis cache for several instances, `secondaryPreferred` reads for discovery traffic,
  request timeouts, rate limiting, a readiness probe that checks MongoDB, and metrics.
- **Coexistence with the on-prem relational monolith:** change data capture (CDC) into
  MongoDB for restaurant data, then phased traffic cut-over by endpoint.
- **Cloud and international:** MongoDB Atlas, zone sharding by region/tenant for data
  residency, field-level encryption for PII, and per-tenant configuration and authentication.

## Tests

```bash
make test               # unit tests: go test ./...
make test-integration   # integration tests against a real MongoDB (needs Docker)
make test-e2e           # starts the stack, seeds it, tests every endpoint over HTTP
make lint               # golangci-lint (config in .golangci.yml)
```

Unit tests cover: query-parameter parsing and validation, DTO mapping, handler status codes
(with a fake service), service-level merging of the parsed query with explicit filters, the
NLQ parser and lexicon, aggregation-pipeline construction, seed row cleaning, the cache
(expiry, eviction, concurrent loads, invalidation) and the caching repository.

Integration tests (`//go:build integration`) run the real MongoDB repository against a live
database: index creation, insert, find by id, duplicate-key conflict, filtering by city and
cuisine, and `$geoNear` distance sorting. They use a throwaway database that is dropped
afterwards. `make test-integration` starts MongoDB from `docker-compose.yml` and points the
tests at it through `MONGO_URI`. If `MONGO_URI` is unset the tests are skipped.

CI (`.github/workflows/ci.yml`) runs lint, unit and integration tests, and the end-to-end
tests (Compose stack plus seed) on every push and pull request.

End-to-end tests (`test/e2e`, build tag `e2e`) call the running API over HTTP with the seeded
Zomato data: create and fetch, validation errors (400/413/415), 404s, list filtering, sorting,
pagination and distance, cache invalidation after a create, the three natural-language
examples from the brief, and bad input on search. `make test-e2e` runs them; set `API_URL` to
test an API that is already running.

## Load test

`loadtest/k6.js` simulates a peak-hour rush with [k6](https://k6.io) (`brew install k6`). It
ramps from a quiet baseline to a peak request rate, holds it, then recovers. The traffic mix
is 50% city and cuisine listings, 15% nearby searches, 25% natural-language searches and 10%
restaurant detail pages. Thresholds: under 1% errors, and p95 latency below 300 ms for
lists, 400 ms for search and 200 ms for detail.

```bash
make up && make seed
make loadtest                                   # defaults: 20 rps baseline, 150 rps peak
k6 run -e PEAK_RPS=300 loadtest/k6.js           # heavier peak
```

Compare the cache on and off to see what it buys:

```bash
k6 run --summary-export=cache-on.json loadtest/k6.js
BITE_CACHE_ENABLED=false docker compose up -d api
k6 run --summary-export=cache-off.json loadtest/k6.js
```

Results on my machine (fill these in after running; numbers depend on hardware):

| | cache on | cache off |
|---|---|---|
| p95 list | _ ms | _ ms |
| p95 search | _ ms | _ ms |
| p95 detail | _ ms | _ ms |
| failed requests | _ % | _ % |

Nearby searches use varying coordinates, so they rarely hit the cache. That is expected, and
it makes them the part that shows MongoDB's `$geoNear` performance.

## Project layout

```
.
├── cmd/
│   ├── server/
│   │   └── server.go             API entrypoint: loads config-<APP_ENV>.yml and starts the app
│   └── seed/
│       ├── main.go               reads the CSV and bulk-upserts into MongoDB
│       ├── clean.go              per-row cleaning rules
│       └── clean_test.go
├── internal/
│   ├── app/
│   │   └── app.go                starts the components in order: MongoDB connection, then REST server
│   ├── config/
│   │   ├── config.go             YAML config structs, loader, BITE_CACHE_ENABLED override
│   │   └── config_test.go
│   ├── api/rest/
│   │   ├── rest.go               REST server setup (path prefix /api)
│   │   ├── router/
│   │   │   └── router.go         routes and GET /healthz
│   │   └── handler/
│   │       ├── restaurants_handler.go   the four endpoints and their status-code mapping
│   │       ├── params.go                query-string parsing for list and search
│   │       ├── validate.go              POST body validation
│   │       ├── dto.go                   request and response shapes
│   │       └── *_test.go
│   ├── service/
│   │   ├── restaurants_svc.go    business logic; merges the parsed query with explicit filters
│   │   └── *_test.go
│   ├── search/nlq/
│   │   ├── parser.go             natural-language query to Intent
│   │   ├── intent.go             Intent and the vocabulary of known cities/cuisines
│   │   ├── lexicon.go            normalisation, patterns, word lists, dish-to-cuisine map
│   │   └── *_test.go
│   ├── cache/
│   │   ├── cache.go              generic in-memory cache: TTL, LRU, shared loads, stats
│   │   └── cache_test.go
│   └── repository/
│       ├── restaurants_repo.go   repository interface, filter types, sentinel errors
│       ├── repository.go         marker interface
│       ├── repository_manager.go registry used to look the repository up by name
│       ├── connections/
│       │   ├── conn_manager.go   opens connections, registers repositories, adds the cache
│       │   └── conn_manager_test.go
│       ├── cached/
│       │   ├── restaurants.go    caching decorator around the repository
│       │   └── restaurants_test.go
│       └── mongodb/
│           ├── repository.go             MongoDB client connect/close
│           ├── restaurants_repo_impl.go  indexes, insert, find by id, list, vocabulary
│           ├── pipeline.go               builds the list/search aggregation pipeline
│           ├── pkg.go                    package logger
│           └── *_test.go
├── pkg/
│   ├── restaurant.go             domain types (Restaurant, Address, Pricing, Rating, ...)
│   ├── normalize.go              cuisine, currency and GeoJSON helpers
│   ├── response.go               JSON response helper
│   └── normalize_test.go
├── data/
│   └── zomato.csv                sample dataset
├── build/
│   └── seed                      compiled seed binary (make build-seed)
├── config-local.yml              settings for running on your machine
├── config-docker.yml             settings used inside Docker Compose
├── Dockerfile                    multi-stage build of the API image
├── docker-compose.yml            mongodb, mongo-express, api
├── loadtest/k6.js                peak-hour load test (k6)
├── mise.toml                     pinned toolchain: Go, golangci-lint, k6
├── Makefile                      up, down, seed, server, test, test-integration, lint, help
├── .golangci.yml                 linter configuration
├── .github/workflows/ci.yml      lint + unit + integration tests
├── go.mod / go.sum
└── LICENSE
```

Request flow: `router` → `handler` (parse and validate) → `service` (NLQ, tenant) → `cached`
repository → `mongodb` repository → MongoDB.

## AI tools disclosure

- **Tools used:** Claude (Anthropic) via claude.ai / Claude Code
- **Used for:** Used for implementation plan generation, Reviewing the implementation against the brief; drafting the Dockerfile, Compose file and this README, implementing the nlq parser and related files, generating test cases
- **Written or decided by me:** Application Architecture, accepted trade-offs, implementation thought process.
- **Verified by me:** ran the stack and the tests; reviewed and edited all generated text
