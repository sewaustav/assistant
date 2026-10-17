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

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config: %v", err)
	}

	textInputChan := make(chan string, 10)
		cmdChan := make(chan string, 10)
	
		// Запускаем CLI в отдельной горутине.
		cliClient := cli.New(cfg, textInputChan, cmdChan)
		go cliClient.Run()

	path, _ := config.Path()
	log.Printf("[main] Конфиг: %s", path)
	log.Printf("[main] Режим: %s, TTS speaker: %s, ASR enabled: %v",
		cfg.Mode, cfg.TTS.Speaker, cfg.ASR.Enabled)

	mgr := lifecycle.NewManager()
	
	// gigastt
	gigasttPath := filepath.Join(os.Getenv("HOME"), ".cargo/bin/gigastt")
	mgr.Register("gigastt", lifecycle.NewGigaSTT(gigasttPath))
	
	// Silero: корень проекта + tts-server
	projectRoot, _ := os.Getwd() // или os.Executable() для бинарника
	ttsDir := filepath.Join(projectRoot, "tts-server")
	pythonPath := filepath.Join(ttsDir, ".venv/bin/python")
	mgr.Register("silero", lifecycle.NewSilero(pythonPath, ttsDir))
	
	if err := mgr.Start(ctx, "gigastt"); err != nil {
    log.Fatalf("gigastt: %v", err)
	}
	if err := mgr.Start(ctx, "silero"); err != nil {
    log.Fatalf("silero: %v", err)
	}

	recognizer, err := speech_recognition.NewRecognizer()
	if err != nil {
		log.Fatalf("Не удалось создать Recognizer: %v", err)
	}

	textChan, err := recognizer.Start(ctx)
	if err != nil {
		log.Fatalf("Не удалось запустить распознавание: %v", err)
	}

	disp := dispatcher.New(textChan, func() {
		log.Println("[main] Получена команда выхода, останавливаемся.")
		cancel()
	}, recognizer)

	go disp.Run(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("\n[main] Получен сигнал завершения.")
	case <-ctx.Done():
		fmt.Println("\n[main] Контекст отменён.")
	}

	recognizer.Stop()
	disp.Close()
	fmt.Println("[main] Завершено.")
	fmt.Println("[main] Завершено.")
}