// Reconnecting WebSocket with exponential backoff.
//
// Why not inline in each page:
//   - Bare `ws.onclose = () => setTimeout(connect, 3000)` has no upper bound,
//     keeps spawning sockets when the server is down, and leaks timers when
//     the component unmounts. We had a real browser "Insufficient resources"
//     outage from exactly this. Centralising makes the teardown correct in
//     one place.
//
// Backoff: 500ms → 1s → 2s → 4s → … capped at 30s.

import { wsStatus } from './wsStatus';

export type WSStatus = 'connecting' | 'open' | 'closed';

export type WSHandle = {
	close: () => void;
};

export type WSOptions = {
	url: string;
	onmessage: (evt: MessageEvent) => void;
	onstatus?: (status: WSStatus) => void;
	maxBackoffMs?: number;
};

export function createReconnectingWS(opts: WSOptions): WSHandle {
	const maxBackoff = opts.maxBackoffMs ?? 30_000;
	let ws: WebSocket | null = null;
	let timer: ReturnType<typeof setTimeout> | null = null;
	let backoff = 500;
	let alive = true;

	function setStatus(s: WSStatus) {
		// Publish to the global store so the sidebar dot reflects
		// connection state regardless of which page opened the WS.
		wsStatus.set(s);
		opts.onstatus?.(s);
	}

	function connect() {
		if (!alive) return;
		setStatus('connecting');
		ws = new WebSocket(opts.url);
		ws.onopen = () => {
			backoff = 500;
			setStatus('open');
		};
		ws.onmessage = opts.onmessage;
		ws.onclose = () => {
			if (!alive) return;
			setStatus('closed');
			timer = setTimeout(connect, backoff);
			backoff = Math.min(backoff * 2, maxBackoff);
		};
		ws.onerror = () => {
			// onclose fires right after; let it handle the reconnect.
		};
	}

	connect();

	return {
		close() {
			alive = false;
			if (timer) clearTimeout(timer);
			timer = null;
			if (ws) {
				ws.onclose = null;
				ws.onerror = null;
				ws.onmessage = null;
				ws.close();
				ws = null;
			}
			// La page owner s'en va : on repasse en "connecting" pour
			// que la prochaine page qui ouvre un WS ne montre pas une
			// couleur stale.
			wsStatus.set('connecting');
		}
	};
}

export function wsURL(path: string): string {
	const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
	return `${proto}//${location.host}${path}`;
}
