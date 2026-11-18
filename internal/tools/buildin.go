package tools

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sewaustav/assistant/internal/apps"
	"github.com/sewaustav/assistant/internal/config"
	"github.com/sewaustav/assistant/internal/tools/platform"
)

// Options — зависимости встроенных инструментов.
type Options struct {
	Apps       *apps.Resolver
	SearchURL  string
	BrowserURL string
}

// TimeTool возвращает текущее время.
type TimeTool struct{}

func (t *TimeTool) Name() string        { return "time" }
func (t *TimeTool) Description() string { return "Get the current time" }
func (t *TimeTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	now := time.Now()
	return fmt.Sprintf("Сейчас %d часов %d минут", now.Hour(), now.Minute()), nil
}

// DateTool возвращает текущую дату.
type DateTool struct{}

func (t *DateTool) Name() string        { return "date" }
func (t *DateTool) Description() string { return "Get today's date" }
func (t *DateTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	now := time.Now()
	months := [...]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
	weekdays := [...]string{"воскресенье", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота"}
	return fmt.Sprintf("Сегодня %d %s %d года, %s", now.Day(), months[int(now.Month())-1], now.Year(), weekdays[int(now.Weekday())]), nil
}

// BrowserTool открывает браузер, URL или поиск.
type BrowserTool struct {
	opts Options
}

func (t *BrowserTool) Name() string        { return "browser" }
func (t *BrowserTool) Description() string { return "Open the default web browser or a URL" }

func (t *BrowserTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	rawURL := strings.TrimSpace(args["url"])
	if rawURL == "" {
		if query := strings.TrimSpace(args["query"]); query != "" {
			return openSearch(t.opts, query)
		}
		rawURL = t.opts.BrowserURL
		if rawURL == "" {
			rawURL = "https://www.google.com"
		}
	}
	rawURL = ensureScheme(rawURL)
	if err := platform.OpenBrowser(rawURL); err != nil {
		return "", fmt.Errorf("не удалось открыть браузер: %w", err)
	}
	return "Открыл браузер", nil
}

var urlPattern = regexp.MustCompile(`(?i)(https?://[^\s]+|www\.[^\s]+|[a-zа-яё0-9-]+\.(?:[a-zа-яё]{2,}|рф)(?:/[^\s]*)?)`)

func (t *BrowserTool) ParseArgs(text string) map[string]string {
	if u := findURL(text); u != "" {
		return map[string]string{"url": u}
	}
	for _, word := range strings.Fields(strings.ToLower(text)) {
		word = strings.Trim(word, ".,!?;:«»\"'")
		if u, ok := siteAliases[word]; ok {
			return map[string]string{"url": u}
		}
	}
	return nil
}

// SearchWebTool ищет в интернете через браузер.
type SearchWebTool struct {
	opts Options
}

func (t *SearchWebTool) Name() string        { return "search_web" }
func (t *SearchWebTool) Description() string { return "Search the web in the default browser" }

func (t *SearchWebTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	query := strings.TrimSpace(args["query"])
	if query == "" {
		return "", fmt.Errorf("не понял, что искать")
	}
	return openSearch(t.opts, query)
}

func (t *SearchWebTool) ParseArgs(text string) map[string]string {
	query := stripWords(text, searchStopWords)
	if query == "" {
		return nil
	}
	return map[string]string{"query": query}
}

// OpenAppTool открывает установленное приложение по имени.
type OpenAppTool struct {
	resolver *apps.Resolver
}

func (t *OpenAppTool) Name() string        { return "open_app" }
func (t *OpenAppTool) Description() string { return "Open an installed application by name" }

func (t *OpenAppTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	name := strings.TrimSpace(args["app"])
	if name == "" {
		return "", fmt.Errorf("не понял, какое приложение открыть")
	}
	if t.resolver == nil {
		return "", fmt.Errorf("список приложений недоступен")
	}
	app, ok := t.resolver.Find(name)
	if !ok {
		// Если приложение не установлено, но имя похоже на сайт
		// («открой ютуб», «открой карты»), открываем его в браузере.
		if site, found := findSite(name); found {
			if err := platform.OpenBrowser(site); err != nil {
				return "", fmt.Errorf("не удалось открыть сайт %q: %w", name, err)
			}
			return "Открываю сайт " + name, nil
		}
		return "", fmt.Errorf("не нашёл приложение %q", name)
	}
	if err := app.Launch(); err != nil {
		return "", fmt.Errorf("не удалось открыть %s: %w", app.Name, err)
	}
	return "Открываю " + app.Name, nil
}

func (t *OpenAppTool) ParseArgs(text string) map[string]string {
	name := stripWords(text, appStopWords)
	if name == "" {
		return nil
	}
	return map[string]string{"app": name}
}

// HotkeyTool — простая команда «нажать комбинацию».
type HotkeyTool struct {
	ToolName        string
	ToolDescription string
	Keys            string
	Result          string
}

func (t *HotkeyTool) Name() string        { return t.ToolName }
func (t *HotkeyTool) Description() string { return t.ToolDescription }
func (t *HotkeyTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	if err := platform.SendHotkey(t.Keys); err != nil {
		return "", fmt.Errorf("%s: %w", strings.ToLower(t.ToolDescription), err)
	}
	return t.Result, nil
}

// VolumeTool меняет громкость.
type VolumeTool struct {
	up bool
}

func (t *VolumeTool) Name() string {
	if t.up {
		return "volume_up"
	}
	return "volume_down"
}
func (t *VolumeTool) Description() string {
	if t.up {
		return "Turn the volume up"
	}
	return "Turn the volume down"
}
func (t *VolumeTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	steps := 3
	if raw := strings.TrimSpace(args["steps"]); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 20 {
			steps = n
		}
	}
	key := "volumedown"
	result := "Сделал тише"
	if t.up {
		key = "volumeup"
		result = "Сделал громче"
	}
	for i := 0; i < steps; i++ {
		if err := platform.SendHotkey(key); err != nil {
			return "", fmt.Errorf("не удалось изменить громкость: %w", err)
		}
	}
	return result, nil
}
func (t *VolumeTool) ParseArgs(text string) map[string]string {
	if n := extractNumber(text); n > 0 {
		if n > 20 {
			n = 20
		}
		return map[string]string{"steps": strconv.Itoa(n)}
	}
	return nil
}

// MediaTool управляет воспроизведением.
type MediaTool struct {
	name   string
	desc   string
	keys   string
	result string
}

func (t *MediaTool) Name() string        { return t.name }
func (t *MediaTool) Description() string { return t.desc }
func (t *MediaTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	if err := platform.SendHotkey(t.keys); err != nil {
		return "", fmt.Errorf("не удалось управлять воспроизведением: %w", err)
	}
	return t.result, nil
}

// LockTool блокирует экран.
type LockTool struct{}

func (t *LockTool) Name() string        { return "lock_screen" }
func (t *LockTool) Description() string { return "Lock the screen" }
func (t *LockTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	if err := platform.LockScreen(); err != nil {
		return "", fmt.Errorf("не удалось заблокировать экран: %w", err)
	}
	return "Экран заблокирован", nil
}

// ScreenshotTool делает снимок экрана.
type ScreenshotTool struct{}

func (t *ScreenshotTool) Name() string        { return "screenshot" }
func (t *ScreenshotTool) Description() string { return "Take a screenshot" }
func (t *ScreenshotTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	path, err := platform.Screenshot()
	if err != nil {
		return "", fmt.Errorf("не удалось сделать скриншот: %w", err)
	}
	return "Скриншот сохранён: " + path, nil
}

// TypeTextTool печатает текст в активное поле.
type TypeTextTool struct{}

func (t *TypeTextTool) Name() string        { return "type_text" }
func (t *TypeTextTool) Description() string { return "Type text into the active input field" }
func (t *TypeTextTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	text := strings.TrimSpace(args["text"])
	if text == "" {
		return "", fmt.Errorf("не понял, какой текст напечатать")
	}
	if err := platform.TypeText(text); err != nil {
		return "", fmt.Errorf("не удалось напечатать текст: %w", err)
	}
	return "Напечатал", nil
}
func (t *TypeTextTool) ParseArgs(text string) map[string]string {
	out := stripWords(text, typeStopWords)
	if out == "" {
		return nil
	}
	return map[string]string{"text": out}
}

// HelpTool перечисляет доступные команды.
type HelpTool struct {
	commands []string
}

func (t *HelpTool) Name() string        { return "help" }
func (t *HelpTool) Description() string { return "List available voice commands" }
func (t *HelpTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	if len(t.commands) == 0 {
		return "Команды не найдены", nil
	}
	return "Доступные команды: " + strings.Join(t.commands, ", "), nil
}

// wrappedTool связывает инструмент с конфигом команды.
type wrappedTool struct {
	inner       any
	name        string
	description string
	keywords    []string
	params      map[string]config.ParameterConfig
	local       bool
	llm         bool
}

func (w *wrappedTool) Name() string        { return w.name }
func (w *wrappedTool) Description() string { return w.description }
func (w *wrappedTool) Keywords() []string  { return w.keywords }
func (w *wrappedTool) LocalEnabled() bool  { return w.local }
func (w *wrappedTool) LLMEnabled() bool    { return w.llm }

func (w *wrappedTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	type execer interface {
		Execute(context.Context, map[string]string) (string, error)
	}
	if e, ok := w.inner.(execer); ok {
		return e.Execute(ctx, args)
	}
	return "", fmt.Errorf("tool %q не умеет Execute", w.name)
}

// ParseArgs извлекает аргументы из обычной фразы.
func (w *wrappedTool) ParseArgs(text string) map[string]string {
	type parser interface {
		ParseArgs(string) map[string]string
	}
	if p, ok := w.inner.(parser); ok {
		return p.ParseArgs(text)
	}
	return nil
}

// Parameters возвращает JSON-схему аргументов для LLM.
func (w *wrappedTool) Parameters() map[string]any {
	if len(w.params) == 0 {
		return nil
	}
	properties := make(map[string]any, len(w.params))
	var required []string
	for name, p := range w.params {
		typ := p.Type
		switch typ {
		case "integer", "number", "boolean", "string", "array", "object":
		default:
			typ = "string"
		}
		prop := map[string]any{"type": typ}
		if p.Description != "" {
			prop["description"] = p.Description
		}
		properties[name] = prop
		if p.Required {
			required = append(required, name)
		}
	}
	sort.Strings(required)
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func isPlatformSupported(platforms []string) bool {
	if len(platforms) == 0 {
		return true
	}
	for _, p := range platforms {
		if p == runtime.GOOS {
			return true
		}
	}
	return false
}

// BuildFromConfig собирает инструменты из commands.json.
func BuildFromConfig(cmds config.CommandsConfig, opts Options) []any {
	names := make([]string, 0, len(cmds))
	for name := range cmds {
		names = append(names, name)
	}
	sort.Strings(names)

	var result []any
	for _, name := range names {
		cfg := cmds[name]
		if !cfg.LocalEnabled && !cfg.LLMEnabled {
			continue
		}
		if !isPlatformSupported(cfg.Platforms) {
			continue
		}
		base := newTool(cfg.Action.Name, opts, cmds)
		if base == nil {
			continue
		}
		result = append(result, &wrappedTool{
			inner:       base,
			name:        name,
			description: cfg.Description,
			keywords:    cfg.Keywords,
			params:      cfg.Parameters,
			local:       cfg.LocalEnabled,
			llm:         cfg.LLMEnabled,
		})
	}
	return result
}

func newTool(action string, opts Options, cmds config.CommandsConfig) any {
	switch action {
	case "time":
		return &TimeTool{}
	case "date":
		return &DateTool{}
	case "browser":
		return &BrowserTool{opts: opts}
	case "search_web":
		return &SearchWebTool{opts: opts}
	case "open_app":
		return &OpenAppTool{resolver: opts.Apps}
	case "select_all":
		return &HotkeyTool{ToolName: action, ToolDescription: "Select all text", Keys: "ctrl+a", Result: "Выделил всё"}
	case "copy":
		return &HotkeyTool{ToolName: action, ToolDescription: "Copy selected text", Keys: "ctrl+c", Result: "Скопировал"}
	case "cut":
		return &HotkeyTool{ToolName: action, ToolDescription: "Cut selected text", Keys: "ctrl+x", Result: "Вырезал"}
	case "paste":
		return &HotkeyTool{ToolName: action, ToolDescription: "Paste from clipboard", Keys: "ctrl+v", Result: "Вставил"}
	case "delete":
		return &HotkeyTool{ToolName: action, ToolDescription: "Delete selected text", Keys: "delete", Result: "Удалил"}
	case "undo":
		return &HotkeyTool{ToolName: action, ToolDescription: "Undo last action", Keys: "ctrl+z", Result: "Отменил"}
	case "redo":
		return &HotkeyTool{ToolName: action, ToolDescription: "Redo last undone action", Keys: redoKeys(), Result: "Повторил"}
	case "close_window":
		return &HotkeyTool{ToolName: action, ToolDescription: "Close the active window", Keys: closeKeys(), Result: "Закрыл окно"}
	case "new_tab":
		return &HotkeyTool{ToolName: action, ToolDescription: "Open a new tab", Keys: "ctrl+t", Result: "Открыл новую вкладку"}
	case "reload_page":
		return &HotkeyTool{ToolName: action, ToolDescription: "Reload the current page", Keys: "ctrl+r", Result: "Обновил страницу"}
	case "save":
		return &HotkeyTool{ToolName: action, ToolDescription: "Save the current document", Keys: "ctrl+s", Result: "Сохранил"}
	case "volume_up":
		return &VolumeTool{up: true}
	case "volume_down":
		return &VolumeTool{up: false}
	case "volume_mute":
		return &HotkeyTool{ToolName: action, ToolDescription: "Mute or unmute the sound", Keys: "mute", Result: "Переключил звук"}
	case "media_play_pause":
		return &MediaTool{name: action, desc: "Play or pause media", keys: "playpause", result: "Переключил воспроизведение"}
	case "media_next":
		return &MediaTool{name: action, desc: "Play the next track", keys: "medianext", result: "Следующий трек"}
	case "media_prev":
		return &MediaTool{name: action, desc: "Play the previous track", keys: "mediaprev", result: "Предыдущий трек"}
	case "lock_screen":
		return &LockTool{}
	case "screenshot":
		return &ScreenshotTool{}
	case "type_text":
		return &TypeTextTool{}
	case "help":
		return &HelpTool{commands: commandTitles(cmds)}
	default:
		return nil
	}
}

// commandTitles возвращает короткий список действий для команды «помощь».
func commandTitles(cmds config.CommandsConfig) []string {
	titles := map[string]string{
		"time": "время", "date": "дата", "browser": "браузер", "search_web": "поиск в интернете",
		"open_app": "открыть программу", "select_all": "выделить всё", "copy": "скопировать",
		"cut": "вырезать", "paste": "вставить", "delete": "удалить", "undo": "отменить",
		"redo": "повторить", "close_window": "закрыть окно", "new_tab": "новая вкладка",
		"reload_page": "обновить страницу", "save": "сохранить", "volume_up": "громче",
		"volume_down": "тише", "volume_mute": "выключить звук", "media_play_pause": "пауза",
		"media_next": "следующий трек", "media_prev": "предыдущий трек", "lock_screen": "блокировка",
		"screenshot": "скриншот", "type_text": "напечатать текст", "help": "помощь",
	}
	seen := make(map[string]struct{})
	var out []string
	for _, cmd := range cmds {
		title, ok := titles[cmd.Action.Name]
		if !ok {
			title = cmd.Action.Name
		}
		if _, dup := seen[title]; dup {
			continue
		}
		seen[title] = struct{}{}
		out = append(out, title)
	}
	sort.Strings(out)
	return out
}

var (
	appStopWords = []string{
		"открой", "откройте", "открыть", "запусти", "запустите", "запустить",
		"включи", "включить", "пожалуйста", "мне", "приложение", "приложении",
		"программу", "программа", "программе", "прилу", "ассистент",
	}
	searchStopWords = []string{
		"найди", "найдите", "поищи", "поищите", "загугли", "загуглите",
		"поиск", "в", "во", "интернете", "интернет", "сети", "сеть",
		"пожалуйста", "мне", "для", "про",
	}
	typeStopWords = []string{
		"напиши", "напишите", "напечатай", "напечатайте", "введи", "введите",
		"текст", "пожалуйста", "мне", "это",
	}
	siteAliases = map[string]string{
		"ютуб": "https://youtube.com", "youtube": "https://youtube.com",
		"вк": "https://vk.com", "vk": "https://vk.com",
		"гугл": "https://google.com", "google": "https://google.com",
		"яндекс": "https://ya.ru", "yandex": "https://ya.ru",
		"телеграм": "https://web.telegram.org", "telegram": "https://web.telegram.org",
		"гитхаб": "https://github.com", "github": "https://github.com",
		"почта": "https://mail.ru", "mail": "https://mail.ru",
		"карты": "https://yandex.ru/maps", "maps": "https://yandex.ru/maps",
		"погода": "https://yandex.ru/pogoda",
	}
)

func stripWords(text string, stop []string) string {
	stopSet := make(map[string]struct{}, len(stop))
	for _, w := range stop {
		stopSet[strings.ToLower(w)] = struct{}{}
	}
	var out []string
	for _, word := range strings.Fields(text) {
		clean := strings.ToLower(strings.Trim(word, ".,!?;:«»\"'"))
		if _, skip := stopSet[clean]; skip {
			continue
		}
		out = append(out, word)
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

func findURL(text string) string {
	match := urlPattern.FindString(text)
	if match == "" {
		return ""
	}
	return match
}

// findSite ищет адрес сайта по алиасу в произвольной фразе.
func findSite(text string) (string, bool) {
	clean := strings.ToLower(strings.Trim(text, " .,!?;:«»\"'"))
	if u, ok := siteAliases[clean]; ok {
		return u, true
	}
	for _, word := range strings.Fields(clean) {
		word = strings.Trim(word, ".,!?;:«»\"'")
		if u, ok := siteAliases[word]; ok {
			return u, true
		}
	}
	return "", false
}

func ensureScheme(raw string) string {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return "https://" + raw
}

func openSearch(opts Options, query string) (string, error) {
	template := opts.SearchURL
	if template == "" {
		template = "https://yandex.ru/search/?text={query}"
	}
	escaped := url.QueryEscape(query)
	var target string
	switch {
	case strings.Contains(template, "{query}"):
		target = strings.ReplaceAll(template, "{query}", escaped)
	case strings.Contains(template, "%s"):
		target = fmt.Sprintf(template, escaped)
	default:
		sep := "?"
		if strings.Contains(template, "?") {
			sep = "&"
		}
		target = template + sep + "q=" + escaped
	}
	if err := platform.OpenBrowser(target); err != nil {
		return "", fmt.Errorf("не удалось открыть браузер: %w", err)
	}
	return "Ищу: " + query, nil
}

func extractNumber(text string) int {
	re := regexp.MustCompile(`\d+`)
	if raw := re.FindString(text); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			return n
		}
	}
	return 0
}

func redoKeys() string {
	if runtime.GOOS == "windows" {
		return "ctrl+y"
	}
	return "ctrl+shift+z"
}

func closeKeys() string {
	if runtime.GOOS == "darwin" {
		return "ctrl+w"
	}
	return "alt+f4"
}
