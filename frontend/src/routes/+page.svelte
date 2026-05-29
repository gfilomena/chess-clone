<script lang="ts">
	import { onMount } from 'svelte';
	import logo from '$lib/assets/logo.svg';
	import { user, authLoading } from '$lib/stores/auth';
	import { t } from '$lib/i18n';
	import { API_URL as API } from '$lib/config';

	let activeGameId = $state<string | null>(null);

	onMount(async () => {
		// Controlla se l'utente ha una partita attiva (non-bot)
		if (!$user) return;
		try {
			const res = await fetch(`${API}/api/games/active`, { credentials: 'include' });
			if (res.ok) {
				const json = await res.json();
				activeGameId = json.data?.game_id ?? null;
			}
		} catch {
			// silenzioso
		}
	});

	// Ricontrolla quando il login si completa
	$effect(() => {
		if ($user && !$authLoading) {
			fetch(`${API}/api/games/active`, { credentials: 'include' })
				.then(r => r.json())
				.then(j => { activeGameId = j.data?.game_id ?? null; })
				.catch(() => {});
		} else if (!$user) {
			activeGameId = null;
		}
	});
</script>

<svelte:head>
	<title>Chess</title>
</svelte:head>

<div class="home">

	<!-- ── Hero ─────────────────────────────────────────────────── -->
	<div class="hero">

		<img src={logo} alt="Chess" class="logo" draggable="false" />

		{#if $authLoading}
			<p class="muted">{$t.home.loading}</p>

		{:else if $user}
			<p class="welcome">{$t.home.welcome}, <strong>{$user.username}</strong>!</p>
			<div class="elo-row">
				<span class="elo-chip bullet">🚀 {$user.elo_bullet ?? 100} Bullet</span>
				<span class="elo-chip blitz">⚡ {$user.elo_blitz ?? 100} Blitz</span>
				<span class="elo-chip rapid">🕐 {$user.elo_rapid ?? 100} Rapid</span>
			</div>
			<div class="cta-row">
				<a href="/play" class="btn btn-primary cta">{$t.home.play_game}</a>
				{#if activeGameId}
					<a href="/game/{activeGameId}" class="btn resume-btn">
						🟢 {$t.home.resume_game}
					</a>
				{/if}
				<a href="/play/bot" class="btn-outline">{$t.home.vs_bot}</a>
			</div>

		{:else}
			<h1 class="tagline">{@html $t.home.tagline.replace('\n', '<br>')}</h1>
			<p class="sub">{$t.home.sub}</p>
			<div class="cta-row">
				<a href="/login" class="btn btn-primary cta">{$t.home.cta_login}</a>
				<a href="/register" class="btn-outline">{$t.home.cta_register}</a>
			</div>
		{/if}
	</div>

	<!-- ── Feature strip ─────────────────────────────────────────── -->
	<div class="features">
		<div class="feat">
			<span class="feat-icon">♜</span>
			<span>{$t.home.feat_matchmaking}</span>
		</div>
		<div class="feat">
			<span class="feat-icon">🤖</span>
			<span>{$t.home.feat_bot}</span>
		</div>
		<div class="feat">
			<span class="feat-icon">⚡</span>
			<span>{$t.home.feat_formats}</span>
		</div>
		<div class="feat">
			<span class="feat-icon">📱</span>
			<span>{$t.home.feat_mobile}</span>
		</div>
	</div>

</div>

<style>
	.home {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		padding: 2rem 1.5rem 4rem;
		gap: 3rem;
	}

	/* ── Hero ── */
	.hero {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1.5rem;
		text-align: center;
		max-width: 520px;
	}

	.logo {
		width: min(320px, 90vw);
		height: auto;
		user-select: none;
	}

	.tagline {
		font-size: clamp(1.6rem, 5vw, 2.3rem);
		font-weight: 800;
		line-height: 1.25;
		color: var(--text);
		margin: 0;
		letter-spacing: -0.02em;
	}

	.sub {
		font-size: 1rem;
		color: var(--text-muted);
		line-height: 1.55;
		margin: 0;
	}

	.welcome {
		font-size: 1.3rem;
		color: var(--text);
		margin: 0;
	}
	.welcome strong { color: var(--accent); }
	.muted { color: var(--text-muted); margin: 0; }

	/* ── ELO chips ── */
	.elo-row {
		display: flex;
		gap: 0.6rem;
		flex-wrap: wrap;
		justify-content: center;
	}
	.elo-chip {
		font-size: 0.82rem;
		font-weight: 600;
		padding: 0.3rem 0.85rem;
		border-radius: 20px;
		border: 1.5px solid var(--border);
		background: var(--bg-card);
		color: var(--text-muted);
	}
	.elo-chip.rapid  { border-color: #81B64C; color: #81B64C; }
	.elo-chip.blitz  { border-color: #e6a817; color: #e6a817; }
	.elo-chip.bullet { border-color: #e05050; color: #e05050; }

	/* ── CTA ── */
	.cta-row {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.75rem;
		width: 100%;
		max-width: 300px;
	}
	.cta {
		width: 100%;
		padding: 0.85rem 1.5rem;
		font-size: 1.05rem;
	}
	.resume-btn {
		width: 100%;
		padding: 0.75rem 1.5rem;
		font-size: 0.95rem;
		text-align: center;
		background: color-mix(in srgb, #2ecc71 12%, transparent);
		border: 2px solid #2ecc71;
		color: #2ecc71;
		border-radius: 8px;
		text-decoration: none;
		font-weight: 600;
		transition: background 0.15s, color 0.15s;
		animation: pulse-green 2s ease-in-out infinite;
	}
	.resume-btn:hover {
		background: color-mix(in srgb, #2ecc71 22%, transparent);
		color: #2ecc71;
		text-decoration: none;
	}
	@keyframes pulse-green {
		0%, 100% { box-shadow: 0 0 0 0 rgba(46, 204, 113, 0); }
		50%       { box-shadow: 0 0 0 6px rgba(46, 204, 113, 0.15); }
	}

	.btn-outline {
		color: var(--text-muted);
		font-size: 0.9rem;
		font-weight: 500;
		text-decoration: none;
		padding: 0.4rem;
		border-radius: 6px;
		transition: color 0.15s;
	}
	.btn-outline:hover { color: var(--text); text-decoration: none; }

	/* ── Feature strip ── */
	.features {
		display: flex;
		gap: 0.75rem;
		flex-wrap: wrap;
		justify-content: center;
	}
	.feat {
		display: flex;
		align-items: center;
		gap: 0.45rem;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 0.6rem 1rem;
		font-size: 0.84rem;
		color: var(--text-muted);
		font-weight: 500;
	}
	.feat-icon { font-size: 1rem; line-height: 1; }

	/* ── Mobile ── */
	@media (max-width: 768px) {
		.home { padding-top: 1rem; gap: 2rem; }
		.logo { width: min(260px, 85vw); }
	}
</style>
