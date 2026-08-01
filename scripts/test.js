import { check } from 'k6';
import http from 'k6/http';

const shortkeys = [
	'6y2c6t79',
	'cn2vd59v',
	'mdxtdrk2',
	'd2z4tzgb',
	'j8cy6efy',
	'zyd3gzh3',
	'ej9j5cq7',
	'4bd3bfxv',
	'qtwgejgm',
	'ba3za6jy',
	'trw8qhcc',
	'ytdqmavk',
	'taxxk3qn',
	'2eds9e3b',
	'8v6dtxtw',
	'8q6qfknb',
	'm68xr92t',
	'9sygtyck',
	'xmyhfjnp',
	'tamgvg26',
	'kaka5s96',
	'545jtw7d',
];

export const options = {
	vus: 10, // 10 virtual users running simultaneously
	iterations: 100, // Total number of requests across all VUs
};

export default function () {
	// Pick a random key from the list
	const key = shortkeys[Math.floor(Math.random() * shortkeys.length)];

	let res = http.get(`https://lynkr-tor7.onrender.com/${key}`);

	check(res, {
		'final status is 200': (r) => r.status === 200,
	});
}
