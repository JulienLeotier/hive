// Dynamic route — prerender is impossible (id is only known at request
// time). SSR off so everything renders client-side.
export const prerender = false;
export const ssr = false;
