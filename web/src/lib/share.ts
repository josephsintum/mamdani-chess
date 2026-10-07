// The phone's share sheet, for sending a game link to a friend.

type Sharer = { share?: (data: ShareData) => Promise<void>; canShare?: (data: ShareData) => boolean };

/** Whether this browser has a share sheet that takes the link. */
export function canShare(url: string, nav: Sharer = navigator): boolean {
	return typeof nav.share === 'function' && (nav.canShare?.({ url }) ?? true);
}

/**
 * Opens the share sheet with the link and a title only: extra text makes
 * some apps send it as its own message, apart from the link's preview card.
 * 'cancelled' means the person backed out; 'failed' means there is no sheet
 * or the browser refused it (Safari does when the tap was too long ago), so
 * the caller copies the link instead.
 */
export async function shareLink(url: string, nav: Sharer = navigator): Promise<'shared' | 'cancelled' | 'failed'> {
	if (typeof nav.share !== 'function') return 'failed';
	try {
		await nav.share({ title: 'Mamdani Chess', url });
		return 'shared';
	} catch (e) {
		return e instanceof DOMException && e.name === 'AbortError' ? 'cancelled' : 'failed';
	}
}
