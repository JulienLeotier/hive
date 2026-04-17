<script lang="ts">
	import { apiGet, apiPost } from '$lib/api';
	import ListScaffold from '$lib/ListScaffold.svelte';

	type Invoice = {
		id: string;
		tenant_id: string;
		period_start: string;
		period_end: string;
		total_amount: number;
		task_count: number;
		currency: string;
		status: string;
		external_id?: string;
		created_at: string;
		issued_at?: string;
		paid_at?: string;
	};

	let invoices = $state<Invoice[]>([]);
	let loading = $state(true);
	let actionError = $state('');

	async function load() {
		try {
			invoices = (await apiGet<Invoice[]>('/api/v1/invoices')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
		const i = setInterval(load, 60000);
		return () => clearInterval(i);
	});

	async function issue(id: string) {
		actionError = '';
		try {
			await apiPost(`/api/v1/invoices/${encodeURIComponent(id)}/issue`, {});
			await load();
		} catch (e) {
			actionError = e instanceof Error ? e.message : String(e);
		}
	}

	async function markPaid(id: string) {
		if (!confirm('Mark this invoice as paid? Usually a gateway webhook does this — use only for offline payment flows.'))
			return;
		actionError = '';
		try {
			await apiPost(`/api/v1/invoices/${encodeURIComponent(id)}/paid`, {});
			await load();
		} catch (e) {
			actionError = e instanceof Error ? e.message : String(e);
		}
	}

	function statusColor(s: string): string {
		const map: Record<string, string> = {
			draft: 'var(--text-muted)',
			issued: 'var(--warn)',
			paid: 'var(--ok)',
			void: 'var(--err)'
		};
		return map[s] ?? 'var(--text-muted)';
	}

	function fmtDate(s?: string): string {
		if (!s) return '—';
		const d = new Date(s);
		if (isNaN(d.getTime())) return s;
		return d.toISOString().slice(0, 10);
	}

	function fmtMoney(amount: number, currency: string): string {
		try {
			return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount);
		} catch {
			return `${amount.toFixed(2)} ${currency}`;
		}
	}

	let totalIssued = $derived(
		invoices
			.filter((i) => i.status === 'issued')
			.reduce((sum, i) => sum + i.total_amount, 0)
	);
	let totalPaid = $derived(
		invoices.filter((i) => i.status === 'paid').reduce((sum, i) => sum + i.total_amount, 0)
	);
</script>

<ListScaffold
	title="Billing"
	subtitle="Monthly invoices aggregated from the costs log. A payment gateway can plug in via the billing.Gateway interface."
	{loading}
	isEmpty={invoices.length === 0}
	emptyText="No invoices yet. The monthly cron generates one at month roll-over."
>
	{#snippet controls()}
		<div class="summary">
			<span>Outstanding: <strong>{fmtMoney(totalIssued, invoices[0]?.currency ?? 'USD')}</strong></span>
			<span>Collected: <strong>{fmtMoney(totalPaid, invoices[0]?.currency ?? 'USD')}</strong></span>
		</div>
	{/snippet}

	{#if actionError}<div class="form-error">{actionError}</div>{/if}

	<table>
		<thead>
			<tr>
				<th>Period</th><th>Tenant</th><th>Tasks</th><th>Amount</th><th>Status</th><th>Issued</th><th>Paid</th><th></th>
			</tr>
		</thead>
		<tbody>
			{#each invoices as inv (inv.id)}
				<tr>
					<td>{fmtDate(inv.period_start)} → {fmtDate(inv.period_end)}</td>
					<td><code>{inv.tenant_id}</code></td>
					<td>{inv.task_count}</td>
					<td><strong>{fmtMoney(inv.total_amount, inv.currency)}</strong></td>
					<td><span class="badge" style="background:{statusColor(inv.status)}">{inv.status}</span></td>
					<td>{fmtDate(inv.issued_at)}</td>
					<td>{fmtDate(inv.paid_at)}</td>
					<td class="actions">
						{#if inv.status === 'draft'}
							<button class="btn" onclick={() => issue(inv.id)}>Issue</button>
						{/if}
						{#if inv.status === 'draft' || inv.status === 'issued'}
							<button class="btn ghost" onclick={() => markPaid(inv.id)}>Mark paid</button>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</ListScaffold>

<style>
	.summary {
		display: flex;
		gap: 1.5rem;
		margin: 1rem 0;
		font-size: 0.9rem;
		color: var(--muted);
	}
	.summary strong { color: var(--text); }
	.actions { display: flex; gap: 0.35rem; }
	.btn {
		padding: 0.2rem 0.6rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 3px;
		cursor: pointer;
		font-size: 0.8rem;
	}
	.btn.ghost {
		background: transparent;
		color: var(--text);
		border: 1px solid var(--border);
	}
	.form-error {
		padding: 0.5rem 0.75rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
		margin-bottom: 1rem;
		font-size: 0.85rem;
	}
</style>
