// See https://svelte.dev/docs/kit/types#app.d.ts
declare global {
	namespace App {
		interface PageState {
			/** The game page was reached from quick match's opponent-found screen. */
			matched?: boolean;
		}
	}
}

export {};
