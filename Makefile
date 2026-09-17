.PHONY: dev-backend dev-frontend build docker-up docker-down tunnel

# 建立到受管节点的 SSH 隧道(本机 5555 → 服务器 5555),WebUI 经此访问 dataplaneapi
tunnel:
	./deploy/tunnel.sh

# 本地开发:先起后端,再起前端(vite 会把 /api 代理到 8080)
dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && bun run dev

build:
	cd backend && go build -o haproxy-webui ./cmd/server
	cd frontend && bun run build

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down
