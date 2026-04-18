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
	import { confirmDialog } from './confirm';

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
			const ok = await confirmDialog({
				title: `Lancer ${skill.name} ?`,
				message: `${skill.command}\n\nCette skill est marquée dangereuse.`,
				confirmLabel: 'Lancer',
				danger: true
			});
			if (!ok) return;
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
		<span class="trigger-icon">{running ? '⏳' : '⚡'}</span>
		<span>Skill BMAD</span>
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
	/* Mêmes CSS vars que le reste du dashboard (voir +layout.svelte).
	   Dark mode supporté via data-theme="dark" → --bg-panel, --text,
	   --border, --accent, etc. sont tous définis selon le thème. */
	/* .runner doit se comporter comme un enfant flex direct de
	   .hero-actions (gap 0.5rem, flex-wrap). inline-flex + align-items
	   stretch = même hauteur de ligne que les <a class="btn ghost">
	   voisins, sinon la baseline décale de 1-2px. */
	.runner {
		position: relative;
		display: inline-flex;
		align-items: stretch;
	}
	.trigger {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.5rem 0.9rem;
		font-size: 0.82rem;
		line-height: 1.25; /* match le rendu naturel de .btn (<a>) */
		font-weight: 600;
		font-family: inherit;
		border-radius: 6px;
		border: 1px solid var(--border);
		background: var(--bg-panel);
		color: var(--text);
		cursor: pointer;
		box-sizing: border-box;
		transition: border-color 0.1s, background 0.1s, color 0.1s;
	}
	.trigger:hover:not(:disabled) {
		border-color: var(--accent);
		color: var(--accent);
	}
	.trigger:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.trigger-icon {
		font-size: 0.9rem;
		line-height: 1;
	}
	.caret {
		font-size: 0.7rem;
		opacity: 0.6;
		margin-left: 2px;
	}

	.menu {
		position: absolute;
		right: 0;
		top: calc(100% + 4px);
		min-width: 320px;
		max-height: 420px;
		overflow-y: auto;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		box-shadow: 0 10px 28px rgba(0, 0, 0, 0.18);
		z-index: 100;
		padding: 4px;
	}
	.item {
		display: block;
		width: 100%;
		text-align: left;
		background: transparent;
		border: none;
		padding: 0.6rem 0.75rem;
		border-radius: 6px;
		cursor: pointer;
		color: var(--text);
		font-family: inherit;
		transition: background 0.1s;
	}
	.item:hover:not(:disabled) {
		background: var(--bg-hover);
	}
	.item:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.item .name {
		font-weight: 600;
		font-size: 0.85rem;
		color: var(--text);
		display: flex;
		align-items: center;
		gap: 0.35rem;
	}
	.item .desc {
		font-size: 0.78rem;
		color: var(--text-muted);
		margin-top: 2px;
	}
	.item .cmd {
		margin-top: 4px;
	}
	.item .cmd code {
		font-size: 0.72rem;
		color: var(--text-muted);
		font-family: ui-monospace, 'SF Mono', Menlo, monospace;
		background: transparent;
		padding: 0;
	}
	.item.dangerous .name { color: var(--err, #dc2626); }
	.flag { font-size: 0.75rem; }

	.toast {
		position: absolute;
		right: 0;
		top: calc(100% + 4px);
		padding: 0.55rem 0.8rem;
		border-radius: 6px;
		font-size: 0.82rem;
		z-index: 101;
		min-width: 260px;
		border: 1px solid var(--border);
		background: var(--bg-panel);
		box-shadow: 0 6px 20px rgba(0, 0, 0, 0.15);
	}
	.toast.success {
		background: color-mix(in srgb, var(--ok, #16a34a) 14%, var(--bg-panel));
		border-color: color-mix(in srgb, var(--ok, #16a34a) 40%, var(--border));
		color: var(--ok, #16a34a);
	}
	.toast.error {
		background: color-mix(in srgb, var(--err, #dc2626) 14%, var(--bg-panel));
		border-color: color-mix(in srgb, var(--err, #dc2626) 40%, var(--border));
		color: var(--err, #dc2626);
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 0.5rem;
	}
	.toast button {
		background: none;
		border: none;
		font-size: 1.05rem;
		cursor: pointer;
		color: inherit;
		padding: 0 0.2rem;
	}
</style>
