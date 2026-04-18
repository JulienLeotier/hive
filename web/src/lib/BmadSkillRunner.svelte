<script lang="ts">
	// BmadSkillRunner : dropdown pour lancer manuellement un skill BMAD
	// sur un projet / epic / story. Parallèle au devloop — utile pour
	// forcer un re-check (/bmad-validate-prd) ou trancher un blocage
	// (/bmad-correct-course) sans attendre le prochain tick.
	//
	// Usage :
	//   <BmadSkillRunner scope="project" projectId={p.id} />
	//   <BmadSkillRunner scope="story"   projectId={p.id} storyId={s.id} />
	//   <BmadSkillRunner scope="epic"    projectId={p.id} epicId={e.id} />

	import { onMount } from 'svelte';
	import { apiGet, apiPost } from './api';

	type Skill = {
		command: string;
		name: string;
		description: string;
		scope: 'project' | 'epic' | 'story';
		phase: string;
		dangerous?: boolean;
	};

	type Props = {
		scope: 'project' | 'epic' | 'story';
		projectId: string;
		epicId?: string;
		storyId?: string;
	};

	let { scope, projectId, epicId, storyId }: Props = $props();

	let skills = $state<Skill[]>([]);
	let open = $state(false);
	let running = $state<string | null>(null); // skill command currently launching
	let error = $state('');
	let ok = $state('');

	onMount(async () => {
		try {
			const list = await apiGet<Skill[]>(`/api/v1/bmad/skills?scope=${scope}`);
			skills = list ?? [];
		} catch {
			// Silencieux : le dropdown reste vide, l'UI affiche "aucun skill".
		}
	});

	async function run(skill: Skill) {
		if (skill.dangerous) {
			if (!confirm(`Lancer ${skill.name} (${skill.command}) ? Cette skill est marquée dangereuse.`)) return;
		}
		running = skill.command;
		error = '';
		ok = '';
		try {
			await apiPost('/api/v1/bmad/run', {
				skill: skill.command,
				project_id: projectId,
				...(epicId ? { epic_id: epicId } : {}),
				...(storyId ? { story_id: storyId } : {})
			});
			ok = `${skill.name} lancée — suis l'avancement dans Activité.`;
			setTimeout(() => (ok = ''), 4000);
			open = false;
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			running = null;
		}
	}
</script>

<div class="runner" class:open>
	<button
		type="button"
		class="trigger"
		onclick={() => (open = !open)}
		disabled={skills.length === 0 || running !== null}
		title="Lancer un skill BMAD sur ce {scope}"
	>
		{running ? '⏳' : '⚡'} Skill BMAD
		{#if skills.length > 0}
			<span class="caret">▾</span>
		{/if}
	</button>

	{#if open && skills.length > 0}
		<div class="menu" role="menu">
			{#each skills as s (s.command)}
				<button
					type="button"
					class="item"
					class:dangerous={s.dangerous}
					onclick={() => run(s)}
					disabled={running !== null}
				>
					<div class="name">
						{s.name}
						{#if s.dangerous}<span class="flag">⚠</span>{/if}
					</div>
					<div class="desc">{s.description}</div>
					<div class="cmd"><code>{s.command}</code></div>
				</button>
			{/each}
		</div>
	{/if}

	{#if ok}
		<div class="toast success" role="status">{ok}</div>
	{/if}
	{#if error}
		<div class="toast error" role="alert">
			{error}
			<button type="button" onclick={() => (error = '')}>×</button>
		</div>
	{/if}
</div>

<style>
	.runner {
		position: relative;
		display: inline-block;
	}
	.trigger {
		background: var(--bg-soft, #f5f6fa);
		border: 1px solid var(--border, #d7dae1);
		border-radius: 6px;
		padding: 6px 10px;
		font-size: 0.85rem;
		cursor: pointer;
		color: var(--text, #1a1f2c);
		font-weight: 500;
	}
	.trigger:hover:not(:disabled) {
		border-color: var(--accent, #4a64ff);
	}
	.trigger:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.caret {
		font-size: 0.7rem;
		margin-left: 4px;
	}
	.menu {
		position: absolute;
		right: 0;
		top: 100%;
		margin-top: 4px;
		min-width: 280px;
		max-height: 400px;
		overflow-y: auto;
		background: var(--bg, #fff);
		border: 1px solid var(--border, #d7dae1);
		border-radius: 8px;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
		z-index: 100;
		padding: 4px;
	}
	.item {
		display: block;
		width: 100%;
		text-align: left;
		background: transparent;
		border: none;
		padding: 8px 10px;
		border-radius: 6px;
		cursor: pointer;
		font-size: 0.85rem;
	}
	.item:hover:not(:disabled) {
		background: var(--bg-soft, #f5f6fa);
	}
	.item:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.item .name {
		font-weight: 600;
		color: var(--text, #1a1f2c);
	}
	.item .desc {
		font-size: 0.8rem;
		color: var(--muted, #6b7280);
		margin-top: 2px;
	}
	.item .cmd {
		margin-top: 4px;
	}
	.item .cmd code {
		font-size: 0.75rem;
		color: var(--muted, #6b7280);
	}
	.item.dangerous .name {
		color: var(--danger, #c23616);
	}
	.flag {
		margin-left: 4px;
	}
	.toast {
		position: absolute;
		right: 0;
		top: calc(100% + 4px);
		padding: 8px 12px;
		border-radius: 6px;
		font-size: 0.85rem;
		z-index: 101;
		min-width: 240px;
	}
	.toast.success {
		background: #e7f5ec;
		color: #1e6b3b;
		border: 1px solid #b7e0c1;
	}
	.toast.error {
		background: #fdecea;
		color: #a13023;
		border: 1px solid #f0bfb9;
		display: flex;
		justify-content: space-between;
		gap: 8px;
	}
	.toast button {
		background: none;
		border: none;
		font-size: 1.1rem;
		cursor: pointer;
		color: inherit;
	}
</style>
