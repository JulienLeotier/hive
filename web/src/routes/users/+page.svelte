<script lang="ts">
	import { apiGet } from '$lib/api';
	import type { User } from '$lib/types';
	import ListScaffold from '$lib/ListScaffold.svelte';

	let users = $state<User[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			users = (await apiGet<User[]>('/api/v1/users')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});

	function roleColor(r: string): string {
		if (r === 'admin') return 'var(--err)';
		if (r === 'operator') return 'var(--accent)';
		return 'var(--text-muted)';
	}
</script>

<ListScaffold
	title="Users"
	subtitle="RBAC directory — OIDC subject → role + tenant mapping."
	{loading}
	isEmpty={users.length === 0}
	emptyText="No users registered. Add one with `hive users add <subject> <role>`."
>
	<table>
		<thead>
			<tr><th>Subject</th><th>Role</th><th>Tenant</th></tr>
		</thead>
		<tbody>
			{#each users as u (u.Subject)}
				<tr>
					<td><strong>{u.Subject}</strong></td>
					<td>
						<span class="badge" style="background:{roleColor(u.Role)}">{u.Role}</span>
					</td>
					<td><code>{u.TenantID}</code></td>
				</tr>
			{/each}
		</tbody>
	</table>
</ListScaffold>
