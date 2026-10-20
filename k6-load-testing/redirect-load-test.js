import { check } from 'k6';
import http from 'k6/http';
import { SharedArray } from 'k6/data';

const BASE_URL = 'https://lynkr-tor7.onrender.com';

// Pool of real shortcodes to hit, loaded once and shared across all VUs.
// data.json is a saved response of GET /api/links (an array of links); refresh it with
//   curl 'http://localhost:8080/api/links?limit=100' > data.json
const shortkeys = new SharedArray('shortkeys', function () {
	return JSON.parse(open('./data.json')).map((link) => link.code);
});

export const options = {
	vus: 100, // 100 virtual users running simultaneously
	iterations: 10000, // Total number of requests across all VUs
};

export default function () {
	// Pick a random key from the pool
	const key = shortkeys[Math.floor(Math.random() * shortkeys.length)];

	const res = http.get(`${BASE_URL}/${key}`, { redirects: 0 });

	check(res, {
		'redirect status is 302': (r) => r.status === 302,
	});
}
