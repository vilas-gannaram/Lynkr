import http from 'k6/http';
import { check } from 'k6';

const shortkeys = [
	'36kawpps',
	'n9fnxs47',
	'9w8tn8yx',
	'mswzdt9t',
	'4qjhgvfk',
	'e9g3yb4g',
];

export const options = {
	vus: 100, // 10 virtual users running simultaneously
	iterations: 10000, // Total number of requests across all VUs
};

export default function () {
	// Pick a random key from the list
	const key = shortkeys[Math.floor(Math.random() * shortkeys.length)];

	let res = http.get(`http://localhost:8080/${key}`);

	check(res, {
		'final status is 200': (r) => r.status === 200,
	});
}
