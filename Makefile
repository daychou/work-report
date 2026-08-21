.PHONY: build dev-server dev-web clean

# 一键构建：前端 dist + Go 二进制 server/bin/server
# 前端不再内嵌进二进制：运行时由 server.static_dir（默认 ../web/dist）从磁盘托管，
# 也可把 web/dist 独立部署到 Nginx（反代 /api 到 Go 服务）
build:
	cd web && npm run build
	cd server && go build -o bin/server ./cmd/server
	@echo "==> built: server/bin/server（运行时确保 static_dir 指向 web/dist，或前端交给 Nginx）"

# 本地开发：后端（另开一个终端跑 make dev-web）
dev-server:
	cd server && go run ./cmd/server -conf config.yaml

# 本地开发：前端（Vite dev server，代理 /api 到后端）
dev-web:
	cd web && npm run dev

clean:
	rm -rf server/bin web/dist
