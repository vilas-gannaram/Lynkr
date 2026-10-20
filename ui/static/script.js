const form = document.getElementById('shorten-form');
const resultBox = document.getElementById('shorten-result');
const display = document.getElementById('short-url-display');
const resultMeta = document.getElementById('result-meta');
const openLink = document.getElementById('open-link');
const errorBox = document.getElementById('shorten-error');
const copyBtn = document.getElementById('copy-btn');
const tbody = document.getElementById('links-body');
const emptyMsg = document.getElementById('links-empty');
const summary = document.getElementById('links-summary');

let currentShortURL = '';

function showError(message) {
	errorBox.textContent = message;
	errorBox.hidden = false;
}

// Errors are JSON ({message}) from the codex helpers but plain text from http.Error.
async function readError(res) {
	const text = (await res.text()).trim();
	try {
		return JSON.parse(text).message || text;
	} catch {
		return text || 'Something went wrong.';
	}
}

function flashLabel(btn, label) {
	btn.textContent = 'Copied!';
	setTimeout(() => (btn.textContent = label), 1500);
}

function updateSummary() {
	const rows = tbody.querySelectorAll('tr');
	let clicks = 0;
	rows.forEach((r) => (clicks += Number(r.dataset.clicks || 0)));
	summary.textContent = `${rows.length} ${rows.length === 1 ? 'link' : 'links'} · ${clicks} clicks`;
	emptyMsg.hidden = rows.length > 0;
}

function button(label, attr, code, extraClass) {
	const btn = document.createElement('button');
	btn.type = 'button';
	btn.className = 'btn-small' + (extraClass ? ' ' + extraClass : '');
	btn.dataset[attr] = code;
	btn.textContent = label;
	return btn;
}

function addRow(code, target, title) {
	tbody.querySelectorAll('tr.is-new').forEach((r) => r.classList.remove('is-new'));

	const tr = document.createElement('tr');
	tr.className = 'is-new';
	tr.dataset.code = code;
	tr.dataset.clicks = '0';

	const codeTd = document.createElement('td');
	codeTd.className = 'code';
	const a = document.createElement('a');
	a.href = '/' + code;
	a.target = '_blank';
	a.rel = 'noopener noreferrer';
	a.textContent = '/' + code;
	codeTd.append(a);

	const destTd = document.createElement('td');
	destTd.className = 'dest';
	const url = document.createElement('div');
	url.className = 'dest-url';
	url.title = target;
	url.textContent = target;
	const when = document.createElement('div');
	when.className = 'meta-sm';
	when.textContent = new Date().toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
	if (title) {
		const heading = document.createElement('div');
		heading.className = 'dest-title';
		heading.textContent = title;
		destTd.append(heading);
	}
	destTd.append(url, when);

	const clicksTd = document.createElement('td');
	clicksTd.className = 'num strong';
	clicksTd.textContent = '0';

	const actTd = document.createElement('td');
	actTd.className = 'act';
	const del = button('Delete', 'delete', code, 'btn-danger');
	del.setAttribute('aria-label', `Delete /${code}`);
	actTd.append(del);

	tr.append(codeTd, destTd, clicksTd, actTd);
	tbody.prepend(tr);
	updateSummary();
}

if (form) {
	form.addEventListener('submit', async (e) => {
		e.preventDefault();
		errorBox.hidden = true;
		resultBox.hidden = true;

		const target = document.getElementById('long-url-input').value;
		const title = document.getElementById('title-input').value.trim();
		const submitBtn = form.querySelector('button[type=submit]');
		submitBtn.disabled = true;

		try {
			const res = await fetch('/api/links', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(title ? { target_url: target, title } : { target_url: target }),
			});

			if (!res.ok) {
				showError(await readError(res));
				return;
			}

			currentShortURL = await res.json();
			const code = currentShortURL.split('/').pop();

			display.textContent = currentShortURL.replace(/^https?:\/\//, '');
			resultMeta.textContent = `${target.length} → ${currentShortURL.length} characters · 0 clicks`;
			openLink.href = currentShortURL;
			resultBox.hidden = false;
			addRow(code, target, title);
			form.reset();
		} catch (err) {
			showError('Network error, please try again.');
		} finally {
			submitBtn.disabled = false;
		}
	});

	copyBtn.addEventListener('click', () => {
		navigator.clipboard.writeText(currentShortURL);
		flashLabel(copyBtn, 'Copy link');
	});
}

if (tbody) {
	tbody.addEventListener('click', async (e) => {
		const del = e.target.closest('button[data-delete]');
		if (!del) return;

		const code = del.dataset.delete;
		if (!confirm(`Delete /${code}? It will stop redirecting immediately.`)) return;

		del.disabled = true;
		try {
			const res = await fetch(`/api/links/${encodeURIComponent(code)}`, { method: 'DELETE' });
			if (!res.ok) {
				alert(await readError(res));
				del.disabled = false;
				return;
			}
			del.closest('tr').remove();
			updateSummary();
		} catch (err) {
			alert('Network error, please try again.');
			del.disabled = false;
		}
	});

	updateSummary();
}

// Pagination: HomePage reads ?page= and ?limit= (default 100; out of range falls back to 10).
// Without a total from the server, a full page means there may be an older one.
const pager = document.getElementById('pager');
if (pager) {
	const params = new URLSearchParams(location.search);
	const page = Math.max(1, parseInt(params.get('page'), 10) || 1);
	const rawLimit = params.has('limit') ? parseInt(params.get('limit'), 10) : 100;
	const pageSize = rawLimit >= 1 && rawLimit <= 100 ? rawLimit : 10;
	const count = Number(pager.dataset.count);

	const hrefFor = (n) => {
		const next = new URLSearchParams(location.search);
		next.set('page', n);
		return `?${next}`;
	};

	const prev = document.getElementById('pager-prev');
	const next = document.getElementById('pager-next');
	prev.hidden = page <= 1;
	next.hidden = count < pageSize;
	prev.href = hrefFor(page - 1);
	next.href = hrefFor(page + 1);
	document.getElementById('pager-page').textContent = `Page ${page}`;
	pager.hidden = prev.hidden && next.hidden;
}
