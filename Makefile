BINARY_NAME   := assistant
MAIN_PATH     := ./cmd/assistant
BUILD_DIR     := ./bin
GO            := go
CARGO_BIN     := $(HOME)/.cargo/bin
GIGASTT_BIN   := $(CARGO_BIN)/gigastt
GIGASTT_LOG   := /tmp/gigastt.log
GIGASTT_PID   := /tmp/gigastt.pid
APP_LOG       := /tmp/assistant.log
APP_PID       := /tmp/assistant.pid


.PHONY: all
all: install build
	@echo ">>> Всё готово. Запускай: make start"

.PHONY: install
install:
	@echo ">>> Проверка Go..."
	@which $(GO) > /dev/null || (echo "Go не установлен. Установи: https://go.dev/dl/" && exit 1)
	@echo ">>> Проверка cargo..."
	@which cargo > /dev/null || (echo "Rust/cargo не установлен. Установи: curl https://sh.rustup.rs -sSf | sh" && exit 1)
	@echo ">>> Проверка PATH для ~/.cargo/bin..."
	@if ! echo "$$PATH" | grep -q "$(CARGO_BIN)"; then \
		echo ">>> Добавляю $(CARGO_BIN) в PATH..."; \
		if ! grep -q 'cargo/bin' $(HOME)/.zshrc 2>/dev/null; then \
			echo 'export PATH="$$HOME/.cargo/bin:$$PATH"' >> $(HOME)/.zshrc; \
			echo ">>> Добавлено в ~/.zshrc. Перезапусти терминал или выполни: source ~/.zshrc"; \
		fi; \
	fi
	@echo ">>> Установка gigastt (если не установлен)..."
	@if [ ! -x "$(GIGASTT_BIN)" ]; then \
		echo ">>> Собираю gigastt через cargo (это займёт несколько минут)..."; \
		cargo install gigastt; \
	else \
		echo ">>> gigastt уже установлен: $(GIGASTT_BIN)"; \
	fi
	@echo ">>> Загрузка Go-зависимостей..."
	$(GO) mod tidy
	$(GO) mod download
	@echo ">>> Установка завершена."

.PHONY: build
build:
	@echo ">>> Сборка $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo ">>> Готово: $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: start
start: build stop-gigastt
	@echo ">>> Запуск gigastt serve..."
	@$(GIGASTT_BIN) serve > $(GIGASTT_LOG) 2>&1 & echo $$! > $(GIGASTT_PID)
	@echo ">>> gigastt PID: $$(cat $(GIGASTT_PID))"
	@echo ">>> Ожидание готовности gigastt..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s http://127.0.0.1:9876/health > /dev/null 2>&1; then \
			echo ">>> gigastt healthcheck OK."; \
			break; \
		fi; \
		echo ">>> ...ждём ($$i/10)"; \
		sleep 1; \
	done
	@echo ">>> Даю модели время загрузиться (3 сек)..."
	@sleep 3
	@echo ">>> Запуск приложения..."
	@$(BUILD_DIR)/$(BINARY_NAME)

.PHONY: start-gigastt
start-gigastt: stop-gigastt
	@echo ">>> Запуск gigastt serve..."
	@$(GIGASTT_BIN) serve > $(GIGASTT_LOG) 2>&1 & echo $$! > $(GIGASTT_PID)
	@echo ">>> gigastt PID: $$(cat $(GIGASTT_PID))"

.PHONY: stop-gigastt
stop-gigastt:
	@if [ -f $(GIGASTT_PID) ]; then \
		PID=$$(cat $(GIGASTT_PID)); \
		if kill -0 $$PID 2>/dev/null; then \
			kill $$PID; \
			echo ">>> gigastt (PID $$PID) остановлен."; \
		else \
			echo ">>> gigastt уже не работает."; \
		fi; \
		rm -f $(GIGASTT_PID); \
	fi

.PHONY: stop
stop: stop-gigastt stop-app
	@echo ">>> Всё остановлено."

.PHONY: stop-app
stop-app:
	@if [ -f $(APP_PID) ]; then \
		PID=$$(cat $(APP_PID)); \
		if kill -0 $$PID 2>/dev/null; then \
			kill $$PID; \
			echo ">>> Приложение (PID $$PID) остановлено."; \
		fi; \
		rm -f $(APP_PID); \
	fi

.PHONY: run
run:
	@echo ">>> Запуск приложения через go run..."
	$(GO) run $(MAIN_PATH)

.PHONY: test
test:
	@echo ">>> Тесты..."
	$(GO) test ./... -v

.PHONY: lint
lint:
	@echo ">>> go vet..."
	$(GO) vet ./...
	@echo ">>> gofmt..."
	@gofmt -l -w .

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	@echo ">>> Очистка..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(GIGASTT_LOG) $(GIGASTT_PID) $(APP_LOG) $(APP_PID)

.PHONY: help
help:
	@echo "Доступные команды:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'