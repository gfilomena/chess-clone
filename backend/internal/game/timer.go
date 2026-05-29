package game

import "time"

// timerState vive in memoria nella Room — nessun Redis necessario.
// Tutti gli accessi avvengono nella goroutine Run() della Room (via canale inbound),
// quindi non serve mutex.
type timerState struct {
	whiteMs     int64
	blackMs     int64
	turnStarted time.Time
}

func newTimerState(timeControlSec int) timerState {
	ms := int64(timeControlSec) * 1000
	return timerState{
		whiteMs:     ms,
		blackMs:     ms,
		turnStarted: time.Now(),
	}
}

// recordMove sottrae il tempo trascorso al giocatore che ha appena mosso
// e restituisce i tempi aggiornati + eventuale timeout.
func (t *timerState) recordMove(moverColor string) (whiteMs, blackMs int64, timedOut bool, loser string) {
	elapsed := time.Since(t.turnStarted).Milliseconds()

	if moverColor == "white" {
		t.whiteMs -= elapsed
		if t.whiteMs <= 0 {
			t.whiteMs = 0
			return t.whiteMs, t.blackMs, true, "white"
		}
	} else {
		t.blackMs -= elapsed
		if t.blackMs <= 0 {
			t.blackMs = 0
			return t.whiteMs, t.blackMs, true, "black"
		}
	}

	t.turnStarted = time.Now()
	return t.whiteMs, t.blackMs, false, ""
}

// checkTimeout verifica se il giocatore attivo ha esaurito il tempo.
// Usa una grazia di 800 ms per compensare la latenza di rete tra client e server.
func (t *timerState) checkTimeout(activeTurn string) (timedOut bool, loser string) {
	elapsed := time.Since(t.turnStarted).Milliseconds()
	const graceMs = 800
	if activeTurn == "white" {
		if t.whiteMs-elapsed <= graceMs {
			return true, "white"
		}
	} else {
		if t.blackMs-elapsed <= graceMs {
			return true, "black"
		}
	}
	return false, ""
}

// currentTimes restituisce i tempi con l'elapsed del turno in corso già sottratto.
func (t *timerState) currentTimes(activeTurn string) (whiteMs, blackMs int64) {
	elapsed := time.Since(t.turnStarted).Milliseconds()
	wMs, bMs := t.whiteMs, t.blackMs
	if activeTurn == "white" {
		wMs -= elapsed
		if wMs < 0 {
			wMs = 0
		}
	} else {
		bMs -= elapsed
		if bMs < 0 {
			bMs = 0
		}
	}
	return wMs, bMs
}
