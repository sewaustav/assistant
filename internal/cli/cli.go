package cli

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/sewaustav/assistant/internal/apps"
	"github.com/sewaustav/assistant/internal/config"
)

// Version отображается командой /version.
var Version = "dev"

// CLI — простой интерфейс командной строки.
type CLI struct {
	cfg       *config.Config
	inputChan chan<- string
	cmdChan   chan<- string
	resolver  *apps.Resolver

	mu   sync.RWMutex
	mode config.Mode
}

// New создаёт CLI.
func New(cfg *config.Config, inputChan chan<- string, cmdChan chan<- string, resolver *apps.Resolver) *CLI {
	return &CLI{
		cfg:       cfg,
		inputChan: inputChan,
		cmdChan:   cmdChan,
		resolver:  resolver,
		mode:      cfg.Mode,
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

		// Локальные команды CLI.
		if strings.HasPrefix(line, "/") {
			if c.handleLocalCommand(line) {
				continue
			}
		}

		// Команды управления (/mode, /quit, @...).
		if strings.HasPrefix(line, "/") || strings.HasPrefix(line, "@") {
			c.cmdChan <- line
			if line == "/quit" || line == "/exit" {
				return
			}
			continue
		}

		// Обычный текст.
		if c.getMode() == config.ModeText {
			c.inputChan <- line
		} else {
			PrintInfo("Голосовой режим: текст игнорируется, используйте /mode text")
		}
	}
}

// handleLocalCommand обрабатывает команды, которым не нужен диспетчер.
// Возвращает true, если команда обработана.
func (c *CLI) handleLocalCommand(line string) bool {
	parts := strings.Fields(strings.TrimLeft(line, "/"))
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case "help", "h", "?":
		c.printHelp()
		return true
	case "apps", "приложения", "программы":
		c.printApps()
		return true
	case "version", "ver":
		PrintInfo("assistant %s (%s/%s)", Version, osName(), osArch())
		return true
	}
	return false
}

func (c *CLI) printHelp() {
	fmt.Println()
	fmt.Println("Команды CLI:")
	fmt.Println("  /mode voice | /mode text  — переключить режим")
	fmt.Println("  /apps                     — список найденных приложений")
	fmt.Println("  /help                     — эта справка")
	fmt.Println("  /version                  — версия")
	fmt.Println("  /quit                     — выход")
	fmt.Println()
	fmt.Println("Голосом можно сказать: «помощь», «открой калькулятор»,")
	fmt.Println("«скопируй», «вставь», «отмени», «тише», «скриншот» и т.п.")
	fmt.Println()
}

func (c *CLI) printApps() {
	if c.resolver == nil {
		PrintInfo("Список приложений недоступен.")
		return
	}
	list := c.resolver.List()
	if len(list) == 0 {
		PrintInfo("Приложения не найдены.")
		return
	}
	names := make([]string, 0, len(list))
	for _, app := range list {
		names = append(names, app.Name)
	}
	const max = 80
	if len(names) > max {
		names = append(names[:max], fmt.Sprintf("… и ещё %d", len(names)-max))
	}
	PrintInfo("Найдено приложений: %d", len(list))
	fmt.Println(strings.Join(names, ", "))
}

// printBanner выводит приветствие и текущий режим.
func (c *CLI) printBanner() {
	fmt.Println("=== Assistant ===")
	fmt.Printf("Режим: %s\n", c.getMode())
	fmt.Printf("Версия: %s\n", Version)
	fmt.Println("Команды: /mode voice | /mode text | /apps | /help | /quit")
}

// printPrompt выводит приглашение с учётом режима.
func (c *CLI) printPrompt() {
	if c.getMode() == config.ModeText {
		fmt.Print("> ")
	} else {
		fmt.Print("[voice] > ")
	}
}

// UpdateMode обновляет отображаемый режим (вызывается диспетчером при смене).
func (c *CLI) UpdateMode(mode config.Mode) {
	c.mu.Lock()
	c.mode = mode
	c.mu.Unlock()
}

func (c *CLI) getMode() config.Mode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mode
}

func osName() string { return runtime.GOOS }
func osArch() string { return runtime.GOARCH }
