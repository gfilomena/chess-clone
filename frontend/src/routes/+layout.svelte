<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { user, loadUser, logout } from '$lib/stores/auth';
	import {
		startHeartbeat, stopHeartbeat,
		startInviteSSE, stopInviteSSE
	} from '$lib/stores/invitations';
	import InviteToast from '$lib/components/InviteToast.svelte';
	import { t, lang, setLang, LANGS } from '$lib/i18n';

	let { children } = $props();

	let sidebarOpen  = $state(false);
	let userMenuOpen = $state(false);

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
</svelte:head>

<!-- ── Mobile top bar (solo < 768px) ───────────────────────── -->
<header class="mobile-header">
	<button
		class="mobile-hamburger"
		onclick={() => sidebarOpen = !sidebarOpen}
		aria-label="Menu"
	>
		{sidebarOpen ? '✕' : '☰'}
	</button>
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
	{/if}
</header>

<!-- ── Backdrop sidebar (mobile) ───────────────────────────── -->
<div
	class="sidebar-backdrop"
	class:sidebar-open={sidebarOpen}
	onclick={() => sidebarOpen = false}
	aria-hidden="true"
></div>

<div class="app-shell">

	<!-- ── Left sidebar ─────────────────────────────────────── -->
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
			{#if $user}
				<div class="user-row">
					<a href="/profile/{$user.id}" class="user-avatar-link" onclick={() => sidebarOpen = false}>
						<div class="user-avatar">{initial}</div>
					</a>
					<a href="/profile/{$user.id}" class="user-info" onclick={() => sidebarOpen = false}>
						<div class="user-name">{$user.username}</div>
						<div class="user-elo">{$user.elo_rapid} ELO</div>
					</a>
					<button class="logout-btn" onclick={handleLogout} title={$t.user.logout}>⏏</button>
				</div>
			{:else}
				<a href="/login" class="nav-item" onclick={() => sidebarOpen = false}>
					<span class="nav-icon">🔑</span>
					<span>{$t.auth.sign_in}</span>
				</a>
				<a href="/register" class="nav-item" onclick={() => sidebarOpen = false}>
					<span class="nav-icon">✨</span>
					<span>{$t.auth.sign_up}</span>
				</a>
			{/if}

			<!-- Language switcher -->
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
		</div>
	</aside>

	<!-- ── Main content ──────────────────────────────────────── -->
	<main class="main-content">
		{@render children()}
	</main>

</div>

<!-- Toast inviti — visibile in ogni pagina -->
<InviteToast />

<style>
	.lang-switcher {
		display: flex;
		gap: 0.35rem;
		padding: 0.6rem 0.5rem 0.3rem;
		border-top: 1px solid var(--border);
		margin-top: 0.5rem;
	}
	.lang-btn {
		background: none;
		border: 1.5px solid transparent;
		border-radius: 6px;
		padding: 0.2rem 0.3rem;
		font-size: 1.1rem;
		cursor: pointer;
		opacity: 0.5;
		transition: opacity 0.15s, border-color 0.15s;
		line-height: 1;
	}
	.lang-btn:hover { opacity: 0.8; }
	.lang-btn.active {
		opacity: 1;
		border-color: var(--accent);
	}
</style>
