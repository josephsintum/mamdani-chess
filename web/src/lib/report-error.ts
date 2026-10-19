// Tells the server about an uncaught error: the page's path (the server
// keeps the route, never a game code) and the message. Nothing about the
// person who saw it.

const ERROR_GAP_MS = 10_000;
let lastError = -Infinity;

/** Reports an uncaught error, at most once every 10 s, so a loop can't flood the server. Never throws. */
export function reportError(path: string, message: string): void {
	const now = Date.now();
	if (now - lastError < ERROR_GAP_MS) return;
	lastError = now;
	const json = JSON.stringify({ path, message: message.slice(0, 300) });
	try {
		if (typeof navigator !== 'undefined' && typeof navigator.sendBeacon === 'function') {
			if (navigator.sendBeacon('/api/error', new Blob([json], { type: 'application/json' }))) return;
		}
		fetch('/api/error', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: json,
			keepalive: true
		}).catch(() => {});
	} catch {
		// A blocked beacon or a page that is going away: nothing to do.
	}
}
