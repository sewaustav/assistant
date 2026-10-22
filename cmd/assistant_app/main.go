package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/sewaustav/assistant/internal/cli"
	"github.com/sewaustav/assistant/internal/config"
	"github.com/sewaustav/assistant/internal/dispatcher"
	"github.com/sewaustav/assistant/internal/lifecycle"
	"github.com/sewaustav/assistant/internal/local"
	"github.com/sewaustav/assistant/internal/logging"
	"github.com/sewaustav/assistant/internal/speech_recognition"
	"github.com/sewaustav/assistant/internal/tools"
)

func main() {
	// 0. Логи — в файл, ДО всего остального.
	logPath, err := logging.Init()
	if err != nil {
		// Если лог не открылся — падаем в stderr, но не молчим.
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

	cmds, err := config.LoadCommands()
	if err != nil {
		log.Fatalf("Commands: %v", err)
	}
	
	builtinTools := tools.BuildFromConfig(cmds)
	
	var localTools []local.Tool
	for _, t := range builtinTools {
		if lt, ok := t.(local.Tool); ok {
			localTools = append(localTools, lt)
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

	cliClient := cli.New(cfg, textInputChan, cmdChan)
	go cliClient.Run()

	mgr := lifecycle.NewManager()

	gigasttPath := filepath.Join(os.Getenv("HOME"), ".cargo/bin/gigastt")
	mgr.Register("gigastt", lifecycle.NewGigaSTT(gigasttPath))

	projectRoot, _ := os.Getwd()
	ttsDir := filepath.Join(projectRoot, "tts-server")
	pythonPath := filepath.Join(ttsDir, ".venv/bin/python")
	mgr.Register("silero", lifecycle.NewSilero(pythonPath, ttsDir))

	var recognizer *speech_recognition.Recognizer

	if cfg.Mode == config.ModeVoice {
		cli.PrintInfo("Запускаю сервера...")

		if err := mgr.Start(ctx, "gigastt"); err != nil {
			log.Fatalf("[main] gigastt: %v", err)
		}
		if err := mgr.Start(ctx, "silero"); err != nil {
			log.Fatalf("[main] silero: %v", err)
		}

		recognizer, err = speech_recognition.NewRecognizer()
		if err != nil {
			log.Fatalf("[main] Recognizer: %v", err)
		}
		if err := recognizer.Start(ctx); err != nil {
			log.Fatalf("[main] Recognizer start: %v", err)
		}

		go func() {
			for raw := range recognizer.RawChan() {
				select {
				case voiceChan <- raw:
				case <-ctx.Done():
					return
				}
			}
		}()

		cli.PrintInfo("Всё загружено. Слушаю...")
	} else {
		cli.PrintInfo("Текстовый режим. Введите сообщение.")
	}

	disp := dispatcher.New(
		cfg,
		voiceChan,
		textInputChan,
		cmdChan,
		responseChan,
		recognizer,
		mgr,
		cliClient,
		localHandler,
		cancel,
	)
	go disp.Run(ctx)

	go func() {
		for resp := range responseChan {
			cli.PrintResponse(resp)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		cli.PrintInfo("Получен сигнал завершения.")
	case <-ctx.Done():
		cli.PrintInfo("Контекст отменён.")
	}

	if recognizer != nil {
		recognizer.Stop()
	}
	disp.Close()
	mgr.StopAll()
	cli.PrintInfo("Завершено.")
}