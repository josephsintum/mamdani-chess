// Notices for the whole app ("bagel-soho joined", a win, a lost connection).
// Pages call notify.*, never svelte-sonner directly, so the library stays
// behind this one file. Toaster.svelte (in the layout) shows them.

import { toast } from 'svelte-sonner';

export interface NotifyOptions {
	/** A second line under the message. */
	description?: string;
	/** The same id replaces the toast in place instead of adding another. */
	id?: string;
	/** How long it stays, in ms; Infinity keeps it until dismissed. */
	duration?: number;
	/** One button, e.g. Rematch. */
	action?: { label: string; onClick: () => void };
}

export const notify = {
	/** Something happened: a player joined, it's your move. */
	info: (message: string, options: NotifyOptions = {}) => toast(message, options),
	/** Good news: you won. */
	success: (message: string, options: NotifyOptions = {}) => toast.success(message, options),
	/** Something went wrong; screen readers hear it at once. */
	error: (message: string, options: NotifyOptions = {}) => toast.error(message, { ...options, important: true }),
	dismiss: (id?: string) => toast.dismiss(id)
};
