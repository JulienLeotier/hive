import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

// Backend port — must match internal/config/config.go default (8233).
const BACKEND = process.env.HIVE_BACKEND ?? 'http://localhost:8233';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		port: 5173,
		strictPort: true,
		proxy: {
			'/api': { target: BACKEND, changeOrigin: true },
			'/ws': { target: BACKEND, ws: true, changeOrigin: true }
		}
	}
});
