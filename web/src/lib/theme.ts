// Dark mode helper with localStorage persistence.
// Usage: import { theme, toggleTheme } from '$lib/theme'.
import { writable } from 'svelte/store';

type Theme = 'light' | 'dark';

function initialTheme(): Theme {
	if (typeof window === 'undefined') return 'light';
	const stored = localStorage.getItem('hive-theme') as Theme | null;
	if (stored) return stored;
	return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

export const theme = writable<Theme>(initialTheme());

export function toggleTheme() {
	theme.update((t) => {
		const next = t === 'dark' ? 'light' : 'dark';
		if (typeof window !== 'undefined') {
			localStorage.setItem('hive-theme', next);
			document.documentElement.setAttribute('data-theme', next);
		}
		return next;
	});
}

export function applyStoredTheme() {
	if (typeof window === 'undefined') return;
	const t = initialTheme();
	document.documentElement.setAttribute('data-theme', t);
}
