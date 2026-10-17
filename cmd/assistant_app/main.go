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
	"github.com/sewaustav/assistant/internal/speech_recognition"
)

func main() {
	fmt.Println("=== Assistant ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Конфиг.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config: %v", err)
	}

	cfgPath, _ := config.Path()
	log.Printf("[main] Конфиг: %s", cfgPath)
	log.Printf("[main] Режим: %s, TTS speaker: %s, ASR enabled: %v",
		cfg.Mode, cfg.TTS.Speaker, cfg.ASR.Enabled)

	// 2. Каналы.
	voiceChan := make(chan string, 10)     // от GigaAM (сырой JSON)
	textInputChan := make(chan string, 10) // от CLI (текст)
	cmdChan := make(chan string, 10)       // команды управления (/mode, /quit)
	responseChan := make(chan string, 10)  // ответы агента

	// 3. CLI — запускаем сразу, он всегда доступен.
	cliClient := cli.New(cfg, textInputChan, cmdChan)
	go cliClient.Run()

	// 4. Lifecycle-менеджер.
	mgr := lifecycle.NewManager()

	gigasttPath := filepath.Join(os.Getenv("HOME"), ".cargo/bin/gigastt")
	mgr.Register("gigastt", lifecycle.NewGigaSTT(gigasttPath))

	projectRoot, _ := os.Getwd()
	ttsDir := filepath.Join(projectRoot, "tts-server")
	pythonPath := filepath.Join(ttsDir, ".venv/bin/python")
	mgr.Register("silero", lifecycle.NewSilero(pythonPath, ttsDir))

	// 5. Recognizer — создаём ТОЛЬКО если режим voice, и ТОЛЬКО после запуска gigastt.
	var recognizer *speech_recognition.Recognizer

	if cfg.Mode == config.ModeVoice {
		log.Println("[main] Стартовый режим: voice — поднимаю сервера...")

		if err := mgr.Start(ctx, "gigastt"); err != nil {
			log.Fatalf("[main] gigastt: %v", err)
		}
		if err := mgr.Start(ctx, "silero"); err != nil {
			log.Fatalf("[main] silero: %v", err)
		}

		// Теперь gigastt готов — можно создавать Recognizer.
		recognizer, err = speech_recognition.NewRecognizer()
		if err != nil {
			log.Fatalf("[main] Recognizer: %v", err)
		}

		if err := recognizer.Start(ctx); err != nil {
			log.Fatalf("[main] Recognizer start: %v", err)
		}

		// Пробрасываем сырой JSON из Recognizer в voiceChan.
		go func() {
			for raw := range recognizer.RawChan() {
				select {
				case voiceChan <- raw:
				case <-ctx.Done():
					return
				}
			}
		}()
	} else {
		log.Println("[main] Стартовый режим: text — сервера не нужны.")
	}

	// 6. Диспетчер.
	disp := dispatcher.New(
		cfg,
		voiceChan,
		textInputChan,
		cmdChan,
		responseChan,
		recognizer, // может быть nil в текстовом режиме
		mgr,
		cliClient,
		cancel,
	)
	go disp.Run(ctx)

	// 7. Читаем ответы и печатаем в CLI.
	go func() {
		for resp := range responseChan {
			cli.PrintResponse(resp)
		}
	}()

	// 8. Ждём сигнала завершения.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("\n[main] Получен сигнал завершения.")
	case <-ctx.Done():
		fmt.Println("\n[main] Контекст отменён.")
	}

	// 9. Корректно останавливаем всё.
	if recognizer != nil {
		recognizer.Stop()
	}
	disp.Close()
	mgr.StopAll()
	fmt.Println("[main] Завершено.")
}