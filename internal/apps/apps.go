// Package apps находит установленные приложения и умеет их запускать.
//
// Linux: разбирает .desktop-файлы.
// Windows: смотрит ярлыки в меню «Пуск» + встроенные приложения.
// macOS: смотрит /Applications.
package apps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"
)

// CustomApp — приложение, добавленное пользователем в config.json.
type CustomApp struct {
	Name    string
	Aliases []string
	Linux   string
	Windows string
	Darwin  string
}

// App — найденное приложение.
type App struct {
	Name    string
	Aliases []string
	Target  string // .desktop/.lnk/.app или команда запуска

	launch func() error
}

// Launch запускает приложение.
func (a App) Launch() error {
	if a.launch == nil {
		return fmt.Errorf("не знаю, как запустить %q", a.Name)
	}
	return a.launch()
}

// Resolver ищет приложения по имени.
type Resolver struct {
	apps []App
	keys map[string][]int
}

// New собирает список приложений: встроенные + системные + пользовательские.
func New(custom map[string]CustomApp) *Resolver {
	r := &Resolver{keys: make(map[string][]int)}

	for name, c := range custom {
		if c.Name == "" {
			c.Name = name
		}
		cmd := commandForOS(c)
		if cmd == "" {
			continue
		}
		display := c.Name
		app := App{
			Name:    display,
			Aliases: append([]string{name}, c.Aliases...),
			Target:  cmd,
			launch: func() error {
				return launchCommand(cmd)
			},
		}
		r.add(app)
	}
	for _, app := range seedApps() {
		r.add(app)
	}
	for _, app := range systemApps() {
		r.add(app)
	}

	sort.SliceStable(r.apps, func(i, j int) bool {
		return strings.ToLower(r.apps[i].Name) < strings.ToLower(r.apps[j].Name)
	})
	r.rebuildIndex()
	return r
}

func commandForOS(c CustomApp) string {
	switch runtime.GOOS {
	case "windows":
		return c.Windows
	case "darwin":
		return c.Darwin
	default:
		if c.Linux != "" {
			return c.Linux
		}
		// Разумный запасной вариант: одна и та же команда на Unix.
		if c.Darwin != "" {
			return c.Darwin
		}
		return c.Windows
	}
}

// List возвращает приложения без дублей, отсортированные по имени.
func (r *Resolver) List() []App {
	seen := make(map[string]struct{}, len(r.apps))
	out := make([]App, 0, len(r.apps))
	for _, app := range r.apps {
		key := normalize(app.Name)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, app)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// Find ищет приложение по имени, псевдониму или части названия.
func (r *Resolver) Find(query string) (App, bool) {
	q := normalize(query)
	if q == "" {
		return App{}, false
	}

	if idxs, ok := r.keys[q]; ok && len(idxs) > 0 {
		return r.apps[idxs[0]], true
	}

	best := -1
	bestScore := 0
	tq := translit(q)
	for i, app := range r.apps {
		score := matchScore(q, tq, app)
		if score > bestScore {
			best = i
			bestScore = score
		}
	}
	if best < 0 || bestScore < 3 {
		return App{}, false
	}
	return r.apps[best], true
}

func (r *Resolver) add(app App) {
	app.Name = strings.TrimSpace(app.Name)
	if app.Name == "" || app.launch == nil {
		return
	}
	r.apps = append(r.apps, app)
}

func (r *Resolver) rebuildIndex() {
	r.keys = make(map[string][]int)
	for i, app := range r.apps {
		names := append([]string{app.Name}, app.Aliases...)
		for _, name := range names {
			for _, key := range nameKeys(name) {
				if key == "" {
					continue
				}
				r.keys[key] = append(r.keys[key], i)
			}
		}
	}
}

func matchScore(q, tq string, app App) int {
	names := append([]string{app.Name}, app.Aliases...)
	best := 0
	for _, name := range names {
		n := normalize(name)
		if n == "" {
			continue
		}
		tn := translit(n)
		switch {
		case n == q || tn == tq || tn == q || n == tq:
			return 100
		case strings.HasPrefix(n, q) || strings.HasPrefix(tn, tq):
			best = max(best, 70+min(len(q), 20))
		case strings.HasPrefix(q, n) || strings.HasPrefix(tq, tn):
			best = max(best, 60+min(len(n), 20))
		case strings.Contains(n, q) || strings.Contains(tn, tq):
			best = max(best, 40+min(len(q), 20))
		case tokensMatch(q, n) || tokensMatch(tq, tn):
			best = max(best, 30+min(len(q), 20))
		}
	}
	return best
}

func tokensMatch(query, name string) bool {
	qt := strings.Fields(query)
	if len(qt) == 0 {
		return false
	}
	nt := strings.Fields(name)
	for _, q := range qt {
		found := false
		for _, n := range nt {
			if len(q) >= 3 && (strings.Contains(n, q) || strings.Contains(q, n)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// nameKeys возвращает варианты ключей для индекса.
func nameKeys(name string) []string {
	n := normalize(name)
	if n == "" {
		return nil
	}
	keys := []string{n, translit(n)}
	// Отдельные слова: «Telegram Desktop» → «telegram».
	for _, token := range strings.Fields(n) {
		if len([]rune(token)) >= 3 {
			keys = append(keys, token, translit(token))
		}
	}
	return keys
}

// normalize приводит имя к нижнему регистру без пунктуации и пробелов.
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '+' || r == '#':
			b.WriteRune(r)
		}
	}
	return stemRu(b.String())
}

// stemRu грубо отрезает русское падежное окончание. Этого достаточно,
// чтобы «музыку» находило «Музыка», а «настройки» — «Настройки».
func stemRu(s string) string {
	runes := []rune(s)
	if len(runes) <= 3 {
		return s
	}
	last := runes[len(runes)-1]
	if !unicode.Is(unicode.Cyrillic, last) {
		return s
	}
	switch last {
	case 'а', 'е', 'ё', 'и', 'о', 'у', 'ы', 'э', 'ю', 'я', 'й', 'ь':
		return string(runes[:len(runes)-1])
	}
	return s
}

var translitMap = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e",
	'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
	'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch",
	'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

// translit переводит кириллицу в латиницу, чтобы «телеграм» находил Telegram.
func translit(s string) string {
	var b strings.Builder
	for _, r := range s {
		if v, ok := translitMap[r]; ok {
			b.WriteString(v)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// launchCommand запускает команду через оболочку, чтобы работали аргументы.
func launchCommand(command string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// start нужен и для ярлыков, и для URI вроде ms-settings:.
		cmd = exec.Command("cmd", "/c", "start", "", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// launchFile открывает файл/ярлык средствами ОС.
func launchFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// openAppBundle открывает .app на macOS.
func openAppBundle(path string) error {
	name := strings.TrimSuffix(filepath.Base(path), ".app")
	cmd := exec.Command("open", "-a", name)
	if err := cmd.Start(); err != nil {
		return launchFile(path)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// PathExists — небольшой помощник для диагностики.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
