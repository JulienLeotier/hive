// Small formatting helpers shared across pages. All user-facing
// strings are French — Hive is a local single-user tool built for a
// French-speaking operator, the international story is out of scope.

// parseTimestamp handles both RFC3339 ("2026-04-17T21:22:20Z", from Go
// json encoding) and SQLite's default datetime ("2026-04-17 21:22:20",
// from the intake store which hands back the raw column). The previous
// implementation blindly appended 'Z' which produced "…ZZ" on RFC3339
// input → NaN → "NaNd ago" in the dashboard.
function parseTimestamp(s: string): number {
	if (!s) return NaN;
	if (s.includes('T') || s.endsWith('Z')) return new Date(s).getTime();
	return new Date(s.replace(' ', 'T') + 'Z').getTime();
}

export function fmtRelative(s: string): string {
	if (!s) return '—';
	const then = parseTimestamp(s);
	if (isNaN(then)) return '—';
	const diff = (Date.now() - then) / 1000;
	if (diff < 5) return "à l'instant";
	if (diff < 60) return `il y a ${Math.floor(diff)} s`;
	if (diff < 3600) return `il y a ${Math.floor(diff / 60)} min`;
	if (diff < 86400) return `il y a ${Math.floor(diff / 3600)} h`;
	return `il y a ${Math.floor(diff / 86400)} j`;
}
