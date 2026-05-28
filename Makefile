.PHONY: dev db db-stop backend frontend install

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
