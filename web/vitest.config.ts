import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';

// Minimal vitest setup : jsdom pour que les composants Svelte testent
// dans un DOM léger. `resolve.conditions` force Vite à servir la
// version browser de Svelte (sinon Svelte 5 résout en server-side et
// onMount throw "lifecycle_function_unavailable").
export default defineConfig({
	plugins: [sveltekit()],
	resolve: {
		conditions: ['browser']
	},
	test: {
		environment: 'jsdom',
		globals: true,
		include: ['src/**/*.{test,spec}.{js,ts}'],
		setupFiles: ['./src/test-setup.ts']
	}
});
