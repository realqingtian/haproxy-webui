.PHONY: dev-backend dev-frontend build test test-integration check e2e docker-up docker-down tunnel

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

# 提交前的统一检查入口(见 AGENTS.md / docs/git-conventions.md)
test:
	cd backend && go test ./...

# 容器级集成测试:起 local-e2e 容器(真实 HAProxy + dataplaneapi v3)后跑真实链路用例;
# 容器不可达时该用例自动跳过,所以 `make test` 无 Docker 也全绿
test-integration:
	cd deploy/dataplaneapi/local-e2e && docker compose up -d --wait
	cd backend && HAPROXY_WEBUI_IT_DPAPI=http://localhost:5555 go test ./internal/api/ -run TestContainerRealDataplaneFlow -v

check: test
	cd backend && go vet ./...
	cd frontend && bun run build

# Playwright E2E 冒烟(核心链路;自动编排 local-e2e 容器 + 独立 DB 后端 + vite dev)。
# 需要 8080/5173 端口空闲、Docker 运行、Playwright 浏览器已安装(cd frontend && bunx playwright install)
e2e:
	cd frontend && bunx playwright test

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down
