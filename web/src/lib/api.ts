// Thin fetch wrapper that surfaces API failures instead of swallowing them.
//
// Before: every page had `try { await fetch(...) } catch { /* noop */ }`,
// which left the UI stuck on "Loading…" with zero signal when the backend
// was down. Now: apiGet throws on network/HTTP errors, and the apiError
// store feeds a global banner in the layout.
//
// Auth: if a hive API key is stored in localStorage (from setup or /login),
// every fetch rides an Authorization: Bearer header. No storage = dev mode
// (the server accepts unauth'd requests when no keys are configured).

import { writable } from 'svelte/store';

export const apiError = writable<string | null>(null);

const KEY_STORAGE = 'hive.api_key';

/** Returns the stored API key or null. SSR-safe (localStorage absent). */
export function getStoredKey(): string | null {
	if (typeof localStorage === 'undefined') return null;
	return localStorage.getItem(KEY_STORAGE);
}

/** Persists an API key for subsequent fetches. Call after /setup or /login. */
export function setStoredKey(key: string): void {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(KEY_STORAGE, key);
}

/** Clears the stored key — sign-out flow. */
export function clearStoredKey(): void {
	if (typeof localStorage === 'undefined') return;
	localStorage.removeItem(KEY_STORAGE);
}

function authHeaders(): Record<string, string> {
	const key = getStoredKey();
	return key ? { Authorization: `Bearer ${key}` } : {};
}

let lastFailAt = 0;

export async function apiGet<T = unknown>(url: string): Promise<T | null> {
	try {
		const r = await fetch(url, { headers: authHeaders() });
		if (r.status === 401) {
			// 401 means either no key or a stale key. Drop whatever we have
			// and let the layout redirect to /login — staying on the page
			// would just spam 401s forever as every $effect polls.
			clearStoredKey();
			throw new Error(`${r.status} ${r.statusText}`);
		}
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

// apiSend is the shared mutation path for POST/DELETE. Errors are surfaced
// to the caller (so forms can show inline feedback) and also replayed to
// the banner for resource failures that aren't form-specific (500s etc).
async function apiSend<T>(method: string, url: string, body?: unknown): Promise<T> {
	const headers: Record<string, string> = { ...authHeaders() };
	if (body !== undefined) headers['Content-Type'] = 'application/json';
	const r = await fetch(url, {
		method,
		headers,
		body: body === undefined ? undefined : JSON.stringify(body)
	});
	const text = await r.text();
	let json: { data?: T; error?: { code?: string; message?: string } } = {};
	if (text) {
		try {
			json = JSON.parse(text);
		} catch {
			throw new Error(`${r.status}: ${text.slice(0, 200)}`);
		}
	}
	if (r.status === 401) {
		clearStoredKey();
	}
	if (!r.ok || json.error) {
		const msg = json.error?.message ?? `${r.status} ${r.statusText}`;
		throw new Error(msg);
	}
	return (json.data ?? (undefined as unknown)) as T;
}

export function apiPost<T = unknown>(url: string, body: unknown): Promise<T> {
	return apiSend<T>('POST', url, body);
}

export function apiDelete<T = unknown>(url: string): Promise<T> {
	return apiSend<T>('DELETE', url);
}
