import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';

// Minimal vitest setup : jsdom pour que les composants Svelte testent
// dans un DOM léger, globals expose describe/it/expect sans import.
// On charge les matchers jest-dom via setup.ts pour toBeInTheDocument.
export default defineConfig({
	plugins: [sveltekit()],
	test: {
		environment: 'jsdom',
		globals: true,
		include: ['src/**/*.{test,spec}.{js,ts}'],
		setupFiles: ['./src/test-setup.ts']
	}
});
