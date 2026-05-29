<script lang="ts">
	import { onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { user, authLoading } from '$lib/stores/auth';
	import { onlineUsers, fetchOnlineUsers, sendInvite } from '$lib/stores/invitations';
	import { API_URL as API } from '$lib/config';
	import { t } from '$lib/i18n';
	import { get } from 'svelte/store';

	// ── Auth guard ────────────────────────────────────────────────────────────
	$effect(() => {
		if (!$authLoading && !$user) goto('/login');
	});

	// ── Time controls ─────────────────────────────────────────────────────────
	type TC = { label: string; tc: number; inc: number; type: 'bullet' | 'blitz' | 'rapid' };

	const CATEGORIES: { name: string; icon: string; controls: TC[] }[] = [
		{
			name: 'Bullet', icon: '🚀',
			controls: [
				{ label: '1 min',  tc: 60,  inc: 0, type: 'bullet' },
				{ label: '1 | 1',  tc: 60,  inc: 1, type: 'bullet' },
				{ label: '2 | 1',  tc: 120, inc: 1, type: 'bullet' },
			]
		},
		{
			name: 'Blitz', icon: '⚡',
			controls: [
				{ label: '3 min',  tc: 180, inc: 0, type: 'blitz' },
				{ label: '3 | 2',  tc: 180, inc: 2, type: 'blitz' },
				{ label: '5 min',  tc: 300, inc: 0, type: 'blitz' },
			]
		},
		{
			name: 'Rapid', icon: '🕐',
			controls: [
				{ label: '10 min',  tc: 600,  inc: 0,  type: 'rapid' },
				{ label: '15 | 10', tc: 900,  inc: 10, type: 'rapid' },
				{ label: '30 min',  tc: 1800, inc: 0,  type: 'rapid' },
			]
		},
	];

	// Default: Rapid 10 min
	let selected: TC | null = $state(CATEGORIES[2].controls[0]);

	// ── Custom time control ───────────────────────────────────────────────────
	let customMode = $state(false);
	let customMin  = $state(5);
	let customSec  = $state(0);
	let customInc  = $state(0);

	// Secondi totali del custom
	const customTcSec = $derived(Math.max(1, customMin * 60 + customSec));

	// Categoria auto-rilevata per il custom
	function tcType(sec: number): 'bullet' | 'blitz' | 'rapid' {
		if (sec <= 179) return 'bullet';
		if (sec <= 600) return 'blitz';
		return 'rapid';
	}

	// Label leggibile del custom: "5 min", "5:30", "5:30 | 3"
	const customLabel = $derived((() => {
		const m = Math.floor(customTcSec / 60);
		const s = customTcSec % 60;
		const time = s === 0 ? `${m} min` : `${m}:${String(s).padStart(2, '0')}`;
		return customInc > 0 ? `${time} | ${customInc}` : time;
	})());

	// TC effettivo da usare (preset o custom)
	const activeTc    = $derived(customMode ? customTcSec          : (selected?.tc  ?? 600));
	const activeInc   = $derived(customMode ? customInc            : (selected?.inc ?? 0));
	const activeType  = $derived(customMode ? tcType(customTcSec)  : (selected?.type ?? 'rapid'));
	const activeLabel = $derived(customMode ? customLabel          : (selected?.label ?? '10 min'));

	// ELO da mostrare in base alla categoria selezionata
	const myElo = $derived((() => {
		if (!$user) return '—';
		switch (activeType) {
			case 'bullet': return $user.elo_bullet ?? $user.elo_rapid ?? '—';
			case 'blitz':  return $user.elo_blitz  ?? $user.elo_rapid ?? '—';
			default:       return $user.elo_rapid  ?? '—';
		}
	})());

	// ── Matchmaking ───────────────────────────────────────────────────────────
	let mm: 'idle' | 'searching' | 'found' | 'error' = $state('idle');
	let waitSeconds = $state(0);
	let errorMsg = $state('');
	let eventSource: EventSource | null = null;
	let waitTimer: ReturnType<typeof setInterval> | null = null;

	// ── Friend invite ─────────────────────────────────────────────────────────
	let fi: 'idle' | 'pending' | 'error' = $state('idle');
	let invitedUsername = $state('');
	let inviteError = $state('');

	// ── Online users polling ──────────────────────────────────────────────────
	let onlineInterval: ReturnType<typeof setInterval> | null = null;

	$effect(() => {
		if (!$authLoading && $user && !onlineInterval) {
			fetchOnlineUsers();
			onlineInterval = setInterval(fetchOnlineUsers, 15_000);
		}
	});

	onDestroy(() => {
		cleanup();
		if (onlineInterval) { clearInterval(onlineInterval); onlineInterval = null; }
	});

	// ── Matchmaking functions ─────────────────────────────────────────────────

	async function startSearch() {
		if (fi === 'pending') return;
		mm = 'searching';
		waitSeconds = 0;
		errorMsg = '';

		try {
			const res = await fetch(`${API}/api/matchmaking/join`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					time_control: activeTc,
					increment:    activeInc,
					game_type:    activeType,
				}),
			});
			if (!res.ok) throw new Error();
		} catch {
			mm = 'error';
			errorMsg = get(t).play.conn_error;
			return;
		}

		waitTimer = setInterval(() => waitSeconds++, 1000);

		eventSource = new EventSource(`${API}/api/matchmaking/stream`, {
			withCredentials: true
		} as EventSourceInit);

		eventSource.addEventListener('connected', () => {});

		eventSource.addEventListener('matched', (e: MessageEvent) => {
			const { game_id } = JSON.parse(e.data);
			mm = 'found';
			cleanup();
			setTimeout(() => goto(`/game/${game_id}`), 1200);
		});

		eventSource.onerror = () => {
			if (mm === 'searching') {
				mm = 'error';
				errorMsg = get(t).play.conn_lost;
				cleanup();
			}
		};
	}

	async function cancelSearch() {
		cleanup();
		await fetch(`${API}/api/matchmaking/leave`, { method: 'DELETE', credentials: 'include' });
		mm = 'idle';
		waitSeconds = 0;
	}

	function cleanup() {
		eventSource?.close();
		eventSource = null;
		if (waitTimer) { clearInterval(waitTimer); waitTimer = null; }
	}

	// ── Friend invite ─────────────────────────────────────────────────────────

	async function handleInvite(targetID: string, targetName: string) {
		if (mm === 'searching') await cancelSearch();
		invitedUsername = targetName;
		fi = 'pending';
		inviteError = '';
		try {
			await sendInvite(targetID, activeTc, activeInc);
		} catch (err: any) {
			fi = 'error';
			inviteError = err.message ?? get(t).play.err_invite;
		}
	}

	function cancelInvite() {
		fi = 'idle';
		invitedUsername = '';
	}

	// ── UI helpers ────────────────────────────────────────────────────────────

	function formatWait(s: number): string {
		const m = Math.floor(s / 60);
		const sec = s % 60;
		return m === 0 ? `${sec}s` : `${m}m ${sec}s`;
	}

	function eloRange(s: number): string {
		if (s < 10) return '±100';
		if (s < 20) return '±200';
		if (s < 30) return '±300';
		if (s < 60) return '±500';
		return get(t).play.any_elo;
	}

	function eloDiff(opponent: number): string {
		const base = Number($user?.elo_rapid ?? 100);
		const diff = opponent - base;
		return diff > 0 ? `+${diff}` : `${diff}`;
	}

	function isSelected(tc: TC): boolean {
		if (customMode) return false;
		return selected?.tc === tc.tc && selected?.inc === tc.inc;
	}

	function selectPreset(tc: TC) {
		selected = tc;
		customMode = false;
	}

	function enableCustom() {
		selected = null;
		customMode = true;
	}
</script>

<svelte:head>
	<title>{$t.play.page_title} — Chess</title>
</svelte:head>

<div class="play-page">

	<!-- ── Invito amico in sospeso ────────────────────────────────────── -->
	{#if fi === 'pending'}
		<div class="invite-waiting-box">
			<div class="spinner"></div>
			<p class="invite-waiting-text">{$t.play.invite_sent(invitedUsername)}</p>
			<p class="invite-waiting-sub">{$t.play.invite_waiting}</p>
			<button class="btn btn-google cancel-btn" onclick={cancelInvite}>{$t.play.invite_cancel}</button>
		</div>

	{:else if fi === 'error'}
		<div class="error-msg" style="max-width:340px;text-align:center">{inviteError}</div>
		<button class="btn btn-primary" onclick={cancelInvite}>{$t.common.ok}</button>

	{:else}

		<!-- ── Time control selector ────────────────────────────────────── -->
		{#if mm === 'idle'}
			<div class="tc-panel">

				{#each CATEGORIES as cat}
					<div class="tc-category">
						<div class="tc-cat-label">
							<span class="tc-cat-icon">{cat.icon}</span>
							{cat.name}
						</div>
						<div class="tc-grid">
							{#each cat.controls as tc}
								<button
									class="tc-btn"
									class:active={isSelected(tc)}
									onclick={() => selectPreset(tc)}
									disabled={mm !== 'idle'}
								>
									{tc.label}
								</button>
							{/each}
						</div>
					</div>
				{/each}

				<!-- ── Sezione personalizzata ──────────────────────────── -->
				<div class="tc-category">
					<div class="tc-cat-label">
						<span class="tc-cat-icon">⚙️</span>
						{$t.play.custom}
					</div>
					<button
						class="tc-btn custom-toggle"
						class:active={customMode}
						onclick={enableCustom}
						disabled={mm !== 'idle'}
					>
						{customMode ? customLabel : $t.play.set_custom}
					</button>

					{#if customMode}
						<div class="custom-inputs">
							<div class="custom-field">
								<label>{$t.play.minutes}</label>
								<input
									type="number" min="0" max="180"
									bind:value={customMin}
									disabled={mm !== 'idle'}
								/>
							</div>
							<div class="custom-field">
								<label>{$t.play.seconds}</label>
								<input
									type="number" min="0" max="59"
									bind:value={customSec}
									disabled={mm !== 'idle'}
								/>
							</div>
							<div class="custom-field">
								<label>{$t.play.inc_per_move}</label>
								<input
									type="number" min="0" max="60"
									bind:value={customInc}
									disabled={mm !== 'idle'}
								/>
							</div>
						</div>
						<p class="custom-preview">
							<span class="custom-type-badge">{activeType}</span>
							{customLabel}
						</p>
					{/if}
				</div>

				<!-- ELO info + azioni -->
				<div class="tc-footer">
					<span class="tc-elo">
						{$t.play.your_elo(activeType, myElo)}
					</span>
					<div class="play-options">
						<button
							class="btn btn-primary play-btn"
							onclick={startSearch}
							disabled={customMode && customTcSec < 1}
						>
							{$t.play.find_game}
						</button>
						<a href="/play/bot" class="bot-btn">
							{$t.play.play_vs_bot}
						</a>
					</div>
				</div>
			</div>

		<!-- ── Searching ───────────────────────────────────────────────── -->
		{:else if mm === 'searching'}
			<div class="tc-info-badge">
				{customMode ? '⚙️' : (CATEGORIES.find(c => c.controls.some(tc => tc.tc === selected?.tc && tc.inc === selected?.inc))?.icon ?? '🕐')}
				{activeLabel} · {activeType}
			</div>
			<div class="searching-box">
				<div class="spinner"></div>
				<p class="wait-time">{$t.play.searching(formatWait(waitSeconds))}</p>
				<p class="elo-info">{$t.play.searching_opponent(eloRange(waitSeconds))}</p>
				<button class="btn btn-google cancel-btn" onclick={cancelSearch}>{$t.play.cancel_search}</button>
			</div>

		<!-- ── Found ────────────────────────────────────────────────────── -->
		{:else if mm === 'found'}
			<div class="found-box">
				<div class="found-icon">⚡</div>
				<p>{$t.play.match_found}</p>
			</div>

		<!-- ── Error ────────────────────────────────────────────────────── -->
		{:else if mm === 'error'}
			<div class="error-msg" style="max-width:340px;text-align:center">{errorMsg}</div>
			<button class="btn btn-primary" onclick={startSearch}>{$t.common.retry}</button>
		{/if}

	{/if}

	<!-- ── Giocatori online ──────────────────────────────────────────────── -->
	{#if fi === 'idle' && mm !== 'found'}
		<section class="online-section">
			<h3 class="online-title">
				<span class="online-dot"></span>
				{$t.play.online_players}
				{#if $onlineUsers.length > 0}
					<span class="online-count">({$onlineUsers.length})</span>
				{/if}
			</h3>

			{#if $onlineUsers.length === 0}
				<p class="online-empty">{$t.play.no_online}</p>
			{:else}
				<ul class="online-list">
					{#each $onlineUsers as u (u.id)}
						<li class="online-item">
							<div class="online-item-left">
								<span class="online-avatar">{u.username[0].toUpperCase()}</span>
								<div class="online-item-info">
									<span class="online-item-name">{u.username}</span>
									<span class="online-item-elo">
										ELO {u.elo_rapid}
										<span
											class="elo-diff"
											class:positive={u.elo_rapid > (Number($user?.elo_rapid) ?? 0)}
											class:negative={u.elo_rapid < (Number($user?.elo_rapid) ?? 0)}
										>({eloDiff(u.elo_rapid)})</span>
									</span>
								</div>
							</div>
							<button
								class="btn btn-primary invite-btn"
								onclick={() => handleInvite(u.id, u.username)}
								disabled={mm === 'searching'}
							>
								{$t.play.challenge}
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	{/if}

</div>

<style>
	.play-page {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100dvh;
		overflow: hidden;
		gap: clamp(0.75rem, 1.5dvh, 1.75rem);
		padding: clamp(0.5rem, 1.5dvh, 1.5rem) 1.5rem;
	}

	/* ── Time control panel ── */
	.tc-panel {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 14px;
		padding: clamp(0.75rem, 1.5dvh, 1.5rem) clamp(1rem, 2vw, 1.75rem);
		width: 100%;
		max-width: 400px;
		display: flex;
		flex-direction: column;
		gap: clamp(0.6rem, 1.2dvh, 1.25rem);
		flex-shrink: 0;
	}

	.tc-category { display: flex; flex-direction: column; gap: 0.5rem; }

	.tc-cat-label {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.85rem;
		font-weight: 700;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}
	.tc-cat-icon { font-size: 1rem; }

	.tc-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.5rem;
	}

	.tc-btn {
		padding: 0.7rem 0.25rem;
		font-size: 0.9rem;
		font-weight: 600;
		background: var(--bg);
		border: 2px solid var(--border);
		border-radius: 8px;
		color: var(--text);
		cursor: pointer;
		transition: border-color 0.12s, background 0.12s, color 0.12s;
	}
	.tc-btn:hover:not(:disabled) { border-color: var(--accent); }
	.tc-btn.active {
		border-color: var(--accent);
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
	}
	.tc-btn:disabled { opacity: 0.45; cursor: default; }

	.tc-footer {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.9rem;
		padding-top: 0.5rem;
		border-top: 1px solid var(--border);
	}
	.tc-elo { font-size: 0.85rem; color: var(--text-muted); }

	.play-options {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.6rem;
		width: 100%;
	}

	.play-btn { width: 100%; padding: 0.9rem; font-size: 1rem; }

	.bot-btn {
		width: 100%;
		padding: 0.75rem;
		font-size: 0.9rem;
		text-align: center;
		text-decoration: none;
		background: transparent;
		border: 1.5px solid var(--border);
		color: var(--text-muted);
		border-radius: 8px;
		transition: border-color 0.12s, color 0.12s;
		display: block;
	}
	.bot-btn:hover { border-color: var(--accent); color: var(--text); }

	/* ── Searching badge ── */
	.tc-info-badge {
		background: var(--bg-card);
		border: 2px solid var(--accent);
		border-radius: 20px;
		padding: 0.4rem 1rem;
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--accent);
	}

	/* ── Searching ── */
	.searching-box {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.75rem;
	}
	.spinner {
		width: 48px; height: 48px;
		border: 4px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin { to { transform: rotate(360deg); } }
	.wait-time { font-size: 1.1rem; }
	.elo-info { font-size: 0.85rem; color: var(--text-muted); }
	.cancel-btn { width: 180px; margin-top: 0.5rem; }

	/* ── Found ── */
	.found-box {
		display: flex; flex-direction: column; align-items: center; gap: 0.75rem;
		animation: fadeIn 0.3s ease;
	}
	.found-icon { font-size: 3rem; animation: bounce 0.6s ease infinite alternate; }
	@keyframes bounce { to { transform: translateY(-8px); } }
	@keyframes fadeIn { from { opacity: 0; transform: scale(0.95); } to { opacity: 1; transform: scale(1); } }

	/* ── Invite waiting ── */
	.invite-waiting-box {
		display: flex; flex-direction: column; align-items: center; gap: 0.6rem;
		background: var(--bg-card);
		border: 2px solid var(--accent);
		border-radius: 12px;
		padding: 2rem 2.5rem;
		text-align: center;
	}
	.invite-waiting-text { font-size: 1.05rem; }
	.invite-waiting-sub { font-size: 0.85rem; color: var(--text-muted); }

	/* ── Online section ── */
	.online-section {
		width: 100%;
		max-width: 400px;
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow: hidden;
	}
	.online-title {
		display: flex; align-items: center; gap: 0.5rem;
		font-size: 0.85rem; font-weight: 600; color: var(--text-muted);
		margin-bottom: 0.75rem;
		text-transform: uppercase; letter-spacing: 0.06em;
	}
	.online-dot {
		width: 8px; height: 8px;
		background: #2ecc71; border-radius: 50%;
		box-shadow: 0 0 6px #2ecc71;
		animation: pulse 2s ease infinite; flex-shrink: 0;
	}
	@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.35; } }
	.online-count { color: var(--text-muted); font-weight: 400; }
	.online-empty { color: var(--text-muted); font-size: 0.9rem; text-align: center; padding: 1.5rem 0; }
	.online-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.5rem; overflow-y: auto; max-height: clamp(120px, 28dvh, 360px); }
	.online-item {
		display: flex; align-items: center; justify-content: space-between;
		background: var(--bg-card); border: 1px solid var(--border);
		border-radius: 10px; padding: 0.75rem 1rem; gap: 0.75rem;
		transition: border-color 0.15s;
	}
	.online-item:hover { border-color: var(--accent); }
	.online-item-left { display: flex; align-items: center; gap: 0.75rem; flex: 1; min-width: 0; }
	.online-avatar {
		width: 36px; height: 36px;
		background: var(--accent); border-radius: 50%;
		display: flex; align-items: center; justify-content: center;
		font-weight: 700; font-size: 0.95rem; flex-shrink: 0;
	}
	.online-item-info { display: flex; flex-direction: column; gap: 0.15rem; min-width: 0; }
	.online-item-name { font-weight: 600; font-size: 0.95rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
	.online-item-elo { font-size: 0.8rem; color: var(--text-muted); }
	.elo-diff { font-size: 0.75rem; }
	.elo-diff.positive { color: #2ecc71; }
	.elo-diff.negative { color: #e74c3c; }
	.invite-btn {
		width: auto !important; padding: 0.45rem 1rem !important;
		font-size: 0.875rem !important; flex-shrink: 0; border-radius: 8px !important;
	}

	/* ── Custom time control ── */
	.custom-toggle {
		width: 100%;
		font-size: 0.875rem;
	}

	/* ── Mobile (≤ 768px) ── */
	@media (max-width: 768px) {
		.play-page {
			padding: clamp(0.4rem, 1dvh, 1rem) 0.75rem;
			gap: clamp(0.5rem, 1dvh, 1rem);
		}
		.tc-panel {
			max-width: 100%;
			padding: clamp(0.6rem, 1.5dvh, 1.25rem) 1rem;
		}
		.online-section {
			max-width: 100%;
		}
		/* Griglia TC: 2 colonne su schermi molto stretti */
		@media (max-width: 360px) {
			.tc-grid {
				grid-template-columns: repeat(2, 1fr);
			}
		}
	}

	.custom-inputs {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.6rem;
		margin-top: 0.25rem;
	}

	.custom-field {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}
	.custom-field label {
		font-size: 0.7rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		font-weight: 600;
	}
	.custom-field input {
		width: 100%;
		padding: 0.55rem 0.5rem;
		background: var(--bg);
		border: 2px solid var(--border);
		border-radius: 6px;
		color: var(--text);
		font-size: 1rem;
		font-weight: 700;
		text-align: center;
		transition: border-color 0.12s;
	}
	.custom-field input:focus {
		outline: none;
		border-color: var(--accent);
	}
	.custom-field input::-webkit-inner-spin-button { opacity: 0.4; }

	.custom-preview {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.85rem;
		color: var(--text-muted);
		margin-top: 0.25rem;
	}
	.custom-type-badge {
		font-size: 0.7rem;
		font-weight: 700;
		text-transform: uppercase;
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
		border-radius: 4px;
		padding: 0.1rem 0.4rem;
	}
</style>
