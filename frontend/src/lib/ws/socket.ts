import { gameState } from '$lib/stores/game';
import { WS_URL as WS_BASE } from '$lib/config';

const WS_URL = `${WS_BASE}/ws/game`;

let socket: WebSocket | null = null;
let gameId: string = '';

export function connectToGame(id: string) {
	gameId = id;
	socket = new WebSocket(`${WS_URL}/${id}`);

	socket.onopen = () => {
		console.log('WebSocket connesso alla partita:', id);
	};

	socket.onmessage = (event) => {
		const msg = JSON.parse(event.data);
		handleMessage(msg);
	};

	socket.onclose = () => {
		console.log('WebSocket disconnesso');
	};

	socket.onerror = (err) => {
		console.error('WebSocket error:', err);
	};
}

function handleMessage(msg: { type: string; payload?: any }) {
	switch (msg.type) {
		case 'game_start':
			// Entrambi i giocatori connessi → la partita inizia
			gameState.update((s) => ({
				...s,
				status: 'active',
				fen: msg.payload.fen,
				playerColor: msg.payload.your_color,
				turn: 'w',                    // il bianco muove sempre per primo
				whiteMs: msg.payload.white_ms,
				blackMs: msg.payload.black_ms
			}));
			break;

		case 'move_made':
			gameState.update((s) => ({
				...s,
				fen: msg.payload.fen,
				pgn: msg.payload.pgn ?? s.pgn,
				turn: msg.payload.turn,
				whiteMs: msg.payload.white_ms,
				blackMs: msg.payload.black_ms
			}));
			break;

		case 'game_over':
			gameState.update((s) => ({
				...s,
				status: 'finished',
				result: msg.payload.result,
				finishReason: msg.payload.reason,
				pgn: msg.payload.pgn
			}));
			break;

		case 'draw_offered':
			gameState.update((s) => ({ ...s, drawOffered: true }));
			break;

		case 'opponent_disconnected':
			console.log(`Avversario disconnesso. Ha ${msg.payload.timeout_seconds}s per riconnettersi`);
			break;

		case 'opponent_reconnected':
			console.log('Avversario riconnesso');
			break;

		case 'timeout':
			gameState.update((s) => ({
				...s,
				status: 'finished',
				result: msg.payload.loser === 'white' ? 'black' : 'white',
				finishReason: 'timeout'
			}));
			break;
	}
}

export function sendMove(from: string, to: string, promotion?: string) {
	send({ type: 'move', payload: { from, to, promotion: promotion ?? null } });
}

export function sendResign() {
	send({ type: 'resign' });
}

export function sendOfferDraw() {
	send({ type: 'offer_draw' });
}

export function sendDrawResponse(accepted: boolean) {
	send({ type: 'draw_response', payload: { accepted } });
}

function send(msg: object) {
	if (socket?.readyState === WebSocket.OPEN) {
		socket.send(JSON.stringify(msg));
	}
}

export function disconnect() {
	socket?.close();
	socket = null;
}
