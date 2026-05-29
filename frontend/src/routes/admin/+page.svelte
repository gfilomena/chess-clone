<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { user } from '$lib/stores/auth';
	import { API_URL as API } from '$lib/config';

	// ── Guard ──────────────────────────────────────────────────────────────────
	// Aspetta che lo store si risolva (null = non loggato, undefined = loading)
	$effect(() => {
		if ($user !== undefined && !$user?.is_admin) goto('/');
	});

	// ── Tabs ───────────────────────────────────────────────────────────────────
	type Tab = 'overview' | 'users' | 'games' | 'live';
	let activeTab = $state<Tab>('overview');

	// ── Data stores ────────────────────────────────────────────────────────────
	let stats        = $state<any>(null);
	let users        = $state<any[]>([]);
	let games        = $state<any[]>([]);
	let hub          = $state<any>(null);
	let queue        = $state<any[]>([]);

	let statsLoading = $state(false);
	let usersLoading = $state(false);
	let gamesLoading = $state(false);
	let liveLoading  = $state(false);

	let userSearch   = $state('');
	let searchTimer  = $state<ReturnType<typeof setTimeout> | null>(null);

	let liveInterval: ReturnType<typeof setInterval> | null = null;
	let actionMsg    = $state('');

	// ── Fetch helpers ──────────────────────────────────────────────────────────
	async function apiFetch(path: string, opts: RequestInit = {}) {
		const r = await fetch(`${API}${path}`, { credentials: 'include', ...opts });
		const j = await r.json();
		return j.success ? j.data : null;
	}

	async function fetchStats() {
		statsLoading = true;
		stats = await apiFetch('/api/admin/stats');
		statsLoading = false;
	}

	async function fetchUsers(q = '') {
		usersLoading = true;
		const qs = q ? `?q=${encodeURIComponent(q)}` : '';
		users = (await apiFetch(`/api/admin/users${qs}`)) ?? [];
		usersLoading = false;
	}

	async function fetchGames() {
		gamesLoading = true;
		games = (await apiFetch('/api/admin/games')) ?? [];
		gamesLoading = false;
	}

	async function fetchLive() {
		liveLoading = true;
		[hub, queue] = await Promise.all([
			apiFetch('/api/admin/hub'),
			apiFetch('/api/admin/queue'),
		]);
		queue ??= [];
		liveLoading = false;
	}

	// ── Tab switching ──────────────────────────────────────────────────────────
	function switchTab(t: Tab) {
		activeTab = t;
		stopLive();
		if (t === 'overview') fetchStats();
		if (t === 'users')    fetchUsers(userSearch);
		if (t === 'games')    fetchGames();
		if (t === 'live')     startLive();
	}

	function startLive() {
		fetchLive();
		liveInterval = setInterval(fetchLive, 5000);
	}
	function stopLive() {
		if (liveInterval) { clearInterval(liveInterval); liveInterval = null; }
	}

	// Debounce ricerca utenti
	function onSearchInput() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => fetchUsers(userSearch), 350);
	}

	// ── Azioni utente ──────────────────────────────────────────────────────────
	async function patchUser(id: string, action: string) {
		const r = await fetch(`${API}/api/admin/users/${id}`, {
			method: 'PATCH',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ action }),
		});
		const j = await r.json();
		if (j.success) {
			actionMsg = `✅ ${action} applicato`;
			setTimeout(() => actionMsg = '', 2500);
			fetchUsers(userSearch);
		}
	}

	async function clearQueue() {
		if (!confirm('Svuotare la coda di matchmaking?')) return;
		const data = await apiFetch('/api/admin/queue', { method: 'DELETE' });
		if (data !== null) {
			actionMsg = `✅ Coda svuotata (${data.cleared} giocatori rimossi)`;
			setTimeout(() => actionMsg = '', 3000);
			fetchLive();
		}
	}

	// ── Lifecycle ──────────────────────────────────────────────────────────────
	onMount(() => fetchStats());
	onDestroy(() => stopLive());

	// ── Formatters ─────────────────────────────────────────────────────────────
	function fmtTC(secs: number) {
		const m = Math.floor(secs / 60);
		return m >= 60 ? `${Math.floor(m/60)}h` : `${m}min`;
	}
	function fmtResult(r: string) {
		return r === 'white' ? '⬜ Bianco' : r === 'black' ? '⬛ Nero' : '½ Patta';
	}
	function fmtReason(r: string) {
		const map: Record<string, string> = {
			checkmate: 'Scaccomatto', resign: 'Abbandono',
			timeout: 'Timeout', draw: 'Patta', stalemate: 'Stallo',
			abandoned: 'Abbandonato', agreement: 'Accordo',
			timeout_vs_insufficient_material: 'Timeout / Mat. insuff.',
			unknown: '—',
		};
		return map[r] ?? r;
	}
	function fmtWait(s: number) {
		return s < 60 ? `${s}s` : `${Math.floor(s/60)}m ${s%60}s`;
	}
	function shortID(id: string) { return id.slice(0, 8); }

	// Sparkline barre di testo
	const BAR_CHARS = ['▁','▂','▃','▄','▅','▆','▇','█'];
	function sparkline(data: {day:string;count:number}[]) {
		if (!data?.length) return '—';
		const max = Math.max(...data.map(d => d.count), 1);
		return data.map(d => {
			const idx = Math.round((d.count / max) * (BAR_CHARS.length - 1));
			return BAR_CHARS[idx];
		}).join('');
	}

	// Totale partite per calcolo percentuali
	const totalFinished = $derived(
		Object.values(stats?.finish_reasons ?? {}).reduce((a: number, b: any) => a + b, 0) as number
	);
	function pct(n: number) {
		if (!totalFinished) return '0%';
		return `${Math.round((n / totalFinished) * 100)}%`;
	}
</script>

<svelte:head><title>Admin — Chess</title></svelte:head>

<div class="admin-wrap">

	<!-- Header -->
	<div class="admin-header">
		<div class="admin-title">
			<span class="admin-icon">⚙️</span>
			<h1>Admin Panel</h1>
			{#if $user}<span class="admin-badge">{$user.username}</span>{/if}
		</div>
		<a href="/" class="back-link">← Torna alla home</a>
	</div>

	<!-- Toast azione -->
	{#if actionMsg}
		<div class="action-toast">{actionMsg}</div>
	{/if}

	<!-- Tabs -->
	<div class="tabs">
		{#each ([['overview','📊 Panoramica'],['users','👥 Utenti'],['games','♟ Partite'],['live','🔴 Live']] as const) as [t, label]}
			<button
				class="tab-btn"
				class:active={activeTab === t}
				onclick={() => switchTab(t)}
			>{label}</button>
		{/each}
	</div>

	<!-- ══════════════════════════════════════════════════════ OVERVIEW ══ -->
	{#if activeTab === 'overview'}
		{#if statsLoading && !stats}
			<p class="loading">Caricamento…</p>
		{:else if stats}
			<!-- Metric cards -->
			<div class="cards">
				<div class="card">
					<span class="card-val">{stats.total_users}</span>
					<span class="card-lbl">Utenti totali</span>
				</div>
				<div class="card">
					<span class="card-val">{stats.total_games}</span>
					<span class="card-lbl">Partite totali</span>
				</div>
				<div class="card accent">
					<span class="card-val">{stats.online_users}</span>
					<span class="card-lbl">Online ora</span>
				</div>
				<div class="card">
					<span class="card-val">{stats.active_rooms}</span>
					<span class="card-lbl">Room WS attive</span>
				</div>
				<div class="card">
					<span class="card-val">{stats.queue_size}</span>
					<span class="card-lbl">In coda</span>
				</div>
			</div>

			<!-- Sparkline -->
			<div class="section">
				<h2>Partite — ultimi 7 giorni</h2>
				<div class="sparkline-row">
					<span class="sparkline">{sparkline(stats.daily_games)}</span>
					<div class="sparkline-labels">
						{#each stats.daily_games as d}
							<span class="spark-day">{d.day.slice(5)} <b>{d.count}</b></span>
						{/each}
					</div>
				</div>
			</div>

			<!-- Finish reasons -->
			<div class="section">
				<h2>Esiti partite</h2>
				<div class="reasons-grid">
					{#each Object.entries(stats.finish_reasons) as [reason, count]}
						<div class="reason-row">
							<span class="reason-name">{fmtReason(reason)}</span>
							<div class="reason-bar-wrap">
								<div class="reason-bar" style="width:{pct(count as number)}"></div>
							</div>
							<span class="reason-count">{count} ({pct(count as number)})</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}

	<!-- ═════════════════════════════════════════════════════════ USERS ══ -->
	{:else if activeTab === 'users'}
		<div class="section-tools">
			<input
				class="search-input"
				type="text"
				placeholder="Cerca username o email…"
				bind:value={userSearch}
				oninput={onSearchInput}
			/>
			{#if usersLoading}<span class="mini-loading">⟳</span>{/if}
		</div>

		<div class="table-wrap">
			<table class="data-table">
				<thead>
					<tr>
						<th>Username</th>
						<th>Email</th>
						<th>ELO</th>
						<th>Partite</th>
						<th>Registrato</th>
						<th>Ultimo accesso</th>
						<th>Azioni</th>
					</tr>
				</thead>
				<tbody>
					{#each users as u}
						<tr class:banned={u.is_banned}>
							<td class="username-cell">
								{u.username}
								{#if u.is_banned}<span class="ban-badge">bannato</span>{/if}
							</td>
							<td class="muted">{u.email}</td>
							<td><b>{u.elo_rapid}</b></td>
							<td>{u.total_games}</td>
							<td class="muted">{u.created_at.slice(0,10)}</td>
							<td class="muted">{u.last_seen ? u.last_seen.slice(0,16).replace('T',' ') : '—'}</td>
							<td class="actions-cell">
								<button class="btn-sm" title="Reset ELO a 800"
									onclick={() => patchUser(u.id, 'reset_elo')}>↺ ELO</button>
								{#if u.is_banned}
									<button class="btn-sm green" title="Sbanna"
										onclick={() => patchUser(u.id, 'unban')}>✅ Sbanna</button>
								{:else}
									<button class="btn-sm red" title="Banna"
										onclick={() => patchUser(u.id, 'ban')}>🚫 Banna</button>
								{/if}
							</td>
						</tr>
					{/each}
					{#if users.length === 0 && !usersLoading}
						<tr><td colspan="7" class="empty">Nessun utente trovato</td></tr>
					{/if}
				</tbody>
			</table>
		</div>

	<!-- ══════════════════════════════════════════════════════ GAMES ═══ -->
	{:else if activeTab === 'games'}
		{#if gamesLoading && games.length === 0}
			<p class="loading">Caricamento…</p>
		{:else}
			<div class="table-wrap">
				<table class="data-table">
					<thead>
						<tr>
							<th>ID</th>
							<th>Bianco</th>
							<th>Nero</th>
							<th>Esito</th>
							<th>Motivo</th>
							<th>TC</th>
							<th>Data</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each games as g}
							<tr>
								<td class="mono muted">{shortID(g.id)}</td>
								<td>{g.white_username}</td>
								<td>{g.black_username}</td>
								<td>{fmtResult(g.result)}</td>
								<td class="muted">{fmtReason(g.finish_reason)}</td>
								<td class="muted">{fmtTC(g.time_control)}</td>
								<td class="muted">{g.created_at.slice(0,16).replace('T',' ')}</td>
								<td>
									<a class="link-btn" href="/analysis/{g.id}" target="_blank">🎬</a>
								</td>
							</tr>
						{/each}
						{#if games.length === 0}
							<tr><td colspan="8" class="empty">Nessuna partita trovata</td></tr>
						{/if}
					</tbody>
				</table>
			</div>
		{/if}

	<!-- ════════════════════════════════════════════════════════ LIVE ═══ -->
	{:else if activeTab === 'live'}
		<div class="live-header">
			<span class="live-dot"></span> Aggiornamento ogni 5s
			{#if liveLoading}<span class="mini-loading">⟳</span>{/if}
		</div>

		<div class="live-grid">
			<!-- Hub rooms -->
			<div class="live-card">
				<h2>Room WebSocket attive</h2>
				{#if hub}
					<p class="live-big">{hub.active_rooms}</p>
					{#if hub.game_ids?.length}
						<ul class="id-list">
							{#each hub.game_ids as id}
								<li>
									<a class="link-btn" href="/game/{id}" target="_blank">
										{shortID(id)}
									</a>
									<a class="link-btn muted" href="/analysis/{id}" target="_blank">🎬</a>
								</li>
							{/each}
						</ul>
					{:else}
						<p class="muted">Nessuna room attiva</p>
					{/if}
				{:else}
					<p class="muted">—</p>
				{/if}
			</div>

			<!-- Queue -->
			<div class="live-card">
				<div class="live-card-header">
					<h2>Coda matchmaking ({queue.length})</h2>
					{#if queue.length > 0}
						<button class="btn-sm red" onclick={clearQueue}>🗑 Svuota</button>
					{/if}
				</div>
				{#if queue.length === 0}
					<p class="muted">Coda vuota</p>
				{:else}
					<table class="data-table small">
						<thead>
							<tr><th>UserID</th><th>ELO</th><th>TC</th><th>Tipo</th><th>Attesa</th></tr>
						</thead>
						<tbody>
							{#each queue as e}
								<tr>
									<td class="mono muted">{shortID(e.user_id)}</td>
									<td><b>{e.elo}</b></td>
									<td>{fmtTC(e.time_control)}</td>
									<td>{e.game_type}</td>
									<td>{fmtWait(e.waiting_secs)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		</div>
	{/if}

</div>

<style>
	.admin-wrap {
		max-width: 1100px;
		margin: 0 auto;
		padding: 2rem 1.5rem 4rem;
	}

	/* ── Header ── */
	.admin-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 1.5rem;
	}
	.admin-title {
		display: flex;
		align-items: center;
		gap: 0.6rem;
	}
	.admin-title h1 {
		font-size: 1.4rem;
		font-weight: 700;
	}
	.admin-icon { font-size: 1.4rem; }
	.admin-badge {
		font-size: 0.72rem;
		background: var(--accent);
		color: #000;
		font-weight: 700;
		padding: 0.15rem 0.5rem;
		border-radius: 20px;
		letter-spacing: 0.02em;
	}
	.back-link {
		font-size: 0.85rem;
		color: var(--text-muted);
	}

	/* ── Action toast ── */
	.action-toast {
		background: rgba(129,182,76,0.15);
		border: 1px solid var(--accent);
		border-radius: 8px;
		padding: 0.5rem 1rem;
		margin-bottom: 1rem;
		font-size: 0.9rem;
		color: var(--accent);
	}

	/* ── Tabs ── */
	.tabs {
		display: flex;
		gap: 0.25rem;
		border-bottom: 2px solid var(--border);
		margin-bottom: 1.5rem;
	}
	.tab-btn {
		background: none;
		border: none;
		color: var(--text-muted);
		font-size: 0.9rem;
		font-weight: 500;
		padding: 0.55rem 1.1rem;
		cursor: pointer;
		border-bottom: 2px solid transparent;
		margin-bottom: -2px;
		transition: color 0.15s, border-color 0.15s;
	}
	.tab-btn:hover { color: var(--text); }
	.tab-btn.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
	}

	/* ── Cards ── */
	.cards {
		display: flex;
		gap: 1rem;
		flex-wrap: wrap;
		margin-bottom: 2rem;
	}
	.card {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 1.2rem 1.6rem;
		min-width: 130px;
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
	}
	.card.accent { border-color: var(--accent); }
	.card-val {
		font-size: 2rem;
		font-weight: 800;
		color: var(--text);
		line-height: 1;
	}
	.card.accent .card-val { color: var(--accent); }
	.card-lbl {
		font-size: 0.75rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	/* ── Section ── */
	.section {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 1.2rem 1.4rem;
		margin-bottom: 1rem;
	}
	.section h2 {
		font-size: 0.85rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: 1rem;
	}

	/* ── Sparkline ── */
	.sparkline-row { display: flex; flex-direction: column; gap: 0.6rem; }
	.sparkline {
		font-size: 1.8rem;
		letter-spacing: 0.05em;
		color: var(--accent);
		font-family: monospace;
	}
	.sparkline-labels {
		display: flex;
		gap: 0.6rem;
		flex-wrap: wrap;
	}
	.spark-day {
		font-size: 0.72rem;
		color: var(--text-muted);
	}
	.spark-day b { color: var(--text); }

	/* ── Finish reasons ── */
	.reasons-grid { display: flex; flex-direction: column; gap: 0.55rem; }
	.reason-row {
		display: grid;
		grid-template-columns: 180px 1fr 110px;
		align-items: center;
		gap: 0.75rem;
	}
	.reason-name { font-size: 0.85rem; }
	.reason-bar-wrap {
		background: var(--border);
		border-radius: 3px;
		height: 6px;
	}
	.reason-bar {
		background: var(--accent);
		height: 100%;
		border-radius: 3px;
		transition: width 0.3s ease;
		min-width: 2px;
	}
	.reason-count { font-size: 0.78rem; color: var(--text-muted); text-align: right; }

	/* ── Tools bar ── */
	.section-tools {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 1rem;
	}
	.search-input {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 6px;
		color: var(--text);
		padding: 0.5rem 0.9rem;
		font-size: 0.9rem;
		outline: none;
		width: 280px;
		transition: border-color 0.2s;
	}
	.search-input:focus { border-color: var(--accent); }

	/* ── Tables ── */
	.table-wrap {
		overflow-x: auto;
		border: 1px solid var(--border);
		border-radius: 10px;
	}
	.data-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.85rem;
	}
	.data-table thead th {
		background: var(--bg-card);
		color: var(--text-muted);
		font-size: 0.72rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		padding: 0.7rem 0.9rem;
		text-align: left;
		border-bottom: 1px solid var(--border);
	}
	.data-table tbody tr {
		border-bottom: 1px solid var(--border);
		transition: background 0.1s;
	}
	.data-table tbody tr:last-child { border-bottom: none; }
	.data-table tbody tr:hover { background: rgba(255,255,255,0.03); }
	.data-table tbody tr.banned { opacity: 0.55; }
	.data-table td {
		padding: 0.6rem 0.9rem;
		vertical-align: middle;
	}
	.data-table.small td, .data-table.small th { padding: 0.45rem 0.7rem; }

	/* ── Table cell helpers ── */
	.muted    { color: var(--text-muted); }
	.mono     { font-family: monospace; font-size: 0.8rem; }
	.username-cell { display: flex; align-items: center; gap: 0.4rem; font-weight: 600; }
	.ban-badge {
		font-size: 0.65rem;
		background: var(--danger);
		color: #fff;
		border-radius: 4px;
		padding: 0.1rem 0.35rem;
		font-weight: 700;
	}
	.actions-cell { display: flex; gap: 0.35rem; }
	.empty { text-align: center; color: var(--text-muted); padding: 2rem !important; }

	/* ── Small buttons ── */
	.btn-sm {
		font-size: 0.72rem;
		font-weight: 600;
		padding: 0.25rem 0.55rem;
		border-radius: 5px;
		border: 1px solid var(--border);
		background: var(--bg-card);
		color: var(--text-muted);
		cursor: pointer;
		transition: border-color 0.15s, color 0.15s;
		white-space: nowrap;
	}
	.btn-sm:hover         { border-color: var(--text-muted); color: var(--text); }
	.btn-sm.red           { border-color: var(--danger); color: var(--danger); }
	.btn-sm.red:hover     { background: rgba(201,95,95,0.1); }
	.btn-sm.green         { border-color: var(--accent); color: var(--accent); }
	.btn-sm.green:hover   { background: rgba(129,182,76,0.1); }
	.link-btn {
		font-size: 0.8rem;
		color: var(--accent);
		text-decoration: none;
		padding: 0.2rem 0.35rem;
		border-radius: 4px;
		transition: background 0.1s;
	}
	.link-btn:hover { background: rgba(129,182,76,0.12); text-decoration: none; }

	/* ── Live tab ── */
	.live-header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.82rem;
		color: var(--text-muted);
		margin-bottom: 1rem;
	}
	.live-dot {
		width: 8px; height: 8px;
		background: #e05050;
		border-radius: 50%;
		animation: livepulse 1.4s ease-in-out infinite;
	}
	@keyframes livepulse {
		0%,100% { opacity: 1; }
		50%      { opacity: 0.3; }
	}
	.live-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
	}
	.live-card {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 1.2rem 1.4rem;
	}
	.live-card h2 {
		font-size: 0.82rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: 0.8rem;
	}
	.live-card-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 0.8rem;
	}
	.live-card-header h2 { margin-bottom: 0; }
	.live-big {
		font-size: 3rem;
		font-weight: 800;
		color: var(--accent);
		line-height: 1;
		margin-bottom: 0.75rem;
	}
	.id-list {
		list-style: none;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		font-size: 0.82rem;
	}
	.id-list li { display: flex; align-items: center; gap: 0.5rem; }

	/* ── Misc ── */
	.loading      { color: var(--text-muted); padding: 2rem; text-align: center; }
	.mini-loading { color: var(--text-muted); font-size: 1.1rem; animation: spin 0.8s linear infinite; }
	@keyframes spin { to { transform: rotate(360deg); } }

	/* ── Mobile ── */
	@media (max-width: 768px) {
		.admin-wrap { padding: 1rem 0.75rem 3rem; }
		.cards { flex-direction: column; }
		.live-grid { grid-template-columns: 1fr; }
		.reason-row { grid-template-columns: 130px 1fr 80px; }
		.tab-btn { font-size: 0.78rem; padding: 0.45rem 0.7rem; }
		.search-input { width: 100%; }
	}
</style>
