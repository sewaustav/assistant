package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sewaustav/assistant/internal/dispatcher"
	"github.com/sewaustav/assistant/internal/speech_recognition"
)

func main() {
	fmt.Println("=== Assistant ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Создаём Recognizer (микрофон + gigastt)
	recognizer, err := speech_recognition.NewRecognizer()
	if err != nil {
		log.Fatalf("Не удалось создать Recognizer: %v", err)
	}

	// 2. Запускаем распознавание
	textChan, err := recognizer.Start(ctx)
	if err != nil {
		log.Fatalf("Не удалось запустить распознавание: %v", err)
	}

	// 3. Собираем Dispatcher.
	// stopFn — что делать при команде "выход".
	disp := dispatcher.New(textChan, func() {
		log.Println("[main] Получена команда выхода, останавливаемся.")
		cancel()
	}, recognizer)

	// 4. Запускаем диспетчер в отдельной горутине
	go disp.Run(ctx)

	// 5. Ждём сигнала завершения (Ctrl+C или команда "выход")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("\n[main] Получен сигнал завершения.")
	case <-ctx.Done():
		fmt.Println("\n[main] Контекст отменён.")
	}

	// 6. Корректно останавливаем всё
	recognizer.Stop()
	disp.Close()
	fmt.Println("[main] Завершено.")
	fmt.Println("[main] Завершено.")
}