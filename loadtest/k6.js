// Peak-hour load test for the BiteExpress discovery API.
//
//   k6 run loadtest/k6.js
//   k6 run -e BASE_URL=http://localhost:8082/api -e PEAK_RPS=300 loadtest/k6.js
//
// Seed the data first (`make seed`). To measure the cache, run twice and compare:
//   make up                                   # cache on (default)
//   k6 run --summary-export=cache-on.json loadtest/k6.js
//   BITE_CACHE_ENABLED=false docker compose up -d api
//   k6 run --summary-export=cache-off.json loadtest/k6.js
import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8082/api';
const BASE_RPS = Number(__ENV.BASE_RPS || 20); // quiet period
const PEAK_RPS = Number(__ENV.PEAK_RPS || 150); // lunch / dinner rush

export const options = {
  scenarios: {
    peak_hour: {
      executor: 'ramping-arrival-rate',
      startRate: BASE_RPS,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 400,
      stages: [
        { target: BASE_RPS, duration: '20s' }, // normal traffic
        { target: PEAK_RPS, duration: '30s' }, // ramp into the rush
        { target: PEAK_RPS, duration: '60s' }, // hold the peak
        { target: BASE_RPS, duration: '20s' }, // recover
      ],
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    'http_req_duration{endpoint:list}': ['p(95)<300'],
    'http_req_duration{endpoint:search}': ['p(95)<400'],
    'http_req_duration{endpoint:by_id}': ['p(95)<200'],
    checks: ['rate>0.99'],
  },
};

// Popular queries repeat a lot in real traffic, which is what a read cache exploits.
const CITIES = ['Gurgaon', 'New Delhi', 'Noida', 'Faridabad'];
const CUISINES = ['north indian', 'chinese', 'south indian', 'fast food', 'cafe'];
const SEARCHES = [
  'Best restaurants in Gurgaon with South Indian Food',
  'Restaurants in Gurgaon under Rs. 1000 for two',
  'best chinese restaurants in New Delhi',
  'cheap cafes in Noida',
];
// A handful of user locations around Gurgaon / Delhi.
const LOCATIONS = [
  [28.4595, 77.0266],
  [28.4089, 77.0378],
  [28.6139, 77.209],
  [28.5355, 77.391],
];

const pick = (a) => a[Math.floor(Math.random() * a.length)];

export function setup() {
  const res = http.get(`${BASE_URL}/v1/restaurants?city=Gurgaon&limit=50`);
  const items = res.status === 200 ? res.json('items') : [];
  if (!items || items.length === 0) {
    throw new Error('No data. Run `make seed` before the load test.');
  }
  return { ids: items.map((r) => r.id) };
}

function tagged(endpoint) {
  return { tags: { endpoint } };
}

export default function (data) {
  const roll = Math.random();
  let res;
  if (roll < 0.5) {
    // Browse: city + cuisine listings, top rated first. The most cacheable traffic.
    const q = `city=${encodeURIComponent(pick(CITIES))}&cuisine=${encodeURIComponent(pick(CUISINES))}&sort=rating&limit=20`;
    res = http.get(`${BASE_URL}/v1/restaurants?${q}`, tagged('list'));
  } else if (roll < 0.65) {
    // Nearby: exact coordinates make cache hits rare, so this stresses MongoDB's $geoNear.
    const [lat, lng] = pick(LOCATIONS);
    const jitter = () => (Math.random() - 0.5) * 0.01;
    res = http.get(
      `${BASE_URL}/v1/restaurants?lat=${lat + jitter()}&lng=${lng + jitter()}&radius=3000&sort=distance&limit=20`,
      tagged('list'),
    );
  } else if (roll < 0.9) {
    // Natural-language search.
    res = http.get(`${BASE_URL}/v1/restaurants/search?q=${encodeURIComponent(pick(SEARCHES))}`, tagged('search'));
  } else {
    // Open a restaurant's detail page.
    res = http.get(`${BASE_URL}/v1/restaurants/${pick(data.ids)}`, tagged('by_id'));
  }
  check(res, { 'status is 200': (r) => r.status === 200 });
}
