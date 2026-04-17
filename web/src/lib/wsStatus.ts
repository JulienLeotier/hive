// Shared store for the single dashboard WebSocket connection status.
// Pages that open their own WS pass { onstatus: wsStatus.set } so the
// layout can surface a single green/amber/red pulse in the sidebar.
//
// We don't care about WHICH page owns the WS — at any time there's at
// most one page visible, so overwriting the status from each onstatus
// callback is correct.

import { writable } from 'svelte/store';
import type { WSStatus } from './ws';

export const wsStatus = writable<WSStatus>('connecting');
