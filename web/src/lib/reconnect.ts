// How long a page waits before opening a stream again after the server
// refused it (a proxy's 502 while a deploy restarts the server, say).

/**
 * The wait before retry number `attempt` (0 for the first): a random 1 to
 * 3 s, then 2 to 6 s. The randomness matters: a deploy drops every open tab
 * at the same moment, and a fixed delay would send them all back together.
 * It never passes 6 s, so a tab is back within the 10 s a restored game
 * gives the side to move before their clock starts (RestoreGrace).
 */
export function retryDelay(attempt: number, random: () => number = Math.random): number {
	const base = Math.min(4000, 2000 * 2 ** attempt);
	return base * (0.5 + random());
}
