// In sviluppo: legge da .env (http://localhost:8080)
// In produzione (stesso-origin): stringa vuota → fetch relativo, WS derivato da window.location

export const API_URL = import.meta.env.VITE_API_URL ?? '';

function resolveWsUrl(): string {
	if (import.meta.env.VITE_WS_URL) return import.meta.env.VITE_WS_URL;
	// Produzione: deriva il protocollo dal browser (ws:// o wss://)
	if (typeof window !== 'undefined') {
		const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
		return `${proto}://${window.location.host}`;
	}
	return 'ws://localhost:8080';
}

export const WS_URL   = resolveWsUrl();
export const DEV_MODE = import.meta.env.VITE_DEV_MODE === 'true';
