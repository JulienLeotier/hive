<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';
	import type { DialogThread } from '$lib/types';
	import ListScaffold from '$lib/ListScaffold.svelte';

	let threads = $state<DialogThread[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			threads = (await apiGet<DialogThread[]>('/api/v1/dialogs')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
		const i = setInterval(load, 5000);
		return () => clearInterval(i);
	});
</script>

<ListScaffold
	title="Dialogs"
	subtitle="Inter-agent conversation threads."
	{loading}
	isEmpty={threads.length === 0}
	emptyText="No dialog threads yet."
>
	<table>
		<thead>
			<tr><th>Topic</th><th>Initiator → Participant</th><th>Messages</th><th>Status</th><th>Started</th></tr>
		</thead>
		<tbody>
			{#each threads as t (t.id)}
				<tr>
					<td><strong>{t.topic}</strong></td>
					<td>{t.initiator} → {t.participant}</td>
					<td>{t.message_count}</td>
					<td>
						<span
							class="badge"
							style="background:{t.status === 'active' ? 'var(--ok)' : 'var(--text-muted)'}"
							>{t.status}</span
						>
					</td>
					<td>{fmtRelative(t.created_at)}</td>
				</tr>
			{/each}
		</tbody>
	</table>
</ListScaffold>
