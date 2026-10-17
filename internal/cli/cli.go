package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sewaustav/assistant/internal/config"
)

// CLI — простой интерфейс командной строки.
type CLI struct {
	cfg      *config.Config
	inputChan chan<- string // канал для отправки текста в диспетчер
	cmdChan   chan<- string // канал для отправки команд управления
}

// New создаёт CLI.
func New(cfg *config.Config, inputChan chan<- string, cmdChan chan<- string) *CLI {
	return &CLI{
		cfg:       cfg,
		inputChan: inputChan,
		cmdChan:   cmdChan,
	}
}

// Run запускает цикл чтения ввода.
func (c *CLI) Run() {
	c.printBanner()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		c.printPrompt()

		if !scanner.Scan() {
			// EOF (Ctrl+D) — выходим.
			fmt.Println("\n[cli] Ввод завершён.")
			c.cmdChan <- "/quit"
			return
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Обработка команд управления (начинаются с / или @).
		if strings.HasPrefix(line, "/") || strings.HasPrefix(line, "@") {
			c.cmdChan <- line
			continue
		}

		// Обычный текст.
		// В голосовом режиме — игнорируем (только команды).
		// В текстовом — отправляем в диспетчер.
		if c.cfg.Mode == config.ModeText {
			c.inputChan <- line
		} else {
			// Голосовой режим: текст игнорируется, но можно показать подсказку.
			fmt.Println("[cli] (голосовой режим: текст игнорируется, используйте /mode text)")
		}
	}
}

// printBanner выводит приветствие и текущий режим.
func (c *CLI) printBanner() {
	fmt.Println("=== Assistant CLI ===")
	fmt.Printf("Режим: %s\n", c.cfg.Mode)
	fmt.Println("Команды: /mode voice | /mode text | /quit")
	fmt.Println("В голосовом режиме текст игнорируется, работают только команды.")
	fmt.Println("=====================")
}

// printPrompt выводит приглашение с учётом режима.
func (c *CLI) printPrompt() {
	if c.cfg.Mode == config.ModeText {
		fmt.Print("> ")
	} else {
		fmt.Print("[voice] > ")
	}
}

// UpdateMode обновляет отображаемый режим (вызывается диспетчером при смене).
func (c *CLI) UpdateMode(mode config.Mode) {
	c.cfg.Mode = mode
	fmt.Printf("\n[cli] Режим переключён на: %s\n", mode)
}