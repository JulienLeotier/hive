<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { apiGet, apiPost } from '$lib/api';
	import { fmtRelative } from '$lib/format';
	import { createReconnectingWS, wsURL } from '$lib/ws';
	import BmadSkillRunner from '$lib/BmadSkillRunner.svelte';

	type IntakeMessage = {
		id: number;
		author: string;
		content: string;
		created_at: string;
	};
	type Conversation = {
		id: string;
		project_id: string;
		role: string;
		status: string;
		messages?: IntakeMessage[];
	};

	type AcceptanceCriterion = {
		id: number;
		text: string;
		passed: boolean;
		ordering: number;
	};
	type Story = {
		id: string;
		title: string;
		description?: string;
		status: string;
		iterations: number;
		agent_id?: string;
		branch?: string;
		pr_url?: string;
		acceptance_criteria?: AcceptanceCriterion[];
		last_review_verdict?: string;
		last_review_feedback?: string;
	};
	type Epic = {
		id: string;
		title: string;
		description?: string;
		status: string;
		stories?: Story[];
	};
	type Project = {
		id: string;
		name: string;
		idea: string;
		prd?: string;
		workdir?: string;
		bmad_output_path?: string;
		repo_path?: string;
		repo_url?: string;
		is_existing?: boolean;
		total_cost_usd?: number;
		failure_stage?: string;
		failure_error?: string;
		status: string;
		created_at: string;
		updated_at: string;
		epics?: Epic[];
	};

	type PhaseStep = {
		id: number;
		phase: string;
		command: string;
		started_at: string;
		finished_at?: string;
		status: string; // running | done | failed
		input_tokens: number;
		output_tokens: number;
		cost_usd: number;
		reply_preview?: string;
		error?: string;
	};

	type ProjectEvent = {
		id: number;
		type: string;
		source: string;
		payload: string;
		created_at: string;
		_parsed?: Record<string, unknown>;
	};

	let project = $state<Project | null>(null);
	let loading = $state(true);
	let activity = $state<ProjectEvent[]>([]);
	let phases = $state<PhaseStep[]>([]);
	let cancelling = $state(false);
	let retryingBuild = $state(false);
	let actionError = $state('');

	// Console drawer : quand l'opérateur clique sur un phase step,
	// on fetch le full reply (reply_full) via /api/v1/phases/{id} et
	// on l'affiche dans un panneau latéral. Distinct de reply_preview
	// (600 premiers chars) qui reste utilisé pour la liste compacte.
	type PhaseStepFull = PhaseStep & {
		project_id: string;
		reply_full?: string;
	};
	let consoleStep = $state<PhaseStepFull | null>(null);
	let consoleLoading = $state(false);

	async function openConsole(stepID: number) {
		consoleLoading = true;
		try {
			consoleStep = (await apiGet<PhaseStepFull>(`/api/v1/phases/${stepID}`)) ?? null;
		} catch {
			consoleStep = null;
		} finally {
			consoleLoading = false;
		}
	}

	function closeConsole() {
		consoleStep = null;
	}

	async function loadPhases() {
		const id = $page.params.id ?? '';
		if (!id) return;
		try {
			phases = (await apiGet<PhaseStep[]>(`/api/v1/projects/${encodeURIComponent(id)}/phases`)) ?? [];
		} catch {
			/* banner */
		}
	}

	async function cancelRun() {
		const id = $page.params.id ?? '';
		if (!confirm('Annuler le build BMAD en cours ? Les skills Claude actuellement en vol seront tuées.')) return;
		cancelling = true;
		actionError = '';
		try {
			await apiPost(`/api/v1/projects/${encodeURIComponent(id)}/cancel`, {});
			await load();
		} catch (e) {
			actionError = e instanceof Error ? e.message : String(e);
		} finally {
			cancelling = false;
		}
	}

	let runningRetro = $state(false);

	async function runRetrospective() {
		const id = $page.params.id ?? '';
		if (!confirm('Lancer une rétrospective BMAD sur ce projet ? /bmad-agent-dev + /bmad-retrospective tourneront en tâche de fond.')) return;
		runningRetro = true;
		actionError = '';
		try {
			await apiPost(`/api/v1/projects/${encodeURIComponent(id)}/retrospective`, {});
			await loadPhases();
		} catch (e) {
			actionError = e instanceof Error ? e.message : String(e);
		} finally {
			runningRetro = false;
		}
	}

	async function retryBuild() {
		const id = $page.params.id ?? '';
		retryingBuild = true;
		actionError = '';
		try {
			await apiPost(`/api/v1/projects/${encodeURIComponent(id)}/retry-architect`, {});
			await load();
			await loadPhases();
		} catch (e) {
			actionError = e instanceof Error ? e.message : String(e);
		} finally {
			retryingBuild = false;
		}
	}

	async function resumeBuild() {
		const id = $page.params.id ?? '';
		// Calcule l'index du premier step non-complété. phases est en
		// ORDER BY id DESC, on prend donc la séquence la plus récente,
		// on la remet en ordre chronologique et on compte les 'done'.
		const ordered = [...phases].reverse();
		const doneCount = ordered.filter((p) => p.status === 'done').length;
		if (doneCount === 0) {
			await retryBuild();
			return;
		}
		retryingBuild = true;
		actionError = '';
		try {
			await apiPost(
				`/api/v1/projects/${encodeURIComponent(id)}/retry-architect?from_step=${doneCount}`,
				{}
			);
			await load();
			await loadPhases();
		} catch (e) {
			actionError = e instanceof Error ? e.message : String(e);
		} finally {
			retryingBuild = false;
		}
	}

	function parseEvt(e: ProjectEvent): ProjectEvent {
		if (e._parsed) return e;
		try {
			e._parsed = JSON.parse(e.payload ?? '{}') as Record<string, unknown>;
		} catch {
			e._parsed = {};
		}
		return e;
	}

	function evtBelongsToProject(e: ProjectEvent, pid: string): boolean {
		parseEvt(e);
		return (e._parsed?.project_id as string | undefined) === pid;
	}

	// Intake state
	let conversation = $state<Conversation | null>(null);
	let intakeLoading = $state(false);
	let replyDraft = $state('');
	let sending = $state(false);
	let intakeDone = $state(false);
	let finalizing = $state(false);
	let intakeError = $state('');
	let editingMessageID = $state<number | null>(null);
	let editDraft = $state('');
	let savingEdit = $state(false);

	function startEdit(m: IntakeMessage) {
		editingMessageID = m.id;
		editDraft = m.content;
		intakeError = '';
	}

	function cancelEdit() {
		editingMessageID = null;
		editDraft = '';
	}

	async function saveEdit() {
		const id = $page.params.id ?? '';
		if (editingMessageID === null || !editDraft.trim()) return;
		savingEdit = true;
		intakeError = '';
		try {
			const resp = await fetch(`/api/v1/projects/${encodeURIComponent(id)}/intake/message`, {
				method: 'PATCH',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ message_id: editingMessageID, content: editDraft })
			});
			if (!resp.ok) throw new Error((await resp.json()).error?.message ?? resp.statusText);
			const json = await resp.json();
			conversation = json.data as Conversation;
			editingMessageID = null;
			editDraft = '';
		} catch (e) {
			intakeError = e instanceof Error ? e.message : String(e);
		} finally {
			savingEdit = false;
		}
	}

	async function load() {
		const id = $page.params.id ?? '';
		if (!id) return;
		try {
			project = await apiGet<Project>(`/api/v1/projects/${encodeURIComponent(id)}`);
		} catch {
			/* banner */
		} finally {
			loading = false;
		}
	}

	async function loadActivity() {
		const id = $page.params.id ?? '';
		if (!id) return;
		try {
			const raw = (await apiGet<ProjectEvent[]>(`/api/v1/events?limit=200`)) ?? [];
			activity = raw.filter((e) => evtBelongsToProject(e, id));
		} catch {
			/* banner */
		}
	}

	async function loadIntake() {
		const id = $page.params.id ?? '';
		if (!id || !project || project.status !== 'draft') return;
		intakeLoading = true;
		try {
			conversation = await apiGet<Conversation>(
				`/api/v1/projects/${encodeURIComponent(id)}/intake`
			);
		} catch (e) {
			intakeError = e instanceof Error ? e.message : String(e);
		} finally {
			intakeLoading = false;
		}
	}

	async function sendReply() {
		const id = $page.params.id ?? '';
		if (!id || !replyDraft.trim()) return;
		intakeError = '';
		sending = true;
		try {
			const resp = (await apiPost(
				`/api/v1/projects/${encodeURIComponent(id)}/intake/messages`,
				{ content: replyDraft }
			)) as { conversation: Conversation; done: boolean };
			conversation = resp.conversation;
			intakeDone = resp.done;
			replyDraft = '';
		} catch (e) {
			intakeError = e instanceof Error ? e.message : String(e);
		} finally {
			sending = false;
		}
	}

	async function finalizePRD() {
		const id = $page.params.id ?? '';
		if (!id) return;
		intakeError = '';
		finalizing = true;
		try {
			await apiPost(`/api/v1/projects/${encodeURIComponent(id)}/intake/finalize`, {});
			await load();
		} catch (e) {
			intakeError = e instanceof Error ? e.message : String(e);
		} finally {
			finalizing = false;
		}
	}

	// Adaptive polling: a project that's actively building can move in
	// seconds (devloop ticks + Claude Code finishes a story), so we poll
	// fast. Draft/shipped projects barely change; poll slowly so we're
	// not hammering SQLite for no reason.
	//
	// IMPORTANT : ce $effect NE doit PAS lire `project` directement,
	// sinon il se ré-exécute à chaque rafraîchissement (project est
	// ré-assigné par load()), créant une boucle infinie → 429 en
	// cascade. On passe par un $derived qui retourne un primitive
	// (number), qui n'est comparé par valeur : status 'building' →
	// 'building' → pas de re-run.
	let pollIntervalMs = $derived.by(() => {
		const fast = project?.status === 'building' || project?.status === 'review' || project?.status === 'planning';
		return fast ? 2000 : 10000;
	});

	// Chargement initial (one-shot, hors boucle réactive).
	onMount(() => {
		load();
		loadActivity();
		loadPhases();
	});

	// Interval polling — ré-armé uniquement quand pollIntervalMs change.
	$effect(() => {
		const ms = pollIntervalMs;
		const i = setInterval(() => {
			load();
			loadActivity();
			loadPhases();
		}, ms);
		return () => clearInterval(i);
	});

	// WebSocket subscription: the devloop emits story.* and project.shipped
	// events as it works. Reacting to them eliminates polling lag — the
	// UI updates the moment a story flips dev → review → done.
	$effect(() => {
		const ws = createReconnectingWS({
			url: wsURL('/ws'),
			onmessage: (msg) => {
				try {
					const evt = JSON.parse(msg.data) as ProjectEvent;
					if (!evt.type) return;
					const pid = $page.params.id ?? '';
					if (!evtBelongsToProject(evt, pid)) return;
					// Prepend to activity feed, dedupe by id, cap at 200.
					activity = [evt, ...activity.filter((x) => x.id !== evt.id)].slice(0, 200);
					// Story/project status changes should also re-fetch the tree.
					if (
						evt.type.startsWith('story.') ||
						evt.type === 'project.shipped' ||
						evt.type === 'project.architect_done' ||
						evt.type === 'project.architect_failed' ||
						evt.type === 'project.cancelled' ||
						evt.type === 'project.iteration_started' ||
						evt.type === 'project.iteration_done' ||
						evt.type === 'project.iteration_failed'
					) load();
					// BMAD step events : refresh phases panel.
					if (evt.type === 'project.bmad_step_started' || evt.type === 'project.bmad_step_finished') {
						loadPhases();
					}
				} catch {
					/* ignore non-JSON frames */
				}
			}
		});
		return () => ws.close();
	});

	// Load the intake conversation whenever we're on a draft project.
	$effect(() => {
		if (project && project.status === 'draft' && !conversation) {
			loadIntake();
		}
	});

	function isActive(s: string): boolean {
		return s === 'dev' || s === 'review' || s === 'in_progress';
	}

	let retrying = $state<Record<string, boolean>>({});

	let editingPRD = $state(false);
	let prdDraft = $state('');
	let savingPRD = $state(false);
	let prdExpanded = $state(false);
	let regenerating = $state(false);
	let prdError = $state('');

	// Onglet actif. Les sections du détail projet sont groupées en
	// tabs pour éviter la page-à-rallonge. Default = overview.
	type Tab = 'overview' | 'stories' | 'prd' | 'activity';
	let activeTab = $state<Tab>('overview');

	function startEditPRD() {
		prdDraft = project?.prd ?? '';
		editingPRD = true;
		prdError = '';
	}

	async function savePRD() {
		const id = $page.params.id ?? '';
		if (!id) return;
		savingPRD = true;
		prdError = '';
		try {
			await fetch(`/api/v1/projects/${encodeURIComponent(id)}/prd`, {
				method: 'PATCH',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ prd: prdDraft })
			}).then(async (r) => {
				if (!r.ok) throw new Error((await r.json()).error?.message ?? r.statusText);
			});
			editingPRD = false;
			await load();
		} catch (e) {
			prdError = e instanceof Error ? e.message : String(e);
		} finally {
			savingPRD = false;
		}
	}

	async function regeneratePlan() {
		const id = $page.params.id ?? '';
		if (!id) return;
		if (!confirm("Régénérer le plan ? L'arbre epics/stories actuel sera effacé et l'Architecte le reconstruira depuis le PRD. Autorisé uniquement avant que le dev n'ait commencé.")) return;
		regenerating = true;
		prdError = '';
		try {
			await apiPost(`/api/v1/projects/${encodeURIComponent(id)}/regenerate-plan`, {});
			await load();
		} catch (e) {
			prdError = e instanceof Error ? e.message : String(e);
		} finally {
			regenerating = false;
		}
	}

	async function retryStory(storyID: string) {
		const id = $page.params.id ?? '';
		if (!id) return;
		retrying = { ...retrying, [storyID]: true };
		try {
			await apiPost(
				`/api/v1/projects/${encodeURIComponent(id)}/stories/${encodeURIComponent(storyID)}/retry`,
				{}
			);
			await load();
		} catch {
			/* banner */
		} finally {
			retrying = { ...retrying, [storyID]: false };
		}
	}

	function eventColor(t: string): string {
		if (t === 'project.shipped' || t === 'story.reviewed') return 'var(--ok)';
		if (t === 'story.blocked' || t.endsWith('.failed')) return 'var(--err)';
		if (t === 'story.review_failed') return 'var(--warn)';
		if (t.startsWith('story.')) return 'var(--accent)';
		return 'var(--muted)';
	}

	function statusColor(s: string): string {
		const map: Record<string, string> = {
			draft: 'var(--text-muted)',
			planning: 'var(--accent)',
			building: 'var(--warn)',
			review: 'var(--warn)',
			shipped: 'var(--ok)',
			failed: 'var(--err)',
			pending: 'var(--text-muted)',
			in_progress: 'var(--warn)',
			done: 'var(--ok)',
			blocked: 'var(--err)',
			dev: 'var(--warn)',
			qa: 'var(--accent)'
		};
		return map[s] ?? 'var(--text-muted)';
	}

	let totalStories = $derived(
		(project?.epics ?? []).reduce((n, e) => n + (e.stories?.length ?? 0), 0)
	);
	let doneStories = $derived(
		(project?.epics ?? []).reduce(
			(n, e) => n + (e.stories?.filter((s) => s.status === 'done').length ?? 0),
			0
		)
	);
	let totalACs = $derived(
		(project?.epics ?? []).reduce(
			(n, e) =>
				n +
				(e.stories?.reduce((m, s) => m + (s.acceptance_criteria?.length ?? 0), 0) ?? 0),
			0
		)
	);
	let passedACs = $derived(
		(project?.epics ?? []).reduce(
			(n, e) =>
				n +
				(e.stories?.reduce(
					(m, s) => m + (s.acceptance_criteria?.filter((ac) => ac.passed).length ?? 0),
					0
				) ?? 0),
			0
		)
	);
</script>

<main>
	<a class="back" href="/projects">← tous les projets</a>

	{#if loading}
		<p class="empty">Chargement du projet…</p>
	{:else if !project}
		<p class="empty">Projet introuvable.</p>
	{:else}
		<header class="hero">
			<div class="hero-top">
				<span class="badge status" style="background:{statusColor(project.status)}">{project.status}</span>
				{#if project.is_existing}
					<span class="chip brownfield" title="Projet basé sur un repo existant — BMAD tourne en mode brownfield">
						🏗 brownfield
					</span>
				{/if}
				{#if (project.total_cost_usd ?? 0) > 0}
					<span class="chip cost" title="Cumul tokens Claude consommés">
						${(project.total_cost_usd ?? 0).toFixed(2)}
					</span>
				{/if}
				<span class="hero-time">mis à jour {fmtRelative(project.updated_at)}</span>
			</div>
			<h1 class="hero-name">{project.name}</h1>
			<p class="hero-idea">{project.idea}</p>

			<div class="hero-actions">
				<a class="btn ghost" href="/projects/{project.id}/files">
					<span class="btn-icon">📁</span> Fichiers
				</a>
				{#if project.repo_url}
					<a class="btn ghost" href={project.repo_url} target="_blank" rel="noopener">
						<span class="btn-icon">↗</span> Repo GitHub
					</a>
				{/if}
				<a class="btn ghost"
					href={`/api/v1/projects/${encodeURIComponent(project.id)}/export`}
					download
					title="Télécharge un .tar.gz : workdir + PRD + epics + historique phases + intake">
					<span class="btn-icon">↓</span> Export .tar.gz
				</a>
				{#if project.status === 'shipped' || project.status === 'building'}
					<a class="btn accent" href="/projects/{project.id}/iterate">
						<span class="btn-icon">➕</span> Nouvelle itération
					</a>
					<button type="button" class="btn ghost" onclick={runRetrospective} disabled={runningRetro}>
						<span class="btn-icon">📝</span>
						{runningRetro ? 'Rétro en cours…' : 'Rétrospective'}
					</button>
				{/if}
				{#if project.status !== 'draft'}
					<BmadSkillRunner scope="project" projectId={project.id} />
				{/if}
			</div>

			{#if project.workdir || project.repo_path || project.bmad_output_path}
				<details class="hero-details">
					<summary>Détails techniques</summary>
					<dl class="refs">
						{#if project.workdir}
							<dt>Répertoire</dt><dd><code>{project.workdir}</code></dd>
						{/if}
						{#if project.repo_path}
							<dt>Repo existant</dt><dd><code>{project.repo_path}</code></dd>
						{/if}
						{#if project.bmad_output_path}
							<dt>Sortie BMAD</dt><dd><code>{project.bmad_output_path}</code></dd>
						{/if}
						<dt>Project ID</dt><dd><code>{project.id}</code></dd>
					</dl>
				</details>
			{/if}
		</header>

		{#if project.failure_stage}
			<div class="fail-banner">
				<div>
					<strong>Build en échec</strong> — étape <code>{project.failure_stage}</code>
					{#if project.failure_error}
						<pre class="fail-error">{project.failure_error}</pre>
					{/if}
				</div>
				<button
					type="button"
					class="retry-btn"
					onclick={resumeBuild}
					disabled={retryingBuild}
					title="Saute les steps déjà réussis, reprend au premier non-terminé">
					{retryingBuild ? 'Reprise…' : '↻ Reprendre au step suivant'}
				</button>
				<button
					type="button"
					class="retry-btn ghost"
					onclick={retryBuild}
					disabled={retryingBuild}
					title="Relance la séquence depuis le début (coûteux)">
					{retryingBuild ? 'Relance…' : '↻ Relancer BMAD'}
				</button>
			</div>
		{/if}

		{#if actionError}
			<div class="err">{actionError}</div>
		{/if}

		{#if project.status !== 'draft'}
			<nav class="tabs-nav" aria-label="Navigation du projet">
				<button type="button"
					class="tab"
					class:active={activeTab === 'overview'}
					onclick={() => (activeTab = 'overview')}>
					<span class="tab-icon">⊞</span> Vue d'ensemble
				</button>
				<button type="button"
					class="tab"
					class:active={activeTab === 'stories'}
					onclick={() => (activeTab = 'stories')}>
					<span class="tab-icon">▦</span> Stories
					{#if project.epics && project.epics.length > 0}
						<span class="tab-badge">{totalStories}</span>
					{/if}
				</button>
				{#if project.prd}
					<button type="button"
						class="tab"
						class:active={activeTab === 'prd'}
						onclick={() => (activeTab = 'prd')}>
						<span class="tab-icon">📄</span> PRD
					</button>
				{/if}
				<button type="button"
					class="tab"
					class:active={activeTab === 'activity'}
					onclick={() => (activeTab = 'activity')}>
					<span class="tab-icon">◈</span> Activité
					{#if activity.length > 0}
						<span class="tab-badge">{activity.length}</span>
					{/if}
				</button>
			</nav>
		{/if}

		{#if activeTab === 'overview' && project.status !== 'draft'}
		{#if (project.status === 'planning' || project.status === 'building') && phases.length > 0}
			{@const running = phases.find((s) => s.status === 'running')}
			{@const done = phases.filter((s) => s.status === 'done').length}
			{@const total = phases.length}
			{@const totalCost = phases.reduce((a, s) => a + (s.cost_usd ?? 0), 0)}
			<section class="phases">
				<div class="phases-head">
					<div class="phases-title">
						<span class="phases-icon">⚙</span>
						<h2>Pipeline BMAD</h2>
						{#if running}
							<span class="running-pill">
								<span class="live-dot"></span>
								en cours
							</span>
						{/if}
					</div>
					{#if running || project.status === 'planning'}
						<button type="button" class="btn danger sm" onclick={cancelRun} disabled={cancelling}>
							{cancelling ? 'Annulation…' : '✕ Annuler'}
						</button>
					{/if}
				</div>

				<div class="phases-summary">
					<div class="sum-item">
						<span class="sum-num">{done}<span class="sum-total">/{total}</span></span>
						<span class="sum-label">étapes terminées</span>
					</div>
					<div class="sum-item">
						<span class="sum-num mono">${totalCost.toFixed(2)}</span>
						<span class="sum-label">Claude cumulé</span>
					</div>
					<div class="sum-item flex-fill">
						<span class="sum-num">{Math.round((done / Math.max(total, 1)) * 100)}<span class="sum-total">%</span></span>
						<span class="sum-label">avancement</span>
					</div>
				</div>

				{#if total > 0}
					<div class="progress-bar">
						<div class="progress-fill" style="width:{(done / Math.max(total, 1)) * 100}%"></div>
					</div>
				{/if}

				<ol class="phase-list">
					{#each phases.slice(0, 15) as s (s.id)}
						<li class="phase-item {s.status}">
							<button
								type="button"
								class="phase-row"
								onclick={() => openConsole(s.id)}
								title="Voir la console Claude pour ce step"
							>
								<span class="phase-status">
									{#if s.status === 'running'}
										<span class="live-dot big"></span>
									{:else if s.status === 'done'}
										✓
									{:else if s.status === 'failed'}
										✕
									{:else}
										·
									{/if}
								</span>
								<code class="phase-cmd">{s.command}</code>
								<span class="phase-phase">{s.phase}</span>
								<span class="phase-meta">
									{#if s.cost_usd > 0}<span>${s.cost_usd.toFixed(3)}</span>{/if}
									{#if s.input_tokens > 0 || s.output_tokens > 0}
										<span class="tokens">{Math.round((s.input_tokens + s.output_tokens) / 1000)}k tok</span>
									{/if}
									<span class="phase-time">{fmtRelative(s.started_at)}</span>
								</span>
								<span class="phase-chev">›</span>
							</button>
						</li>
					{/each}
				</ol>
			</section>
		{/if}

		<section class="progress">
			<h2>Avancement</h2>
			{#if totalACs > 0}
				<div class="bar" aria-label="avancement des critères d'acceptation">
					<div
						class="bar-fill"
						class:shipped={project.status === 'shipped'}
						style="width:{Math.round((passedACs / totalACs) * 100)}%"
					></div>
					<span class="bar-label">
						{passedACs}/{totalACs} ACs · {Math.round((passedACs / totalACs) * 100)}%
					</span>
				</div>
			{/if}
			<div class="metrics">
				<div><strong>{doneStories}/{totalStories}</strong><span>stories terminées</span></div>
				<div><strong>{passedACs}/{totalACs}</strong><span>critères validés</span></div>
				<div><strong>{project.epics?.length ?? 0}</strong><span>epics</span></div>
			</div>
		</section>
		{/if}
		<!-- /overview tab -->

		{#if project.status === 'draft'}
			<section class="intake">
				<h2>Intake — discussion avec l'agent PM</h2>
				{#if project.is_existing}
					<p class="brownfield-note">
						🏗 <strong>Projet brownfield</strong> — Hive tourne sur un repo existant.
						BMAD va lancer <code>/bmad-document-project</code> pour comprendre la base de code,
						puis <code>/bmad-edit-prd</code> pour étendre le PRD avec ta feature.
					</p>
				{/if}
				{#if intakeLoading && !conversation}
					<p class="empty">Démarrage de la conversation…</p>
				{:else if conversation}
					<div class="chat">
						{#each conversation.messages ?? [] as m (m.id)}
							<div class="bubble" class:user={m.author === 'user'} class:agent={m.author !== 'user'}>
								<div class="bubble-head">
									<strong>{m.author === 'user' ? 'Toi' : 'Agent PM'}</strong>
									<span class="muted">{fmtRelative(m.created_at)}</span>
									{#if m.author === 'user' && editingMessageID !== m.id}
										<button type="button" class="edit-msg"
											onclick={() => startEdit(m)}
											title="Modifier ce message">✎</button>
									{/if}
								</div>
								{#if editingMessageID === m.id}
									<textarea
										class="edit-area"
										rows="3"
										bind:value={editDraft}
										disabled={savingEdit}
									></textarea>
									<div class="edit-actions">
										<button type="button"
											onclick={saveEdit}
											disabled={savingEdit || !editDraft.trim()}>
											{savingEdit ? 'Enregistrement…' : 'Enregistrer'}
										</button>
										<button type="button" onclick={cancelEdit} disabled={savingEdit}>
											Annuler
										</button>
									</div>
								{:else}
									<div class="bubble-content">{m.content}</div>
								{/if}
							</div>
						{/each}
					</div>

					{#if intakeError}<div class="err">{intakeError}</div>{/if}

					{#if conversation.status === 'finalized'}
						<p class="done-note">
							Conversation finalisée. Le PRD a été écrit — si le projet ne
							démarre pas, clique sur <strong>Finaliser le PRD</strong>.
						</p>
					{:else}
						<form class="reply-form" onsubmit={(e) => { e.preventDefault(); sendReply(); }}>
							<textarea
								bind:value={replyDraft}
								rows="3"
								placeholder="Ta réponse…"
								disabled={sending}
							></textarea>
							<div class="reply-actions">
								<button type="submit" disabled={sending || !replyDraft.trim()}>
									{sending ? 'Envoi…' : 'Envoyer'}
								</button>
								{#if intakeDone}
									<button
										type="button"
										class="primary"
										onclick={finalizePRD}
										disabled={finalizing}>
										{finalizing ? 'Rédaction du PRD…' : '✓ Finaliser le PRD et lancer la construction'}
									</button>
									<span class="muted">Le PM a assez d'info pour rédiger le PRD. Continue la discussion pour affiner, ou finalise quand tu es prêt.</span>
								{/if}
							</div>
						</form>
					{/if}
				{/if}
			</section>
		{/if}
		<!-- /draft intake -->

		{#if activeTab === 'prd' && project.prd && project.status !== 'draft'}
			{@const prdLines = project.prd.split('\n').length}
			{@const prdBytes = new Blob([project.prd]).size}
			<section class="panel">
				<div class="prd-head">
					<h3>
						PRD
						<span class="prd-meta">{prdLines} lignes · {(prdBytes / 1024).toFixed(1)} KB</span>
					</h3>
					<div class="prd-actions">
						{#if editingPRD}
							<button type="button" onclick={savePRD} disabled={savingPRD || !prdDraft.trim()}>
								{savingPRD ? 'Enregistrement…' : 'Enregistrer'}
							</button>
							<button type="button" onclick={() => (editingPRD = false)} disabled={savingPRD}>
								Annuler
							</button>
						{:else}
							<button type="button" onclick={startEditPRD}>✎ Éditer</button>
							{#if project.status !== 'shipped'}
								<button
									type="button"
									class="warn"
									onclick={regeneratePlan}
									disabled={regenerating}
									title="Efface le plan actuel et demande à l'Architecte de le reconstruire depuis le PRD. Seulement avant que le dev n'ait commencé."
								>
									{regenerating ? 'Régénération…' : '↻ Régénérer'}
								</button>
							{/if}
						{/if}
					</div>
				</div>
				{#if prdError}<div class="err">{prdError}</div>{/if}
				{#if editingPRD}
					<textarea class="prd-editor" rows="18" bind:value={prdDraft}></textarea>
				{:else}
					<pre class="prd">{project.prd}</pre>
				{/if}
			</section>
		{/if}

		{#if activeTab === 'activity' && activity.length > 0 && project.status !== 'draft'}
			<section class="activity">
				<h2>Activité <span class="count">{activity.length}</span></h2>
				<ul class="feed">
					{#each activity.slice(0, 50) as e (e.id)}
						{@const parsed = (parseEvt(e)._parsed ?? {}) as Record<string, unknown>}
						<li>
							<span class="t" style="color:{eventColor(e.type)}">{e.type}</span>
							<span class="muted">{fmtRelative(e.created_at)}</span>
							{#if typeof parsed.story === 'string'}
								<span class="story-ref">{parsed.story}</span>
							{/if}
							{#if typeof parsed.feedback === 'string' && parsed.feedback}
								<span class="feedback">{parsed.feedback}</span>
							{/if}
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		{#if activeTab === 'stories' && project.status !== 'draft'}
		<section class="tree">
			<h2>Découpage du travail</h2>
			{#if !project.epics || project.epics.length === 0}
				{#if project.status === 'planning'}
					<p class="planning">
						<span class="spinner"></span>
						L'Architecte BMAD décompose le PRD en epics et stories… ça peut prendre quelques minutes.
					</p>
				{:else}
					<p class="empty">Pas encore d'epics. L'agent Architecte les produira une fois le PRD verrouillé.</p>
				{/if}
			{:else}
				{#each project.epics as epic (epic.id)}
					<div class="epic">
						<header>
							<h3>{epic.title}</h3>
							<span class="badge" style="background:{statusColor(epic.status)}">{epic.status}</span>
						</header>
						{#if epic.description}
							<p class="desc">{epic.description}</p>
						{/if}
						{#if epic.stories && epic.stories.length > 0}
							<ul class="stories">
								{#each epic.stories as story (story.id)}
									<li class:active={isActive(story.status)}>
										<div class="story-head">
											<strong>{story.title}</strong>
											<span class="badge" style="background:{statusColor(story.status)}">{story.status}</span>
											{#if story.iterations > 0}
												<span class="muted">· {story.iterations} itération{story.iterations > 1 ? 's' : ''}</span>
											{/if}
											{#if story.pr_url}
												<a class="pr-link" href={story.pr_url} target="_blank" rel="noopener" title="Pull request ouverte par BMAD">
													🔀 PR
												</a>
											{:else if story.branch}
												<code class="branch-tag" title="Branche feature créée par BMAD">{story.branch}</code>
											{/if}
											{#if story.status === 'blocked'}
												<button
													type="button"
													class="retry"
													onclick={() => retryStory(story.id)}
													disabled={retrying[story.id]}
													title="Remet le compteur d'itérations à zéro et relance la story dans le dev loop"
												>
													{retrying[story.id] ? 'Relance…' : '↻ Réessayer'}
												</button>
											{/if}
											<BmadSkillRunner scope="story" projectId={project.id} storyId={story.id} />
										</div>
										{#if story.last_review_feedback && story.status !== 'done' && story.last_review_verdict !== 'pass'}
											<div
												class="review-feedback"
												class:blocked={story.status === 'blocked'}
												title="Dernier verdict du relecteur : {story.last_review_verdict}"
											>
												<span class="review-label">Relecteur :</span>
												{story.last_review_feedback}
											</div>
										{/if}
										{#if story.acceptance_criteria && story.acceptance_criteria.length > 0}
											<ul class="acs">
												{#each story.acceptance_criteria as ac (ac.id)}
													<li class:passed={ac.passed}>
														<span class="check">{ac.passed ? '✓' : '○'}</span>
														{ac.text}
													</li>
												{/each}
											</ul>
										{/if}
									</li>
								{/each}
							</ul>
						{/if}
					</div>
				{/each}
			{/if}
		</section>
		{/if}
		<!-- /stories tab -->
	{/if}
</main>

{#if consoleStep || consoleLoading}
	<!-- Backdrop + drawer : cliquer en dehors ferme la console. On
		 garde consoleStep en state tant qu'il est rempli ; loading
		 affiche juste un spinner vide pour que la drawer ne clignote pas. -->
	<div
		class="console-backdrop"
		role="presentation"
		onclick={closeConsole}
		onkeydown={(e) => e.key === 'Escape' && closeConsole()}
	></div>
	<aside class="console-drawer" aria-label="Console Claude">
		<header class="console-head">
			<div class="console-title">
				<span class="console-icon">▸</span>
				<code>{consoleStep?.command ?? '…'}</code>
				{#if consoleStep}
					<span class="console-phase">{consoleStep.phase}</span>
					<span class="console-status {consoleStep.status}">{consoleStep.status}</span>
				{/if}
			</div>
			<button type="button" class="console-close" onclick={closeConsole} aria-label="Fermer">×</button>
		</header>
		{#if consoleLoading}
			<div class="console-body loading"><span class="spinner"></span> Chargement…</div>
		{:else if consoleStep}
			<div class="console-meta">
				<span>Démarré {fmtRelative(consoleStep.started_at)}</span>
				{#if consoleStep.finished_at}
					<span>Fini {fmtRelative(consoleStep.finished_at)}</span>
				{/if}
				{#if consoleStep.cost_usd > 0}
					<span>${consoleStep.cost_usd.toFixed(4)}</span>
				{/if}
				{#if consoleStep.input_tokens > 0 || consoleStep.output_tokens > 0}
					<span>{consoleStep.input_tokens} in · {consoleStep.output_tokens} out tokens</span>
				{/if}
			</div>
			{#if consoleStep.error}
				<div class="console-error">
					<strong>Erreur :</strong>
					<pre>{consoleStep.error}</pre>
				</div>
			{/if}
			<div class="console-body">
				{#if consoleStep.reply_full}
					<pre class="console-pre">{consoleStep.reply_full}</pre>
				{:else}
					<p class="empty">
						{#if consoleStep.status === 'running'}
							Skill en cours — la console s'affichera quand BMAD aura terminé cette étape.
						{:else}
							Aucune sortie console pour ce step.
						{/if}
					</p>
				{/if}
			</div>
		{/if}
	</aside>
{/if}

<style>
	main {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
		max-width: 1000px;
	}
	.back {
		color: var(--muted);
		text-decoration: none;
		font-size: 0.85rem;
	}
	.back:hover { color: var(--accent); }
	h2 { font-size: 1.05rem; margin: 0 0 0.75rem 0; }
	h3 { font-size: 0.95rem; margin: 0 0 0.5rem 0; }
	.muted { color: var(--muted); }

	/* ===== Tabs nav ===== */
	.tabs-nav {
		display: flex;
		gap: 0.25rem;
		padding: 0.25rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
		margin: 0.5rem 0;
		overflow-x: auto;
		scrollbar-width: none;
	}
	.tabs-nav::-webkit-scrollbar { display: none; }
	.tab {
		display: inline-flex;
		align-items: center;
		gap: 0.45rem;
		padding: 0.55rem 0.95rem;
		background: transparent;
		border: none;
		border-radius: 6px;
		color: var(--text-muted);
		font: inherit;
		font-size: 0.85rem;
		font-weight: 600;
		cursor: pointer;
		white-space: nowrap;
		transition: background 0.1s, color 0.1s;
	}
	.tab:hover {
		background: var(--bg-hover);
		color: var(--text);
	}
	.tab.active {
		background: color-mix(in srgb, var(--accent) 14%, transparent);
		color: var(--accent);
	}
	.tab-icon {
		font-size: 0.95rem;
		opacity: 0.85;
	}
	.tab-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 20px;
		height: 20px;
		padding: 0 0.5rem;
		background: var(--bg-hover);
		color: var(--text-muted);
		border-radius: 999px;
		font-size: 0.7rem;
		font-weight: 700;
		margin-left: 0.15rem;
	}
	.tab.active .tab-badge {
		background: color-mix(in srgb, var(--accent) 22%, transparent);
		color: var(--accent);
	}
	@media (max-width: 767px) {
		.tab { padding: 0.5rem 0.7rem; font-size: 0.8rem; }
		.tab-icon { display: none; }
	}

	/* ===== Hero header ===== */
	.hero {
		background:
			radial-gradient(ellipse at top right, color-mix(in srgb, var(--accent) 10%, transparent), transparent 60%),
			var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 1.5rem 1.75rem;
	}
	.hero-top {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		flex-wrap: wrap;
		margin-bottom: 0.9rem;
	}
	.hero-top .status {
		font-size: 0.72rem;
		padding: 0.2rem 0.7rem;
		border-radius: 999px;
		font-weight: 700;
		text-transform: lowercase;
		letter-spacing: 0.04em;
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		padding: 0.2rem 0.7rem;
		background: var(--bg-hover);
		border: 1px solid var(--border);
		border-radius: 999px;
		font-size: 0.72rem;
		color: var(--text);
		font-weight: 500;
	}
	.chip.brownfield {
		background: color-mix(in srgb, var(--accent) 12%, transparent);
		border-color: color-mix(in srgb, var(--accent) 40%, transparent);
		color: var(--accent);
	}
	.chip.cost {
		background: color-mix(in srgb, var(--warn) 14%, transparent);
		border-color: color-mix(in srgb, var(--warn) 40%, transparent);
		color: var(--warn);
		font-family: ui-monospace, monospace;
		font-weight: 600;
	}
	.hero-time {
		margin-left: auto;
		font-size: 0.75rem;
		color: var(--text-muted);
	}
	.hero-name {
		font-size: 1.8rem;
		line-height: 1.15;
		margin: 0 0 0.5rem;
		font-weight: 700;
		letter-spacing: -0.02em;
	}
	.hero-idea {
		margin: 0 0 1.1rem;
		font-size: 0.95rem;
		color: var(--text-muted);
		line-height: 1.55;
		max-width: 720px;
	}
	.hero-actions {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
	}
	.btn {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.5rem 0.9rem;
		border-radius: 6px;
		font-size: 0.82rem;
		font-weight: 600;
		cursor: pointer;
		text-decoration: none;
		border: 1px solid var(--border);
		background: var(--bg-panel);
		color: var(--text);
		transition: border-color 0.1s, background 0.1s, color 0.1s;
	}
	.btn:hover { border-color: var(--accent); color: var(--accent); }
	.btn.accent {
		background: color-mix(in srgb, var(--accent) 14%, transparent);
		border-color: color-mix(in srgb, var(--accent) 40%, transparent);
		color: var(--accent);
	}
	.btn.accent:hover {
		background: color-mix(in srgb, var(--accent) 22%, transparent);
	}
	.btn:disabled { opacity: 0.5; cursor: not-allowed; }
	.btn-icon { font-size: 0.9rem; line-height: 1; }

	.hero-details {
		margin-top: 1rem;
		padding-top: 0.9rem;
		border-top: 1px dashed var(--border);
	}
	.hero-details > summary {
		cursor: pointer;
		font-size: 0.78rem;
		color: var(--text-muted);
		list-style: none;
		user-select: none;
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
	}
	.hero-details > summary::-webkit-details-marker { display: none; }
	.hero-details > summary::before {
		content: '▸';
		font-size: 0.7rem;
		transition: transform 0.15s;
	}
	.hero-details[open] > summary::before { transform: rotate(90deg); display: inline-block; }
	.hero-details[open] > summary { margin-bottom: 0.6rem; }
	.hero-details:hover > summary { color: var(--text); }
	.refs {
		display: grid;
		grid-template-columns: max-content 1fr;
		column-gap: 0.75rem;
		row-gap: 0.15rem;
		margin: 0.75rem 0 0;
		font-size: 0.8rem;
	}
	.refs dt { color: var(--muted); }
	.refs dd {
		margin: 0;
		font-family: ui-monospace, monospace;
		overflow-wrap: anywhere;
		word-break: break-all;
	}
	@media (max-width: 767px) {
		.refs {
			grid-template-columns: 1fr;
			row-gap: 0.35rem;
		}
		.refs dt {
			font-size: 0.7rem;
			text-transform: uppercase;
			letter-spacing: 0.05em;
		}
		.refs dd { margin-bottom: 0.5rem; }
	}
	.badge {
		display: inline-block;
		padding: 0.125rem 0.5rem;
		border-radius: 4px;
		color: white;
		font-size: 0.7rem;
		font-weight: 500;
	}
	.badge.brownfield {
		background: color-mix(in srgb, var(--accent) 22%, var(--bg-alt));
		color: var(--accent);
		border: 1px solid var(--accent);
	}
	.repo-link {
		display: inline-flex;
		align-items: center;
		gap: 0.15rem;
		padding: 0.1rem 0.45rem;
		border: 1px solid var(--border);
		border-radius: 4px;
		color: var(--muted);
		text-decoration: none;
		font-size: 0.72rem;
	}
	.repo-link:hover { color: var(--accent); border-color: var(--accent); }
	.brownfield-note {
		padding: 0.6rem 0.85rem;
		background: color-mix(in srgb, var(--accent) 10%, var(--bg-alt));
		border-left: 3px solid var(--accent);
		border-radius: 0 4px 4px 0;
		font-size: 0.85rem;
		line-height: 1.5;
	}
	.brownfield-note code {
		font-family: ui-monospace, monospace;
		font-size: 0.78rem;
		background: var(--bg);
		padding: 0.05rem 0.35rem;
		border-radius: 3px;
	}
	.cost-pill {
		display: inline-block;
		padding: 0.1rem 0.5rem;
		background: color-mix(in srgb, var(--warn) 18%, var(--bg-alt));
		border: 1px solid color-mix(in srgb, var(--warn) 45%, var(--border));
		border-radius: 999px;
		color: var(--warn);
		font-size: 0.72rem;
		font-weight: 600;
	}
	.fail-banner {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
		padding: 0.9rem 1rem;
		background: color-mix(in srgb, var(--err) 12%, var(--bg-alt));
		border-left: 3px solid var(--err);
		border-radius: 0 6px 6px 0;
		flex-wrap: wrap;
	}
	.fail-banner > div { flex: 1 1 200px; min-width: 0; }
	.fail-banner code {
		font-family: ui-monospace, monospace;
		font-size: 0.8rem;
		background: var(--bg);
		padding: 0.1rem 0.45rem;
		border-radius: 3px;
	}
	.fail-error {
		margin: 0.5rem 0 0;
		padding: 0.5rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		font-size: 0.75rem;
		white-space: pre-wrap;
		max-height: 160px;
		overflow-y: auto;
	}
	.retry-btn {
		padding: 0.45rem 0.85rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-weight: 600;
		white-space: nowrap;
	}
	.retry-btn:disabled { opacity: 0.5; cursor: not-allowed; }
	.header-actions {
		margin-top: 0.75rem;
		display: flex;
		gap: 0.5rem;
	}
	.export-btn {
		padding: 0.4rem 0.8rem;
		border: 1px solid var(--border);
		border-radius: 4px;
		color: var(--text-muted);
		text-decoration: none;
		font-size: 0.78rem;
		transition: border-color 0.1s, color 0.1s;
	}
	.export-btn:hover {
		border-color: var(--accent);
		color: var(--accent);
	}

	/* ===== Phases panel ===== */
	.phases {
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 1.25rem 1.4rem;
	}
	.phases-head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.75rem;
		margin-bottom: 1rem;
	}
	.phases-title {
		display: flex;
		align-items: center;
		gap: 0.6rem;
	}
	.phases-title h2 {
		margin: 0;
		font-size: 0.9rem;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--text);
		font-weight: 700;
	}
	.phases-icon {
		width: 28px;
		height: 28px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
		display: inline-flex;
		align-items: center;
		justify-content: center;
		font-size: 0.9rem;
		flex-shrink: 0;
	}
	.running-pill {
		display: inline-flex;
		align-items: center;
		gap: 0.45rem;
		padding: 0.25rem 0.75rem 0.25rem 0.65rem;
		background: color-mix(in srgb, var(--warn) 14%, transparent);
		border: 1px solid color-mix(in srgb, var(--warn) 40%, transparent);
		border-radius: 999px;
		font-size: 0.7rem;
		color: var(--warn);
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	/* Dot pulsant : utilisé dans running-pill + phase-status.running.
	   Ring extérieur qui grandit et fade pour donner l'effet "live". */
	.live-dot {
		display: inline-block;
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: currentColor;
		position: relative;
		flex-shrink: 0;
	}
	.live-dot.big { width: 10px; height: 10px; }
	.live-dot::after {
		content: '';
		position: absolute;
		inset: -4px;
		border-radius: 50%;
		border: 2px solid currentColor;
		opacity: 0.6;
		animation: live-ring 1.6s ease-out infinite;
	}
	@keyframes live-ring {
		0% { transform: scale(0.6); opacity: 0.7; }
		100% { transform: scale(1.6); opacity: 0; }
	}
	.btn.danger {
		border-color: color-mix(in srgb, var(--err) 40%, var(--border));
		color: var(--err);
	}
	.btn.danger:hover {
		background: color-mix(in srgb, var(--err) 15%, transparent);
		border-color: var(--err);
		color: var(--err);
	}
	.btn.sm {
		padding: 0.4rem 0.75rem;
		font-size: 0.75rem;
	}

	/* Summary cards */
	.phases-summary {
		display: flex;
		gap: 0.75rem;
		flex-wrap: wrap;
		margin-bottom: 0.85rem;
	}
	.sum-item {
		padding: 0.6rem 0.9rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 8px;
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
		min-width: 110px;
	}
	.sum-item.flex-fill { flex: 1 1 200px; min-width: 0; }
	.sum-num {
		font-size: 1.15rem;
		font-weight: 700;
		line-height: 1;
		font-variant-numeric: tabular-nums;
	}
	.sum-num.mono {
		font-family: ui-monospace, monospace;
		font-size: 1rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.sum-num.mono.small {
		font-size: 0.85rem;
		font-weight: 600;
	}
	.sum-total {
		font-size: 0.85rem;
		color: var(--text-muted);
		font-weight: 500;
	}
	.sum-label {
		font-size: 0.7rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	/* Progress bar */
	.progress-bar {
		height: 6px;
		background: var(--bg-hover);
		border-radius: 999px;
		overflow: hidden;
		margin-bottom: 1rem;
	}
	.progress-fill {
		height: 100%;
		background: linear-gradient(90deg, var(--accent), color-mix(in srgb, var(--accent) 70%, var(--ok)));
		border-radius: 999px;
		transition: width 0.4s ease;
	}

	/* Phase list — vertical stepped timeline */
	.phase-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		max-height: 340px;
		overflow-y: auto;
	}
	.phase-item {
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 6px;
		font-size: 0.82rem;
		transition: border-color 0.1s;
		overflow: hidden;
	}
	.phase-row {
		display: grid;
		grid-template-columns: 24px 1fr auto 16px;
		align-items: center;
		gap: 0.6rem;
		padding: 0.55rem 0.75rem;
		width: 100%;
		background: transparent;
		border: none;
		text-align: left;
		cursor: pointer;
		color: inherit;
		font-family: inherit;
		font-size: inherit;
	}
	.phase-row:hover {
		background: color-mix(in srgb, var(--accent) 6%, transparent);
	}
	.phase-chev {
		color: var(--text-muted);
		font-size: 1rem;
	}
	.phase-item.running {
		background: color-mix(in srgb, var(--accent) 8%, var(--bg));
		border-color: color-mix(in srgb, var(--accent) 30%, var(--border));
	}
	.phase-item.failed {
		background: color-mix(in srgb, var(--err) 10%, var(--bg));
		border-color: color-mix(in srgb, var(--err) 40%, var(--border));
	}
	.phase-status {
		width: 24px;
		height: 24px;
		border-radius: 50%;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		font-size: 0.75rem;
		font-weight: 700;
		flex-shrink: 0;
	}
	.phase-item.done .phase-status {
		background: color-mix(in srgb, var(--ok) 20%, transparent);
		color: var(--ok);
	}
	.phase-item.failed .phase-status {
		background: color-mix(in srgb, var(--err) 20%, transparent);
		color: var(--err);
	}
	.phase-item.running .phase-status {
		background: color-mix(in srgb, var(--accent) 20%, transparent);
		color: var(--accent);
	}
	.phase-cmd {
		font-family: ui-monospace, monospace;
		font-size: 0.82rem;
		font-weight: 500;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		grid-column: 2;
		background: transparent;
		padding: 0;
		color: var(--text);
	}
	.phase-phase {
		grid-column: 2;
		grid-row: 2;
		font-size: 0.68rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.phase-meta {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 0.15rem;
		font-size: 0.72rem;
		color: var(--text-muted);
		font-variant-numeric: tabular-nums;
		grid-column: 3;
		grid-row: 1 / span 2;
	}
	.phase-meta .tokens { opacity: 0.7; }
	.phase-time { opacity: 0.6; }

	.small { font-size: 0.72rem; }
	@keyframes pulse-dot { 0%,100% { opacity: 1; } 50% { opacity: 0.3; } }
	.empty {
		color: var(--muted);
		font-style: italic;
	}
	.planning {
		display: flex;
		gap: 0.5rem;
		align-items: center;
		padding: 0.75rem 1rem;
		background: color-mix(in srgb, var(--accent) 12%, var(--bg-alt));
		border-left: 3px solid var(--accent);
		border-radius: 4px;
		color: var(--text);
		font-size: 0.9rem;
	}
	.spinner {
		display: inline-block;
		width: 14px;
		height: 14px;
		border: 2px solid var(--accent);
		border-top-color: transparent;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin { to { transform: rotate(360deg); } }
	.bar {
		position: relative;
		height: 18px;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 9px;
		overflow: hidden;
		margin-bottom: 0.75rem;
	}
	.bar-fill {
		height: 100%;
		background: linear-gradient(90deg, var(--accent), color-mix(in srgb, var(--accent) 60%, var(--ok)));
		transition: width 400ms ease-out;
	}
	.bar-fill.shipped {
		background: var(--ok);
	}
	.bar-label {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 0.7rem;
		font-weight: 600;
		color: var(--text);
		mix-blend-mode: difference;
		filter: invert(1);
	}
	.metrics {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
		gap: 1rem;
	}
	.metrics div {
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.metrics strong {
		display: block;
		font-size: 1.5rem;
		margin-bottom: 0.2rem;
	}
	.metrics span {
		font-size: 0.8rem;
		color: var(--muted);
	}
	.panel {
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.prd {
		white-space: pre-wrap;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		padding: 0.75rem;
		font-size: 0.85rem;
		overflow-x: auto;
		max-height: 600px;
		overflow-y: auto;
		margin: 0;
	}
	.prd-head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.5rem;
		gap: 0.5rem;
		flex-wrap: wrap;
	}
	.prd-head h3 {
		margin: 0;
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 1rem;
	}
	.prd-meta {
		font-size: 0.7rem;
		color: var(--text-muted);
		font-weight: 500;
		font-variant-numeric: tabular-nums;
		text-transform: none;
		letter-spacing: 0;
	}

	/* Preview collapsed : snippet + fade + bouton */
	.prd-preview {
		position: relative;
		cursor: pointer;
		border-radius: 4px;
	}
	.prd-snippet {
		max-height: 140px;
		overflow: hidden;
		mask-image: linear-gradient(to bottom, black 40%, transparent 100%);
		-webkit-mask-image: linear-gradient(to bottom, black 40%, transparent 100%);
	}
	.prd-fade {
		display: flex;
		justify-content: center;
		margin-top: -1.5rem;
		position: relative;
		z-index: 1;
	}
	.prd-expand-btn {
		padding: 0.4rem 0.9rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		color: var(--text);
		font: inherit;
		font-size: 0.78rem;
		font-weight: 600;
		cursor: pointer;
		transition: border-color 0.1s, color 0.1s;
		box-shadow: 0 2px 8px color-mix(in srgb, var(--text) 5%, transparent);
	}
	.prd-expand-btn:hover {
		border-color: var(--accent);
		color: var(--accent);
	}
	.prd-actions {
		display: flex;
		gap: 0.4rem;
	}
	.prd-actions button {
		padding: 0.3rem 0.7rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font-size: 0.8rem;
		cursor: pointer;
	}
	.prd-actions button.warn {
		border-color: var(--warn);
		color: var(--warn);
	}
	.prd-actions button.warn:hover { background: color-mix(in srgb, var(--warn) 15%, var(--bg)); }
	.prd-actions button:disabled { opacity: 0.5; cursor: not-allowed; }
	.prd-editor {
		width: 100%;
		padding: 0.75rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font: inherit;
		font-family: ui-monospace, monospace;
		font-size: 0.85rem;
		resize: vertical;
	}
	.intake {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}
	.chat {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		max-height: 520px;
		overflow-y: auto;
		padding: 0.5rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.bubble {
		padding: 0.55rem 0.75rem;
		border-radius: 8px;
		max-width: 75%;
	}
	.bubble.user {
		align-self: flex-end;
		background: color-mix(in srgb, var(--accent) 18%, var(--bg));
		border: 1px solid color-mix(in srgb, var(--accent) 40%, var(--border));
	}
	.bubble.agent {
		align-self: flex-start;
		background: var(--bg);
		border: 1px solid var(--border);
	}
	.bubble-head {
		display: flex;
		gap: 0.5rem;
		font-size: 0.7rem;
		color: var(--muted);
		margin-bottom: 0.25rem;
	}
	.edit-msg {
		margin-left: auto;
		background: none;
		border: none;
		color: var(--muted);
		cursor: pointer;
		font-size: 0.75rem;
		padding: 0;
	}
	.edit-msg:hover { color: var(--accent); }
	.edit-area {
		width: 100%;
		padding: 0.4rem 0.6rem;
		background: var(--bg);
		border: 1px solid var(--accent);
		border-radius: 4px;
		font: inherit;
		color: inherit;
		resize: vertical;
	}
	.edit-actions {
		display: flex;
		gap: 0.35rem;
		margin-top: 0.4rem;
	}
	.edit-actions button {
		padding: 0.25rem 0.7rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 4px;
		cursor: pointer;
		font: inherit;
		font-size: 0.78rem;
		color: inherit;
	}
	.edit-actions button:first-child {
		background: var(--accent);
		color: white;
		border: none;
	}
	.edit-actions button:disabled { opacity: 0.5; cursor: not-allowed; }
	.bubble-content {
		white-space: pre-wrap;
		font-size: 0.9rem;
		line-height: 1.4;
	}
	.reply-form {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.reply-form textarea {
		padding: 0.55rem 0.75rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		color: inherit;
		font: inherit;
		font-family: inherit;
		resize: vertical;
	}
	.reply-actions {
		display: flex;
		gap: 0.5rem;
		align-items: center;
		flex-wrap: wrap;
	}
	.reply-actions button {
		padding: 0.45rem 0.85rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		cursor: pointer;
		color: inherit;
		font-weight: 500;
	}
	.reply-actions button.primary {
		background: var(--accent);
		color: white;
		border: none;
	}
	.reply-actions button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.done-note {
		padding: 0.6rem 0.85rem;
		background: color-mix(in srgb, var(--ok) 12%, var(--bg-alt));
		border-left: 3px solid var(--ok);
		border-radius: 4px;
		font-size: 0.85rem;
	}
	.err {
		padding: 0.5rem 0.75rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
		font-size: 0.85rem;
	}
	.epic {
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		margin-bottom: 0.75rem;
	}
	.epic header {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}
	.desc {
		color: var(--muted);
		font-size: 0.9rem;
		margin: 0.4rem 0 0.75rem 0;
	}
	.stories {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.activity .count {
		font-size: 0.75rem;
		color: var(--muted);
		font-weight: 400;
		margin-left: 0.25rem;
	}
	.feed {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		max-height: 280px;
		overflow-y: auto;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 0.5rem 0.75rem;
	}
	.feed li {
		display: flex;
		gap: 0.6rem;
		align-items: baseline;
		font-size: 0.82rem;
		line-height: 1.4;
	}
	.feed .t { font-weight: 600; min-width: 11rem; }
	.feed .story-ref {
		font-family: ui-monospace, monospace;
		font-size: 0.78rem;
		color: var(--text);
	}
	.feed .feedback {
		color: var(--muted);
		font-style: italic;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 38rem;
	}
	.stories li {
		padding: 0.6rem 0.75rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		transition: border-color 200ms ease, box-shadow 200ms ease;
	}
	.stories li.active {
		border-color: var(--warn);
		box-shadow: 0 0 0 2px color-mix(in srgb, var(--warn) 35%, transparent);
		animation: pulse 1.8s ease-in-out infinite;
	}
	@keyframes pulse {
		0%, 100% { box-shadow: 0 0 0 2px color-mix(in srgb, var(--warn) 35%, transparent); }
		50%      { box-shadow: 0 0 0 4px color-mix(in srgb, var(--warn) 18%, transparent); }
	}
	.story-head {
		display: flex;
		gap: 0.5rem;
		align-items: center;
		font-size: 0.9rem;
	}
	.review-feedback {
		margin: 0.5rem 0 0;
		padding: 0.45rem 0.65rem;
		background: color-mix(in srgb, var(--warn) 10%, var(--bg));
		border-left: 3px solid var(--warn);
		border-radius: 0 4px 4px 0;
		color: var(--text);
		font-size: 0.8rem;
		line-height: 1.4;
	}
	.review-feedback.blocked {
		background: color-mix(in srgb, var(--err) 12%, var(--bg));
		border-left-color: var(--err);
	}
	.review-label {
		color: var(--muted);
		margin-right: 0.3rem;
		font-weight: 600;
	}
	.pr-link {
		display: inline-flex;
		align-items: center;
		gap: 0.2rem;
		padding: 0.15rem 0.5rem;
		background: color-mix(in srgb, var(--accent) 16%, var(--bg-alt));
		border: 1px solid var(--accent);
		border-radius: 999px;
		color: var(--accent);
		text-decoration: none;
		font-size: 0.72rem;
		font-weight: 600;
	}
	.pr-link:hover { background: color-mix(in srgb, var(--accent) 30%, var(--bg-alt)); }
	.branch-tag {
		display: inline-block;
		padding: 0.1rem 0.45rem;
		font-family: ui-monospace, monospace;
		font-size: 0.68rem;
		color: var(--muted);
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 999px;
	}
	.retry {
		margin-left: auto;
		padding: 0.2rem 0.55rem;
		background: var(--bg-alt);
		border: 1px solid var(--warn);
		color: var(--warn);
		border-radius: 4px;
		font-size: 0.75rem;
		cursor: pointer;
	}
	.retry:hover { background: color-mix(in srgb, var(--warn) 18%, var(--bg-alt)); }
	.retry:disabled { opacity: 0.5; cursor: not-allowed; }
	.acs {
		list-style: none;
		padding: 0;
		margin: 0.5rem 0 0 0;
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		font-size: 0.85rem;
	}
	.acs li {
		padding: 0.2rem 0;
		background: transparent;
		border: none;
		border-radius: 0;
		color: var(--muted);
	}
	.acs li.passed {
		color: var(--ok);
	}
	.acs .check {
		display: inline-block;
		width: 1rem;
		margin-right: 0.4rem;
	}

	/* Console drawer — panneau latéral qui affiche la sortie complète
	   du skill Claude (reply_full). Click hors du drawer ou bouton X
	   ferme. Backdrop translucide pour désactiver les clics sur le
	   contenu en arrière-plan. */
	.console-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.35);
		z-index: 200;
	}
	.console-drawer {
		position: fixed;
		top: 0;
		right: 0;
		bottom: 0;
		width: min(720px, 95vw);
		background: var(--bg, #fff);
		border-left: 1px solid var(--border);
		box-shadow: -8px 0 24px rgba(0, 0, 0, 0.12);
		z-index: 201;
		display: flex;
		flex-direction: column;
		animation: slide-in 0.18s ease-out;
	}
	@keyframes slide-in {
		from { transform: translateX(100%); }
		to { transform: translateX(0); }
	}
	.console-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 1rem 1.25rem;
		border-bottom: 1px solid var(--border);
		gap: 0.75rem;
		flex-shrink: 0;
	}
	.console-title {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		min-width: 0;
		flex: 1;
	}
	.console-icon {
		font-size: 0.8rem;
		color: var(--text-muted);
	}
	.console-title code {
		font-family: ui-monospace, monospace;
		font-size: 0.9rem;
		font-weight: 600;
		color: var(--text);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}
	.console-phase {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
		background: color-mix(in srgb, var(--text-muted) 15%, transparent);
		padding: 2px 6px;
		border-radius: 4px;
	}
	.console-status {
		font-size: 0.72rem;
		padding: 2px 8px;
		border-radius: 10px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		font-weight: 600;
	}
	.console-status.done {
		background: color-mix(in srgb, var(--ok) 18%, transparent);
		color: var(--ok);
	}
	.console-status.failed {
		background: color-mix(in srgb, var(--err) 18%, transparent);
		color: var(--err);
	}
	.console-status.running {
		background: color-mix(in srgb, var(--accent) 18%, transparent);
		color: var(--accent);
	}
	.console-close {
		background: transparent;
		border: none;
		font-size: 1.6rem;
		cursor: pointer;
		color: var(--text-muted);
		line-height: 1;
		padding: 0 0.4rem;
	}
	.console-close:hover {
		color: var(--text);
	}
	.console-meta {
		display: flex;
		flex-wrap: wrap;
		gap: 1rem;
		padding: 0.6rem 1.25rem;
		background: var(--bg-soft, rgba(0,0,0,0.02));
		font-size: 0.78rem;
		color: var(--text-muted);
		font-variant-numeric: tabular-nums;
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
	}
	.console-error {
		padding: 0.75rem 1.25rem;
		background: color-mix(in srgb, var(--err) 10%, transparent);
		border-bottom: 1px solid color-mix(in srgb, var(--err) 30%, var(--border));
	}
	.console-error pre {
		margin: 0.4rem 0 0;
		font-size: 0.82rem;
		color: var(--err);
		white-space: pre-wrap;
		word-break: break-word;
	}
	.console-body {
		flex: 1;
		overflow-y: auto;
		padding: 1rem 1.25rem;
		min-height: 0;
	}
	.console-body.loading {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		color: var(--text-muted);
	}
	.console-pre {
		font-family: ui-monospace, 'SF Mono', Menlo, monospace;
		font-size: 0.82rem;
		line-height: 1.55;
		white-space: pre-wrap;
		word-break: break-word;
		color: var(--text);
		margin: 0;
	}
</style>
