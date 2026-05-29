<script lang="ts">
	import { register, user, authLoading } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { API_URL } from '$lib/config';
	import { t } from '$lib/i18n';
	import { resendVerification } from '$lib/stores/auth';

	// Se già autenticato → vai alla home
	$effect(() => {
		if (!$authLoading && $user) {
			goto('/');
		}
	});

	let username = $state('');
	let email    = $state('');
	let password = $state('');
	let confirm  = $state('');
	let error    = $state('');
	let loading  = $state(false);

	// Stato post-registrazione
	let emailSent     = $state(false);
	let sentToEmail   = $state('');
	let resendLoading = $state(false);
	let resendDone    = $state(false);

	async function handleRegister(e: Event) {
		e.preventDefault();
		error = '';

		if (password !== confirm) {
			error = $t.auth.err_passwords;
			return;
		}
		if (password.length < 8) {
			error = $t.auth.err_pwd_len;
			return;
		}

		loading = true;
		try {
			const data = await register(username, email, password);
			sentToEmail = data.email ?? email;
			emailSent = true;
		} catch (err: any) {
			error = err.message ?? $t.auth.err_register;
		} finally {
			loading = false;
		}
	}

	async function handleResend() {
		resendLoading = true;
		resendDone = false;
		try {
			await resendVerification(sentToEmail);
			resendDone = true;
		} catch {
			// ignora — il backend risponde sempre 200
			resendDone = true;
		} finally {
			resendLoading = false;
		}
	}
</script>

<svelte:head>
	<title>Chess</title>
</svelte:head>

{#if $authLoading}
	<div class="auth-checking">
		<div class="auth-spinner"></div>
	</div>

{:else if emailSent}
	<!-- ── Stato: email di verifica inviata ───────────────────────────────── -->
	<div class="form-card email-sent-card">
		<div class="email-icon">✉️</div>
		<h1>{$t.auth.verify_sent_title}</h1>
		<p class="sent-desc">{$t.auth.verify_sent_desc}</p>
		<p class="sent-email">{sentToEmail}</p>
		<p class="sent-spam">{$t.auth.verify_sent_spam}</p>

		<div class="resend-row">
			{#if resendDone}
				<span class="resent-ok">{$t.auth.verify_resent}</span>
			{:else}
				<button
					class="btn btn-secondary"
					onclick={handleResend}
					disabled={resendLoading}
				>
					{resendLoading ? $t.auth.verify_resending : $t.auth.verify_resend}
				</button>
			{/if}
		</div>

		<p class="form-footer">
			{$t.auth.have_account} <a href="/login">{$t.auth.sign_in}</a>
		</p>
	</div>

{:else}
	<!-- ── Form registrazione ─────────────────────────────────────────────── -->
	<div class="form-card">
		<h1>{$t.auth.register_title}</h1>

		{#if error}
			<div class="error-msg">{error}</div>
		{/if}

		<form onsubmit={handleRegister}>
			<div class="field">
				<label for="username">{$t.auth.username}</label>
				<input
					id="username"
					type="text"
					bind:value={username}
					placeholder="il_tuo_username"
					required
					minlength="3"
					maxlength="30"
					autocomplete="username"
				/>
			</div>

			<div class="field">
				<label for="email">{$t.auth.email}</label>
				<input
					id="email"
					type="email"
					bind:value={email}
					placeholder={$t.auth.email_placeholder}
					required
					autocomplete="email"
				/>
			</div>

			<div class="field">
				<label for="password">{$t.auth.password}</label>
				<input
					id="password"
					type="password"
					bind:value={password}
					placeholder={$t.auth.pwd_placeholder}
					required
					minlength="8"
					autocomplete="new-password"
				/>
			</div>

			<div class="field">
				<label for="confirm">{$t.auth.confirm_password}</label>
				<input
					id="confirm"
					type="password"
					bind:value={confirm}
					placeholder="••••••••"
					required
					autocomplete="new-password"
				/>
			</div>

			<button class="btn btn-primary" type="submit" disabled={loading}>
				{loading ? $t.auth.register_loading : $t.auth.register_btn}
			</button>
		</form>

		<div class="divider">{$t.auth.or}</div>

		<a href="{API_URL}/api/auth/google" class="btn btn-google">
			<svg width="18" height="18" viewBox="0 0 48 48">
				<path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/>
				<path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/>
				<path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"/>
				<path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.18 1.48-4.97 2.31-8.16 2.31-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/>
			</svg>
			{$t.auth.google}
		</a>

		<p class="form-footer">
			{$t.auth.have_account} <a href="/login">{$t.auth.sign_in}</a>
		</p>
	</div>
{/if}

<style>
	.auth-checking {
		display: flex;
		justify-content: center;
		align-items: center;
		padding: 8rem 0;
	}

	.auth-spinner {
		width: 36px;
		height: 36px;
		border: 3px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}

	@keyframes spin { to { transform: rotate(360deg); } }

	/* ── Email sent state ── */
	.email-sent-card {
		text-align: center;
	}

	.email-icon {
		font-size: 3rem;
		margin-bottom: 0.75rem;
	}

	.sent-desc {
		color: var(--text-secondary);
		font-size: 0.95rem;
		margin: 0.5rem 0;
		line-height: 1.5;
	}

	.sent-email {
		font-weight: 600;
		color: var(--accent);
		margin: 0.25rem 0 0.75rem;
		font-size: 0.95rem;
		word-break: break-all;
	}

	.sent-spam {
		color: var(--text-secondary);
		font-size: 0.82rem;
		margin-bottom: 1.25rem;
	}

	.resend-row {
		display: flex;
		justify-content: center;
		margin-bottom: 1rem;
	}

	.resent-ok {
		color: var(--accent);
		font-size: 0.9rem;
		font-weight: 600;
	}

	.btn-secondary {
		background: transparent;
		border: 1px solid var(--border);
		color: var(--text-secondary);
		padding: 0.55rem 1.2rem;
		border-radius: 8px;
		cursor: pointer;
		font-size: 0.88rem;
		transition: border-color 0.15s, color 0.15s;
	}

	.btn-secondary:hover:not(:disabled) {
		border-color: var(--accent);
		color: var(--text);
	}

	.btn-secondary:disabled {
		opacity: 0.5;
		cursor: default;
	}
</style>
