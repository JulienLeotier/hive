import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
		runes: ({ filename }) => (filename.split(/[/\\]/).includes('node_modules') ? undefined : true)
	},
	kit: {
		// Static adapter : le bundle SvelteKit est pré-rendu en HTML/CSS/JS
		// statiques embarqués par le binaire Go via //go:embed. Pas de
		// runtime Node dans le produit final.
		adapter: adapter({
			pages: '../internal/dashboard/dist',
			assets: '../internal/dashboard/dist',
			fallback: 'index.html'
		})
	}
};

export default config;
