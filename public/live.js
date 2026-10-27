// Keeps the page live. Connects to the server's event stream and puts each
// fragment into the element whose data-sse attribute names it. After each
// event, a 'fragment' event naming it is dispatched on document for code
// that reacts to new content.
import { report } from './report.js';

// EventSource retries dropped connections by itself, but gives up for good
// on an HTTP error (e.g. a proxy's 502 during a deploy). Then start over.
const RETRY_MS = 5000;

// handlers maps the names of events that aren't page fragments to functions
// that get the event's data. EventSource only delivers events it has a
// listener for, so every name must be known up front. The names also go to
// the server, which then sends only those: each page gets what it shows.
export function connect(url, handlers = {}) {
	const listeners = {};
	for (const el of document.querySelectorAll('[data-sse]')) {
		listeners[el.dataset.sse] = (html) => {
			el.innerHTML = html;
		};
	}
	Object.assign(listeners, handlers);
	const events = Object.keys(listeners).join(',');

	function open() {
		const source = new EventSource(`${url}?events=${events}`);
		for (const [name, fn] of Object.entries(listeners)) {
			source.addEventListener(name, (e) => {
				fn(e.data);
				document.dispatchEvent(new CustomEvent('fragment', { detail: name }));
			});
		}
		source.addEventListener('open', () => report('sse: connected'));
		source.addEventListener('error', () => {
			if (source.readyState === EventSource.CLOSED) {
				report('sse: closed, reconnecting');
				setTimeout(open, RETRY_MS);
			}
		});
	}
	open();
}
