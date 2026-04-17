// Dynamic route — id is only known at request time, so prerender is a no-op
// (and SvelteKit's crawler can't discover it from the build graph). Keep
// SSR disabled so the page is a pure SPA render using the stored API key.
export const prerender = false;
export const ssr = false;
