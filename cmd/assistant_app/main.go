package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sewaustav/assistant/internal/config"
	"github.com/sewaustav/assistant/internal/dispatcher"
	"github.com/sewaustav/assistant/internal/speech_recognition"
)

func main() {
	fmt.Println("=== Assistant ===")

	fmt.Println("=== Assistant ===")
	
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config: %v", err)
	}

	path, _ := config.Path()
	log.Printf("[main] Конфиг: %s", path)
	log.Printf("[main] Режим: %s, TTS speaker: %s, ASR enabled: %v",
		cfg.Mode, cfg.TTS.Speaker, cfg.ASR.Enabled)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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