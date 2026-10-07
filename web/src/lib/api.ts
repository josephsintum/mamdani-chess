// What every API call does with a reply: read its JSON, and turn a refusal
// into the server's own words.

/** A reply's JSON, or {} when it has none (an empty body, a proxy's HTML page). */
export async function jsonOf<T extends object>(res: Response): Promise<Partial<T>> {
	return ((await res.json().catch(() => ({}))) ?? {}) as Partial<T>;
}

/** Why the server refused: its `error`, else the status text. */
export async function errorOf(res: Response): Promise<string> {
	return (await jsonOf<{ error: string }>(res)).error ?? res.statusText;
}

/** POSTs JSON (or nothing); returns null, or why it was refused. */
export async function post(path: string, body?: unknown): Promise<string | null> {
	const res = await fetch(
		path,
		body === undefined
			? { method: 'POST' }
			: { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) }
	);
	return res.ok ? null : errorOf(res);
}
