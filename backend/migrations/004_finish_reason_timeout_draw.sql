-- Aggiunge il nuovo motivo di fine partita per timeout con materiale insufficiente
ALTER TYPE finish_reason ADD VALUE IF NOT EXISTS 'timeout_vs_insufficient_material';
