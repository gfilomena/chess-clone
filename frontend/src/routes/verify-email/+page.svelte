<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { verifyEmail } from '$lib/stores/auth';
	import { t } from '$lib/i18n';

	type Status = 'checking' | 'success' | 'error';
	let status = $state<Status>('checking');

	$effect(() => {
		const token = $page.url.searchParams.get('token');
		if (!token) {
			status = 'error';
			return;
		}

		verifyEmail(token)
			.then(() => {
				status = 'success';
				// Dopo 1.5s → home (il cookie JWT è già stato impostato dal backend)
				setTimeout(() => goto('/'), 1500);
			})
			.catch(() => {
				status = 'error';
			});
	});
</script>

<svelte:head>
	<title>Chess — {$t.auth.verify_page_title}</title>
</svelte:head>

<div class="verify-card">
	{#if status === 'checking'}
		<div class="spinner-wrap">
			<div class="spinner"></div>
		</div>
		<p class="verify-msg">{$t.auth.verify_page_checking}</p>

	{:else if status === 'success'}
		<div class="icon-ok">✓</div>
		<p class="verify-msg success">{$t.auth.verify_page_success}</p>

	{:else}
		<div class="icon-err">✕</div>
		<p class="verify-msg error">{$t.auth.verify_page_error}</p>
		<a href="/login" class="btn-back">{$t.auth.verify_page_back}</a>
	{/if}
</div>

<style>
	.verify-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		min-height: 50vh;
		gap: 1rem;
		padding: 2rem;
		text-align: center;
	}

	.spinner-wrap {
		margin-bottom: 0.5rem;
	}

	.spinner {
		width: 42px;
		height: 42px;
		border: 3px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}

	@keyframes spin { to { transform: rotate(360deg); } }

	.icon-ok,
	.icon-err {
		width: 56px;
		height: 56px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 1.6rem;
		font-weight: 700;
	}

	.icon-ok {
		background: rgba(106, 168, 79, 0.15);
		color: var(--accent);
		border: 2px solid var(--accent);
	}

	.icon-err {
		background: rgba(220, 53, 69, 0.12);
		color: #dc3545;
		border: 2px solid #dc3545;
	}

	.verify-msg {
		font-size: 1rem;
		color: var(--text-secondary);
		margin: 0;
	}

	.verify-msg.success { color: var(--accent); font-weight: 600; }
	.verify-msg.error   { color: #dc3545; }

	.btn-back {
		margin-top: 0.5rem;
		padding: 0.55rem 1.4rem;
		border: 1px solid var(--border);
		border-radius: 8px;
		color: var(--text-secondary);
		text-decoration: none;
		font-size: 0.88rem;
		transition: border-color 0.15s, color 0.15s;
	}

	.btn-back:hover {
		border-color: var(--accent);
		color: var(--text);
	}
</style>
