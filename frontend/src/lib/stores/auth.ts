import { writable } from 'svelte/store';
import { API_URL as API } from '$lib/config';

export interface User {
	id: string;
	username: string;
	email: string;
	elo_rapid: number;
	elo_blitz: number;
	elo_bullet: number;
	is_admin: boolean;
}

export const user = writable<User | null>(null);
export const authLoading = writable<boolean>(true);


export async function loadUser() {
	try {
		const res = await fetch(`${API}/api/auth/me`, { credentials: 'include' });
		if (res.ok) {
			const json = await res.json();
			user.set(json.data);
		} else {
			user.set(null);
		}
	} catch {
		user.set(null);
	} finally {
		authLoading.set(false);
	}
}

export async function login(email: string, password: string) {
	const res = await fetch(`${API}/api/auth/login`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email, password })
	});
	const json = await res.json();
	// Rilancia con il codice errore — permette detection specifica nel UI
	if (!json.success) throw new Error(json.error?.code ?? json.error?.message ?? 'LOGIN_ERROR');
	await loadUser();
}

export async function register(username: string, email: string, password: string) {
	const res = await fetch(`${API}/api/auth/register`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ username, email, password })
	});
	const json = await res.json();
	if (!json.success) throw new Error(json.error?.message ?? 'Errore registrazione');
	// Restituisce {status:'email_sent', email:'...'} — NON carica l'utente
	// perché l'account non è ancora verificato
	return json.data as { status: string; email: string };
}

export async function resendVerification(email: string) {
	const res = await fetch(`${API}/api/auth/resend-verification`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email })
	});
	const json = await res.json();
	if (!json.success) throw new Error(json.error?.message ?? 'Errore');
	return json.data;
}

export async function verifyEmail(token: string) {
	const res = await fetch(`${API}/api/auth/verify-email`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ token })
	});
	const json = await res.json();
	if (!json.success) throw new Error(json.error?.message ?? 'Token non valido');
	// Il backend ha impostato il cookie JWT — carica l'utente
	await loadUser();
	return json.data;
}

export async function logout() {
	await fetch(`${API}/api/auth/logout`, { method: 'POST', credentials: 'include' });
	user.set(null);
}

/** Solo DEV_MODE=true — login con il solo username, senza password */
export async function devLogin(username: string) {
	const res = await fetch(`${API}/api/auth/dev-login`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ username })
	});

	let json: any;
	try {
		json = await res.json();
	} catch {
		// Il backend non è stato riavviato oppure non supporta DEV_MODE
		throw new Error(`Endpoint non trovato (HTTP ${res.status}) — riavvia il backend con "make backend"`);
	}

	if (!json.success) throw new Error(json.error?.message ?? 'Utente non trovato');
	await loadUser();
}
