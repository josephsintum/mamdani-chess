import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		sveltekit({
			// SPA: one index.html; the Go server falls back to it for every route.
			adapter: adapter({ fallback: 'index.html' })
		})
	],
	server: {
		// `pnpm dev` on :5173 talks to `go run ./cmd/server` on :8080.
		proxy: { '/api': 'http://localhost:8080' },
		// The production build lands in web/build; rebuilding it must not
		// reload the dev page.
		watch: { ignored: ['**/build/**'] }
	}
});
