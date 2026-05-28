package game

import "encoding/json"

// ── Messaggi in entrata (client → server) ──────────────────────────────────

type InboundMsg struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type MovePayload struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Promotion string `json:"promotion"`
}

type DrawResponsePayload struct {
	Accepted bool `json:"accepted"`
}

// ── Messaggi in uscita (server → client) ──────────────────────────────────

type OutboundMsg struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type MoveMadePayload struct {
	From    string `json:"from"`
	To      string `json:"to"`
	FEN     string `json:"fen"`
	PGN     string `json:"pgn"`
	Turn    string `json:"turn"`     // "w" | "b"
	WhiteMs int64  `json:"white_ms"` // ms rimanenti bianco
	BlackMs int64  `json:"black_ms"` // ms rimanenti nero
}

type GameOverPayload struct {
	Result string `json:"result"` // "white" | "black" | "draw"
	Reason string `json:"reason"` // "checkmate" | "timeout" | ...
	PGN    string `json:"pgn"`
}

type TimeoutPayload struct {
	Loser string `json:"loser"` // "white" | "black"
}

type DisconnectPayload struct {
	TimeoutSeconds int `json:"timeout_seconds"`
}

type ErrorPayload struct {
	Reason string `json:"reason"`
}
