import { dev } from '$app/env';
import { error } from '@sveltejs/kit';

// The UI sandbox exists only in `pnpm dev`, never in production builds.
export function load() {
	if (!dev) error(404, 'Not found');
}
