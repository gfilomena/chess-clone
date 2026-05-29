<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { user, authLoading, loadUser, logout } from '$lib/stores/auth';
	import {
		startHeartbeat, stopHeartbeat,
		startInviteSSE, stopInviteSSE
	} from '$lib/stores/invitations';
	import InviteToast from '$lib/components/InviteToast.svelte';
	import CookieBanner from '$lib/components/CookieBanner.svelte';
	import { t, lang, setLang, LANGS } from '$lib/i18n';

	let { children } = $props();

	let sidebarOpen  = $state(false);
	let userMenuOpen = $state(false);

	// Sidebar visibile solo quando l'utente è autenticato
	const showSidebar = $derived(!!$user);

	onMount(() => loadUser());
	onDestroy(() => { stopHeartbeat(); stopInviteSSE(); });

	$effect(() => {
		if ($user) { startHeartbeat(); startInviteSSE(); }
		else        { stopHeartbeat(); stopInviteSSE(); }
	});

	// Chiudi sidebar e menu utente ad ogni navigazione
	const currentPath = $derived($page.url.pathname);
	$effect(() => {
		currentPath;
		sidebarOpen  = false;
		userMenuOpen = false;
	});

	async function handleLogout() {
		stopHeartbeat();
		stopInviteSSE();
		await logout();
		window.location.href = '/';
	}

	function isActive(path: string) {
		return currentPath === path || currentPath.startsWith(path + '/');
	}

	const initial = $derived($user?.username?.[0]?.toUpperCase() ?? '?');
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Chess</title>
	<meta name="description" content="Scacchi online gratuiti con matchmaking ELO, bot Stockfish e analisi partite. Rapid, Blitz, Bullet — nessun abbonamento." />
</svelte:head>

<!-- ── Mobile top bar (solo < 768px) ───────────────────────── -->
<header class="mobile-header">
	{#if showSidebar}
		<button
			class="mobile-hamburger"
			onclick={() => sidebarOpen = !sidebarOpen}
			aria-label="Menu"
		>
			{sidebarOpen ? '✕' : '☰'}
		</button>
	{:else}
		<!-- Placeholder per mantenere il logo centrato -->
		<div class="mobile-hamburger-placeholder"></div>
	{/if}

	<img src={favicon} alt="" class="mobile-logo-icon" aria-hidden="true" />
	<span class="mobile-logo-text">Chess</span>

	{#if $user}
		<div class="user-chip-wrap">
			<button
				class="mobile-user-chip"
				onclick={() => userMenuOpen = !userMenuOpen}
				aria-label="Menu utente"
			>{initial}</button>
			{#if userMenuOpen}
				<div class="user-menu-backdrop" onclick={() => userMenuOpen = false} aria-hidden="true"></div>
				<div class="user-dropdown">
					<a href="/profile/{$user.id}" class="dropdown-item" onclick={() => userMenuOpen = false}>
						👤 {$t.user.profile}
					</a>
					<button class="dropdown-item dropdown-logout" onclick={handleLogout}>
						⏏ {$t.user.logout}
					</button>
				</div>
			{/if}
		</div>
	{:else}
		<div class="mobile-hamburger-placeholder"></div>
	{/if}
</header>

<!-- ── Backdrop sidebar (mobile) ───────────────────────────── -->
{#if showSidebar}
	<div
		class="sidebar-backdrop"
		class:sidebar-open={sidebarOpen}
		onclick={() => sidebarOpen = false}
		aria-hidden="true"
	></div>
{/if}

<div class="app-shell" class:no-sidebar={!showSidebar}>

	<!-- ── Left sidebar (solo utenti loggati) ───────────────── -->
	{#if showSidebar}
		<aside class="sidebar" class:sidebar-open={sidebarOpen}>

			<a href="/" class="sidebar-logo" onclick={() => sidebarOpen = false}>
				<img src={favicon} alt="" class="sidebar-logo-img" aria-hidden="true" />
				<span class="sidebar-logo-text">Chess</span>
			</a>

			<nav class="sidebar-nav">
				<a href="/play" class="nav-item" class:active={isActive('/play')} onclick={() => sidebarOpen = false}>
					<span class="nav-icon">🎮</span>
					<span>{$t.nav.play}</span>
				</a>
				<a href="/leaderboard" class="nav-item" class:active={isActive('/leaderboard')} onclick={() => sidebarOpen = false}>
					<span class="nav-icon">🏆</span>
					<span>{$t.nav.leaderboard}</span>
				</a>
			</nav>

			<div class="sidebar-bottom">
				<div class="user-row">
					<a href="/profile/{$user!.id}" class="user-avatar-link" onclick={() => sidebarOpen = false}>
						<div class="user-avatar">{initial}</div>
					</a>
					<a href="/profile/{$user!.id}" class="user-info" onclick={() => sidebarOpen = false}>
						<div class="user-name">{$user!.username}</div>
						<div class="user-elo">{$user!.elo_rapid} ELO</div>
					</a>
					<button class="logout-btn" onclick={handleLogout} title={$t.user.logout}>⏏</button>
				</div>
			</div>
		</aside>
	{/if}

	<!-- ── Main content ──────────────────────────────────────── -->
	<main class="main-content">
		{@render children()}
	</main>

</div>

<!-- ── Bottom bar fissa — About · Privacy · Lingue ─────────── -->
<footer class="bottom-bar">
	<nav class="bottom-links">
		<a href="/about" class="bottom-link">{$t.nav.about}</a>
		<span class="bottom-sep">·</span>
		<a href="/privacy" class="bottom-link">{$t.nav.privacy}</a>
	</nav>
	<div class="bottom-divider"></div>
	<div class="lang-switcher">
		{#each LANGS as l}
			<button
				class="lang-btn"
				class:active={$lang === l.code}
				onclick={() => setLang(l.code)}
				title={l.label}
			>{l.flag}</button>
		{/each}
	</div>
</footer>

<!-- Toast inviti — visibile in ogni pagina -->
<InviteToast />

<!-- Cookie consent banner (GDPR) -->
<CookieBanner />

<style>
	/* ── Bottom bar fissa ── */
	.bottom-bar {
		position: fixed;
		bottom: 0;
		left: 0;
		right: 0;
		height: 40px;
		z-index: 200;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 1rem;
		background: var(--sidebar-bg, #1a1a1a);
		border-top: 1px solid var(--border);
	}

	.bottom-links {
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}
	.bottom-link {
		font-size: 0.75rem;
		color: var(--text-muted, #888);
		text-decoration: none;
		opacity: 0.7;
		transition: opacity 0.15s;
	}
	.bottom-link:hover { opacity: 1; }
	.bottom-sep {
		font-size: 0.75rem;
		color: var(--text-muted, #888);
		opacity: 0.4;
	}
	.bottom-divider {
		width: 1px;
		height: 16px;
		background: var(--border);
		opacity: 0.5;
	}

	/* ── Language switcher ── */
	.lang-switcher {
		display: flex;
		gap: 0.25rem;
	}
	.lang-btn {
		background: none;
		border: 1.5px solid transparent;
		border-radius: 5px;
		padding: 0.1rem 0.25rem;
		font-size: 1rem;
		cursor: pointer;
		opacity: 0.5;
		transition: opacity 0.15s, border-color 0.15s;
		line-height: 1;
	}
	.lang-btn:hover { opacity: 0.85; }
	.lang-btn.active {
		opacity: 1;
		border-color: var(--accent);
	}

	/* ── Mobile hamburger placeholder (centra il logo) ── */
	.mobile-hamburger-placeholder {
		width: 40px;
		height: 40px;
		flex-shrink: 0;
	}
</style>
