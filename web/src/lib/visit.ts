// What the page tells the server about a visit: the route it's on, the
// screen and whether it's a touch screen (for phone/tablet/desktop) and, on
// the first page, where the visitor came from. The server keeps the route, never a game code, and the
// referrer's host, never its path.

export type VisitPayload = { path: string; w: number; h: number; touch: boolean; referrer: string };

export function visitPayload(
	url: URL,
	first: boolean,
	referrer: string,
	screen: { width: number; height: number },
	touch: boolean
): VisitPayload {
	const tag = url.searchParams.get('ref');
	return {
		path: url.pathname,
		w: screen.width,
		h: screen.height,
		touch,
		referrer: tag ? `ref:${tag}` : first ? referrer : ''
	};
}

/** Sends a page view. Never throws and never surfaces a failure: a visit is not worth a toast. */
export function reportVisit(payload: VisitPayload): void {
	send('/api/visit', payload);
}

const ERROR_GAP_MS = 10_000;
let lastError = -Infinity;

/** Reports an uncaught error, at most once every 10 s, so a loop can't flood the server. */
export function reportError(path: string, message: string): void {
	const now = Date.now();
	if (now - lastError < ERROR_GAP_MS) return;
	lastError = now;
	send('/api/error', { path, message: message.slice(0, 300) });
}

function send(path: string, body: unknown): void {
	const json = JSON.stringify(body);
	try {
		if (typeof navigator !== 'undefined' && typeof navigator.sendBeacon === 'function') {
			if (navigator.sendBeacon(path, new Blob([json], { type: 'application/json' }))) return;
		}
		fetch(path, { method: 'POST', headers: { 'content-type': 'application/json' }, body: json, keepalive: true }).catch(
			() => {}
		);
	} catch {
		// A blocked beacon or a page that is going away: nothing to do.
	}
}
