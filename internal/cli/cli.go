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
	cfg       *config.Config
	inputChan chan<- string
	cmdChan   chan<- string
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
			PrintInfo("Ввод завершён.")
			c.cmdChan <- "/quit"
			return
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Команды управления (/mode, /quit, @...).
		if strings.HasPrefix(line, "/") || strings.HasPrefix(line, "@") {
			c.cmdChan <- line
			continue
		}

		// Обычный текст.
		if c.cfg.Mode == config.ModeText {
			c.inputChan <- line
		} else {
			PrintInfo("Голосовой режим: текст игнорируется, используйте /mode text")
		}
	}
}

// printBanner выводит приветствие и текущий режим.
func (c *CLI) printBanner() {
	fmt.Println("=== Assistant ===")
	fmt.Printf("Режим: %s\n", c.cfg.Mode)
	fmt.Println("Команды: /mode voice | /mode text | /quit")
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
	PrintInfo("Режим переключён на: %s", mode)
}