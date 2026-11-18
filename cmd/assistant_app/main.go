package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/sewaustav/assistant/internal/apps"
	"github.com/sewaustav/assistant/internal/cli"
	"github.com/sewaustav/assistant/internal/config"
	"github.com/sewaustav/assistant/internal/dispatcher"
	"github.com/sewaustav/assistant/internal/lifecycle"
	"github.com/sewaustav/assistant/internal/llm"
	"github.com/sewaustav/assistant/internal/local"
	"github.com/sewaustav/assistant/internal/logging"
	"github.com/sewaustav/assistant/internal/paths"
	"github.com/sewaustav/assistant/internal/tools"
)

// version подставляется при сборке:
// go build -ldflags "-X main.version=1.0.0"
var version = "dev"

func main() {
	cli.Version = version

	// 0. Логи — в файл, ДО всего остального.
	logPath, err := logging.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось инициализировать логи: %v\n", err)
	} else {
		log.Printf("[main] Логи: %s", logPath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config: %v", err)
	}

	cfgPath, _ := config.Path()
	log.Printf("[main] Конфиг: %s", cfgPath)
	log.Printf("[main] Режим: %s, TTS speaker: %s, ASR enabled: %v",
		cfg.Mode, cfg.TTS.Speaker, cfg.ASR.Enabled)

	// Приложения: системные + встроенные + пользовательские из config.json.
	appResolver := apps.New(customApps(cfg.Apps))

	cmds, err := config.LoadCommands()
	if err != nil {
		log.Fatalf("Commands: %v", err)
	}

	builtinTools := tools.BuildFromConfig(cmds, tools.Options{
		Apps:       appResolver,
		SearchURL:  cfg.SearchURL,
		BrowserURL: "https://www.google.com",
	})

	var localTools []local.Tool
	var llmTools []llm.Tool
	for _, t := range builtinTools {
		if lt, ok := t.(local.Tool); ok {
			if enabled, ok := t.(interface{ LocalEnabled() bool }); !ok || enabled.LocalEnabled() {
				localTools = append(localTools, lt)
			}
		}
		if lt, ok := t.(llm.Tool); ok {
			if enabled, ok := t.(interface{ LLMEnabled() bool }); !ok || enabled.LLMEnabled() {
				llmTools = append(llmTools, lt)
			}
		}
	}

	localHandler, err := local.New(localTools)
	if err != nil {
		log.Fatalf("Local handler: %v", err)
	}

	voiceChan := make(chan string, 10)
	textInputChan := make(chan string, 10)
	cmdChan := make(chan string, 10)
	responseChan := make(chan string, 10)

	cliClient := cli.New(cfg, textInputChan, cmdChan, appResolver)
	go cliClient.Run()

	mgr := lifecycle.NewManager()

	gigasttPath := paths.FindGigaSTT(cfg.Paths.GigaSTT)
	if gigasttPath != "" {
		mgr.Register("gigastt", lifecycle.NewGigaSTT(gigasttPath, asrArgs(cfg)))
		log.Printf("[main] gigastt: %s", gigasttPath)
	} else {
		log.Printf("[main] gigastt не найден")
	}

	ttsDir := paths.FindTTSDir(cfg.Paths.TTSDir)
	pythonPath := ""
	if ttsDir != "" {
		pythonPath = paths.FindPython(ttsDir, cfg.Paths.Python)
	}
	if pythonPath != "" {
		mgr.Register("silero", lifecycle.NewSilero(pythonPath, ttsDir))
		log.Printf("[main] silero: %s (%s)", pythonPath, ttsDir)
	} else {
		log.Printf("[main] Python для Silero не найден (tts_dir=%s)", ttsDir)
	}

	llmHandler := llm.New(cfg, llmTools)

	disp := dispatcher.New(
		cfg,
		voiceChan,
		textInputChan,
		cmdChan,
		responseChan,
		nil,
		mgr,
		cliClient,
		localHandler,
		llmHandler,
		cancel,
	)
	go disp.Run(ctx)

	go func() {
		for resp := range responseChan {
			cli.PrintResponse(resp)
		}
	}()

	// Голосовой режим: если что-то не готово — не падаем, а честно
	// переключаемся в текстовый режим.
	if cfg.Mode == config.ModeVoice {
		if !cfg.ASR.Enabled {
			cli.PrintError(fmt.Errorf("распознавание речи выключено (asr.enabled=false)"))
			disp.SwitchMode(ctx, config.ModeText)
		} else if gigasttPath == "" {
			cli.PrintError(fmt.Errorf("gigastt не найден"))
			cli.PrintInfo("Установи его командой: cargo install gigastt")
			disp.SwitchMode(ctx, config.ModeText)
		} else {
			cli.PrintInfo("Запускаю сервера...")
			if err := disp.StartVoice(ctx); err != nil {
				if ctx.Err() != nil {
					// Пользователь нажал /quit или Ctrl+C во время старта.
					disp.Close()
					mgr.StopAll()
					return
				}
				cli.PrintError(err)
				cli.PrintInfo("Переключаюсь в текстовый режим. Голосовые команды будут доступны после исправления.")
				disp.SwitchMode(ctx, config.ModeText)
			} else {
				cli.PrintInfo("Всё загружено. Слушаю...")
			}
		}
	} else {
		cli.PrintInfo("Текстовый режим. Введите сообщение или /help.")
	}

	if logPath != "" {
		cli.PrintInfo("Лог: %s", logPath)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		cli.PrintInfo("Получен сигнал завершения.")
	case <-ctx.Done():
		cli.PrintInfo("Контекст отменён.")
	}

	disp.Close()
	mgr.StopAll()
	cli.PrintInfo("Завершено.")
}

func customApps(in map[string]config.AppConfig) map[string]apps.CustomApp {
	out := make(map[string]apps.CustomApp, len(in))
	for key, cfg := range in {
		out[key] = apps.CustomApp{
			Name:    cfg.Name,
			Aliases: cfg.Aliases,
			Linux:   cfg.Linux,
			Windows: cfg.Windows,
			Darwin:  cfg.Darwin,
		}
	}
	return out
}

// asrArgs собирает аргументы для `gigastt serve`.
func asrArgs(cfg *config.Config) []string {
	var args []string
	if u, err := url.Parse(cfg.ASR.ServerURL); err == nil && u.Port() != "" && u.Port() != "9876" {
		args = append(args, "--port", u.Port())
	}
	if cfg.ASR.ModelDir != "" {
		args = append(args, "--model-dir", cfg.ASR.ModelDir)
	}
	if cfg.ASR.VAD {
		args = append(args, "--vad", "--endpoint-mode", "assistant")
	}
	args = append(args, cfg.ASR.ExtraArgs...)
	return args
}
