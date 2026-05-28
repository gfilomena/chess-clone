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

	let { children } = $props();

	onMount(() => loadUser());
	onDestroy(() => { stopHeartbeat(); stopInviteSSE(); });

	$effect(() => {
		if ($user) { startHeartbeat(); startInviteSSE(); }
		else        { stopHeartbeat(); stopInviteSSE(); }
	});

	async function handleLogout() {
		stopHeartbeat();
		stopInviteSSE();
		await logout();
		window.location.href = '/';
	}

	// Active route highlight
	const currentPath = $derived($page.url.pathname);
	function isActive(path: string) {
		return currentPath === path || currentPath.startsWith(path + '/');
	}

	// User initial for avatar
	const initial = $derived($user?.username?.[0]?.toUpperCase() ?? '?');
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Chess Clone</title>
</svelte:head>

<div class="app-shell">

	<!-- ── Left sidebar ─────────────────────────────────────── -->
	<aside class="sidebar">

		<!-- Logo -->
		<a href="/" class="sidebar-logo">
			<span class="sidebar-logo-icon">♟</span>
			<span class="sidebar-logo-text">Chess Clone</span>
		</a>

		<!-- Nav items -->
		<nav class="sidebar-nav">
			<a href="/play" class="nav-item" class:active={isActive('/play')}>
				<span class="nav-icon">🎮</span>
				<span>Gioca</span>
			</a>
			<span class="nav-item disabled">
				<span class="nav-icon">🧩</span>
				<span>Problemi</span>
			</span>
			<span class="nav-item disabled">
				<span class="nav-icon">🎓</span>
				<span>Impara</span>
			</span>
			<span class="nav-item disabled">
				<span class="nav-icon">🏋️</span>
				<span>Allenati</span>
			</span>
			<span class="nav-item disabled">
				<span class="nav-icon">📺</span>
				<span>Guarda</span>
			</span>
			<span class="nav-item disabled">
				<span class="nav-icon">👥</span>
				<span>Community</span>
			</span>

			<div class="nav-divider"></div>

			<span class="nav-item disabled">
				<span class="nav-icon">⋯</span>
				<span>Altro</span>
			</span>
		</nav>

		<!-- Bottom: user info -->
		<div class="sidebar-bottom">
			{#if $user}
				<div class="user-row">
					<div class="user-avatar">{initial}</div>
					<div class="user-info">
						<div class="user-name">{$user.username}</div>
						<div class="user-elo">{$user.elo_rapid} ELO</div>
					</div>
					<button class="logout-btn" onclick={handleLogout} title="Esci">⏏</button>
				</div>
			{:else}
				<a href="/login" class="nav-item">
					<span class="nav-icon">🔑</span>
					<span>Accedi</span>
				</a>
				<a href="/register" class="nav-item">
					<span class="nav-icon">✨</span>
					<span>Registrati</span>
				</a>
			{/if}
		</div>
	</aside>

	<!-- ── Main content ──────────────────────────────────────── -->
	<main class="main-content">
		{@render children()}
	</main>

</div>

<!-- Toast inviti — visibile in ogni pagina -->
<InviteToast />
