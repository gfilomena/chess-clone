import { writable } from 'svelte/store';
import { goto } from '$app/navigation';
import { API_URL as API } from '$lib/config';

// ─── Tipi ────────────────────────────────────────────────────────────────────

export interface OnlineUser {
	id: string;
	username: string;
	elo_rapid: number;
}

export interface InvitePayload {
	from_id: string;
	from_username: string;
	from_elo: number;
}

// ─── Store ───────────────────────────────────────────────────────────────────

export const onlineUsers = writable<OnlineUser[]>([]);

/** Invito in arrivo da gestire con il toast */
export const pendingInvite = writable<InvitePayload | null>(null);

// ─── Singleton SSE + heartbeat ───────────────────────────────────────────────

let inviteSSE: EventSource | null = null;
let heartbeatTimer: ReturnType<typeof setInterval> | null = null;
let sseReconnectTimer: ReturnType<typeof setTimeout> | null = null;

// ─── Heartbeat ───────────────────────────────────────────────────────────────

async function sendHeartbeat() {
	try {
		await fetch(`${API}/api/users/heartbeat`, {
			method: 'POST',
			credentials: 'include'
		});
	} catch {
		// silenzioso — la rete potrebbe essere temporaneamente non disponibile
	}
}

export function startHeartbeat() {
	if (heartbeatTimer) return;
	sendHeartbeat(); // primo battito immediato
	heartbeatTimer = setInterval(sendHeartbeat, 30_000);
}

export function stopHeartbeat() {
	if (heartbeatTimer) {
		clearInterval(heartbeatTimer);
		heartbeatTimer = null;
	}
}

// ─── SSE inviti ──────────────────────────────────────────────────────────────

export function startInviteSSE() {
	if (inviteSSE) return; // già aperto

	inviteSSE = new EventSource(`${API}/api/invitations/stream`, {
		withCredentials: true
	} as EventSourceInit);

	inviteSSE.addEventListener('invited', (e: MessageEvent) => {
		const payload: InvitePayload = JSON.parse(e.data);
		pendingInvite.set(payload);
	});

	inviteSSE.addEventListener('matched', (e: MessageEvent) => {
		const { game_id } = JSON.parse(e.data);
		pendingInvite.set(null);
		goto(`/game/${game_id}`);
	});

	inviteSSE.onerror = () => {
		inviteSSE?.close();
		inviteSSE = null;
		// Riconnessione automatica dopo 4 secondi
		if (sseReconnectTimer) clearTimeout(sseReconnectTimer);
		sseReconnectTimer = setTimeout(startInviteSSE, 4_000);
	};
}

export function stopInviteSSE() {
	if (sseReconnectTimer) {
		clearTimeout(sseReconnectTimer);
		sseReconnectTimer = null;
	}
	inviteSSE?.close();
	inviteSSE = null;
}

// ─── API online users ─────────────────────────────────────────────────────────

export async function fetchOnlineUsers(): Promise<void> {
	try {
		const res = await fetch(`${API}/api/users/online`, { credentials: 'include' });
		if (res.ok) {
			const json = await res.json();
			onlineUsers.set(json.data ?? []);
		}
	} catch {
		// silenzioso
	}
}

// ─── API inviti ───────────────────────────────────────────────────────────────

export async function sendInvite(toUserID: string): Promise<void> {
	const res = await fetch(`${API}/api/invitations`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ to_user_id: toUserID })
	});
	const json = await res.json();
	if (!json.success) throw new Error(json.error?.message ?? 'Errore invio invito');
}

export async function acceptInvite(fromID: string): Promise<string> {
	const res = await fetch(`${API}/api/invitations/${fromID}/accept`, {
		method: 'POST',
		credentials: 'include'
	});
	const json = await res.json();
	if (!json.success) throw new Error(json.error?.message ?? 'Invito scaduto o non trovato');
	return json.data.game_id as string;
}

export async function declineInvite(fromID: string): Promise<void> {
	pendingInvite.set(null);
	try {
		await fetch(`${API}/api/invitations/${fromID}`, {
			method: 'DELETE',
			credentials: 'include'
		});
	} catch {
		// silenzioso
	}
}
