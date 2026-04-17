// Dynamic route — prerender is impossible (id is only known at request
// time). SSR off so the stored API key drives all fetches.
export const prerender = false;
export const ssr = false;
