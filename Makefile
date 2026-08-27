.PHONY: build dev-server dev-web install-desktop dev-desktop build-desktop check-desktop clean

CARGO_BIN ?= $(shell command -v cargo 2>/dev/null || rustup which cargo 2>/dev/null)
RUST_BIN_DIR := $(dir $(CARGO_BIN))
TAURI_TARGET_DIR := $(CURDIR)/desktop/src-tauri/target

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

# 安装桌面端与共享 API 包依赖（首次开发执行）
install-desktop:
	cd packages/shared && npm install
	cd desktop && npm install

# macOS 桌面客户端：Tauri 托盘应用（需 Rust stable）
dev-desktop:
	cd desktop && CARGO_TARGET_DIR="$(TAURI_TARGET_DIR)" PATH="$(RUST_BIN_DIR):$$PATH" npm run tauri dev

build-desktop:
	cd desktop && CARGO_TARGET_DIR="$(TAURI_TARGET_DIR)" PATH="$(RUST_BIN_DIR):$$PATH" npm run tauri build

check-desktop:
	cd packages/shared && npm run typecheck
	cd web && npm run typecheck
	cd desktop && npm run test
	cd desktop && npm run build
	cd desktop/src-tauri && PATH="$(RUST_BIN_DIR):$$PATH" "$(CARGO_BIN)" fmt --check
	cd desktop/src-tauri && PATH="$(RUST_BIN_DIR):$$PATH" "$(CARGO_BIN)" clippy --all-targets -- -D warnings
	cd desktop/src-tauri && PATH="$(RUST_BIN_DIR):$$PATH" "$(CARGO_BIN)" test

clean:
	rm -rf server/bin web/dist desktop/dist desktop/src-tauri/target
