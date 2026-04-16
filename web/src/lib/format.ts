// Small formatting helpers shared across pages.

export function fmtUSD(n: number): string {
	return `$${n.toFixed(4)}`;
}

export function fmtDate(s: string): string {
	if (!s) return '—';
	return s.replace('T', ' ').slice(0, 19);
}

export function fmtRelative(s: string): string {
	if (!s) return '—';
	const then = new Date(s.replace(' ', 'T') + 'Z').getTime();
	const diff = (Date.now() - then) / 1000;
	if (diff < 60) return `${Math.floor(diff)}s ago`;
	if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
	if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
	return `${Math.floor(diff / 86400)}d ago`;
}

export function truncate(s: string, max: number): string {
	if (!s || s.length <= max) return s;
	return s.slice(0, max - 1) + '…';
}
