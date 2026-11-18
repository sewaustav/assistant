# --- Переменные ---
BINARY_NAME   := assistant
MAIN_PATH     := ./cmd/assistant_app
BUILD_DIR     := ./bin
GO            := go
CARGO_BIN     := $(HOME)/.cargo/bin
GIGASTT_BIN   := $(CARGO_BIN)/gigastt
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS       := -X main.version=$(VERSION)

# --- Silero TTS ---
TTS_DIR       := ./tts-server
TTS_VENV      := $(TTS_DIR)/.venv
TTS_REQS      := $(TTS_DIR)/requirements.txt

# --- ОС ---
HOST_OS       := $(shell $(GO) env GOOS)
ifeq ($(OS),Windows_NT)
  HOST_OS     := windows
endif

ifeq ($(HOST_OS),windows)
  BIN_EXT     := .exe
  GIGASTT_BIN := $(CARGO_BIN)/gigastt.exe
  TTS_PYTHON  := $(TTS_VENV)/Scripts/python.exe
  TTS_PIP     := $(TTS_VENV)/Scripts/pip.exe
else
  BIN_EXT     :=
  TTS_PYTHON  := $(TTS_VENV)/bin/python
  TTS_PIP     := $(TTS_VENV)/bin/pip
endif

# --- Цели ---

.PHONY: all
all: install build
	@echo ">>> Всё готово. Запускай: make run"

# --- Установка ---

.PHONY: install
install: install-go install-tts
	@echo ">>> Установка завершена."

.PHONY: install-go
install-go:
	@echo ">>> Проверка Go..."
	@which $(GO) > /dev/null || (echo "Go не установлен. Установи: https://go.dev/dl/" && exit 1)
	@echo ">>> Проверка cargo..."
	@which cargo > /dev/null || (echo "Rust/cargo не установлен. Установи: https://rustup.rs/" && exit 1)
ifeq ($(HOST_OS),windows)
	@echo ">>> Windows: добавь $(CARGO_BIN) в PATH вручную, если gigastt не находится."
else
	@echo ">>> Проверка PATH для ~/.cargo/bin..."
	@if ! echo "$$PATH" | grep -q "$(CARGO_BIN)"; then \
		echo ">>> Добавляю $(CARGO_BIN) в PATH..."; \
		if ! grep -q 'cargo/bin' $(HOME)/.zshrc 2>/dev/null; then \
			echo 'export PATH="$$HOME/.cargo/bin:$$PATH"' >> $(HOME)/.zshrc; \
			echo ">>> Добавлено в ~/.zshrc. Перезапусти терминал или выполни: source ~/.zshrc"; \
		fi; \
	fi
endif
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

.PHONY: install-tts
install-tts:
	@echo ">>> Проверка Python..."
	@which python3 > /dev/null || which python > /dev/null || (echo "Python3 не установлен" && exit 1)
	@echo ">>> Создание venv для Silero TTS (если нет)..."
	@if [ ! -d "$(TTS_VENV)" ]; then \
		if which python3 > /dev/null; then python3 -m venv $(TTS_VENV); else python -m venv $(TTS_VENV); fi; \
		echo ">>> venv создан: $(TTS_VENV)"; \
	else \
		echo ">>> venv уже существует: $(TTS_VENV)"; \
	fi
	@echo ">>> Установка Python-зависимостей..."
	@$(TTS_PIP) install --upgrade pip -q
	@$(TTS_PIP) install -r $(TTS_REQS) -q
	@echo ">>> Silero TTS готов."

# --- Сборка ---

.PHONY: build
build:
	@echo ">>> Сборка $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT) $(MAIN_PATH)
	@echo ">>> Готово: $(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT)"

.PHONY: build-linux
build-linux:
	@echo ">>> Сборка под Linux..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GO) build -trimpath -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)

.PHONY: build-windows
build-windows:
	@echo ">>> Сборка под Windows (без CGO)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)

.PHONY: build-darwin
build-darwin:
	@echo ">>> Сборка под macOS (нужен macOS или osxcross)..."
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GO) build -trimpath -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)

.PHONY: build-all
build-all: build-linux build-windows
	@echo ">>> Готово. macOS собирай на macOS: make build-darwin"

# --- Запуск ---

# Запуск собранного бинарника. Сервера (gigastt, silero) управляются из Go.
.PHONY: run
run: build
	@echo ">>> Запуск $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)$(BIN_EXT)

# Быстрый запуск через go run (без сборки).
.PHONY: dev
dev:
	@echo ">>> Запуск через go run..."
	$(GO) run $(MAIN_PATH)

# --- Разработка ---

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

# --- Очистка ---

.PHONY: clean
clean:
	@echo ">>> Очистка..."
	@rm -rf $(BUILD_DIR)

.PHONY: clean-tts
clean-tts:
	@echo ">>> Удаление venv Silero TTS..."
	@rm -rf $(TTS_VENV)

# --- Справка ---

.PHONY: help
help:
	@echo "Доступные команды:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
