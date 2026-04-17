// Thin fetch wrapper that surfaces API failures instead of swallowing them.
//
// Before: every page had `try { await fetch(...) } catch { /* noop */ }`,
// which left the UI stuck on "Loading…" with zero signal when the backend
// was down. Now: apiGet throws on network/HTTP errors, and the apiError
// store feeds a global banner in the layout.

import { writable } from 'svelte/store';

export const apiError = writable<string | null>(null);

let lastFailAt = 0;

export async function apiGet<T = unknown>(url: string): Promise<T | null> {
	try {
		const r = await fetch(url);
		if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
		const json = await r.json();
		apiError.set(null);
		return (json?.data ?? null) as T | null;
	} catch (e) {
		// Debounce: only overwrite the banner if no fresh failure in the
		// last 500ms, so a burst of parallel requests shows one message.
		const now = Date.now();
		if (now - lastFailAt > 500) {
			const msg = e instanceof Error ? e.message : String(e);
			apiError.set(`${url}: ${msg}`);
		}
		lastFailAt = now;
		throw e;
	}
}
