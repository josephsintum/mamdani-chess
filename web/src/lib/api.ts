// What every API call does with a reply: read its JSON, and turn a refusal
// into the server's own words.

/** A reply's JSON, or {} when it has none (an empty body, a proxy's HTML page). */
export async function jsonOf<T extends object>(res: Response): Promise<Partial<T>> {
	return ((await res.json().catch(() => ({}))) ?? {}) as Partial<T>;
}

/** A POST of `body` as JSON. */
export const jsonPost = (body: unknown): RequestInit => ({
	method: 'POST',
	headers: { 'content-type': 'application/json' },
	body: JSON.stringify(body)
});

/** GETs (or `init`s) a reply's JSON; throws "could not <what> (status)" on a refusal. */
export async function fetchJSON<T>(path: string, what: string, init?: RequestInit): Promise<T> {
	const res = await fetch(path, init);
	if (!res.ok) throw new Error(`could not ${what} (${res.status})`);
	return (await res.json()) as T;
}

/** POSTs JSON (or nothing); returns null, or why it was refused: the server's `error`, else the status text. */
export async function post(path: string, body?: unknown): Promise<string | null> {
	const res = await fetch(path, body === undefined ? { method: 'POST' } : jsonPost(body));
	return res.ok ? null : ((await jsonOf<{ error: string }>(res)).error ?? res.statusText);
}
