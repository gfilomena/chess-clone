.PHONY: dev db db-stop backend frontend install build build-docker

# Avvia PostgreSQL + Redis
db:
	docker compose up -d
	@echo "Aspetto che i DB siano pronti..."
	@sleep 3
	@echo "DB pronti!"

# Ferma i DB
db-stop:
	docker compose down

# Avvia il backend Go (.env caricato automaticamente in main.go)
backend:
	cd backend && go run ./cmd/server/main.go

# Installa dipendenze Go
install-backend:
	cd backend && go mod tidy

# Installa dipendenze frontend + copia Stockfish in static
install-frontend:
	cd frontend && npm install
	cp frontend/node_modules/stockfish/bin/stockfish-18-lite-single.js frontend/static/stockfish.js
	cp frontend/node_modules/stockfish/bin/stockfish-18-lite-single.wasm frontend/static/stockfish.wasm
	@echo "Stockfish copiato in static/"

# Installa tutto
install: install-backend install-frontend

# Avvia il frontend SvelteKit sulla porta 5174
frontend:
	cd frontend && npm run dev -- --port 5174

# Tutto insieme (richiede tmux o terminali separati)
dev:
	@echo "Avvia in terminali separati:"
	@echo "  make db"
	@echo "  make backend"
	@echo "  make frontend"

# ── Produzione ──────────────────────────────────────────────────────────────

# Build locale: frontend SPA → embedded nel binario Go
# Output: ./chess-clone (binario singolo pronto per il deploy)
build:
	@echo "==> Build frontend SPA..."
	cd frontend && npm run build
	@echo "==> Copia build in backend/cmd/server/static..."
	rm -rf backend/cmd/server/static
	mkdir -p backend/cmd/server/static
	cp -r frontend/build/. backend/cmd/server/static/
	touch backend/cmd/server/static/.gitkeep
	@echo "==> Build binario Go (con frontend embedded)..."
	cd backend && CGO_ENABLED=0 go build -ldflags="-s -w" -o ../chess-clone ./cmd/server
	@echo "==> Fatto! Binario: ./chess-clone"
	@echo "    Avvia con: DATABASE_URL=... REDIS_URL=... ./chess-clone"

# Build via Docker (simula esattamente il deploy Railway)
build-docker:
	docker build -t chess-clone:local .
	@echo "==> Immagine pronta: chess-clone:local"
	@echo "    Avvia con: docker run -p 8080:8080 --env-file backend/.env chess-clone:local"
