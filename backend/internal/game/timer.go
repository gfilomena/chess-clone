package game

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chess-clone/backend/internal/db"
)

type timerState struct {
	WhiteMs      int64  `json:"white_ms"`
	BlackMs      int64  `json:"black_ms"`
	Turn         string `json:"turn"`          // "w" | "b"
	TurnStarted  int64  `json:"turn_started"`  // unix ms
}

func timerKey(gameID string) string {
	return fmt.Sprintf("game:timer:%s", gameID)
}

// InitTimer inizializza il timer in Redis all'inizio della partita
func InitTimer(ctx context.Context, rdb *db.Redis, gameID string, timeControlSec int) error {
	ms := int64(timeControlSec) * 1000
	state := timerState{
		WhiteMs:     ms,
		BlackMs:     ms,
		Turn:        "w",
		TurnStarted: time.Now().UnixMilli(),
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return rdb.Client.Set(ctx, timerKey(gameID), raw, 2*time.Hour).Err()
}

// RecordMove calcola il tempo usato e aggiorna Redis.
// Restituisce i nuovi tempi e se c'è stato un timeout.
func RecordMove(ctx context.Context, rdb *db.Redis, gameID string) (whiteMs, blackMs int64, timedOut bool, loser string, err error) {
	raw, err := rdb.Client.Get(ctx, timerKey(gameID)).Bytes()
	if err != nil {
		return 0, 0, false, "", fmt.Errorf("timer non trovato: %w", err)
	}

	var state timerState
	if err = json.Unmarshal(raw, &state); err != nil {
		return 0, 0, false, "", err
	}

	elapsed := time.Now().UnixMilli() - state.TurnStarted

	// Sottrai il tempo dal giocatore corrente
	if state.Turn == "w" {
		state.WhiteMs -= elapsed
		if state.WhiteMs <= 0 {
			state.WhiteMs = 0
			return state.WhiteMs, state.BlackMs, true, "white", nil
		}
	} else {
		state.BlackMs -= elapsed
		if state.BlackMs <= 0 {
			state.BlackMs = 0
			return state.WhiteMs, state.BlackMs, true, "black", nil
		}
	}

	// Passa il turno
	if state.Turn == "w" {
		state.Turn = "b"
	} else {
		state.Turn = "w"
	}
	state.TurnStarted = time.Now().UnixMilli()

	// Salva in Redis
	updated, _ := json.Marshal(state)
	rdb.Client.Set(ctx, timerKey(gameID), updated, 2*time.Hour)

	return state.WhiteMs, state.BlackMs, false, "", nil
}

// GetTimer restituisce i tempi correnti (con elapsed calcolato)
func GetTimer(ctx context.Context, rdb *db.Redis, gameID string) (whiteMs, blackMs int64) {
	raw, err := rdb.Client.Get(ctx, timerKey(gameID)).Bytes()
	if err != nil {
		return 0, 0
	}
	var state timerState
	if err = json.Unmarshal(raw, &state); err != nil {
		return 0, 0
	}

	elapsed := time.Now().UnixMilli() - state.TurnStarted
	wMs, bMs := state.WhiteMs, state.BlackMs
	if state.Turn == "w" {
		wMs = max(0, wMs-elapsed)
	} else {
		bMs = max(0, bMs-elapsed)
	}
	return wMs, bMs
}

// DeleteTimer rimuove il timer da Redis
func DeleteTimer(ctx context.Context, rdb *db.Redis, gameID string) {
	rdb.Client.Del(ctx, timerKey(gameID))
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
