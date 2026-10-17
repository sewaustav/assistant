package lifecycle

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

type Service struct {
	Name      string
	Cmd       *exec.Cmd
	ReadyURL  string
	HealthURL string
	mu        sync.Mutex
	started   bool
}

type Manager struct {
	services map[string]*Service
}

func NewManager() *Manager {
	return &Manager{services: make(map[string]*Service)}
}

func (m *Manager) Register(name string, svc *Service) {
	m.services[name] = svc
}

func (m *Manager) Start(ctx context.Context, name string) error {
	svc, ok := m.services[name]
	if !ok {
		return fmt.Errorf("сервис %q не зарегистрирован", name)
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.started {
		log.Printf("[lifecycle] %s уже запущен", name)
		return nil
	}

	log.Printf("[lifecycle] Запуск %s...", name)

	// Платформозависимая настройка (Setpgid на Unix, CREATE_NEW_PROCESS_GROUP на Windows)
	configureProcessGroup(svc.Cmd)

	if err := svc.Cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить %s: %w", name, err)
	}

	if err := waitReady(ctx, svc); err != nil {
		killProcessTree(svc.Cmd)
		return fmt.Errorf("%s не стал готов: %w", name, err)
	}

	svc.started = true
	log.Printf("[lifecycle] %s готов", name)
	return nil
}

func (m *Manager) Stop(name string) error {
	svc, ok := m.services[name]
	if !ok {
		return fmt.Errorf("сервис %q не зарегистрирован", name)
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if !svc.started || svc.Cmd.Process == nil {
		log.Printf("[lifecycle] %s не запущен", name)
		return nil
	}

	log.Printf("[lifecycle] Остановка %s...", name)
	killProcessTree(svc.Cmd)
	svc.started = false
	log.Printf("[lifecycle] %s остановлен", name)
	return nil
}

func (m *Manager) StopAll() {
	for name := range m.services {
		_ = m.Stop(name)
	}
}

func waitReady(ctx context.Context, svc *Service) error {
	url := svc.ReadyURL
	if url == "" {
		url = svc.HealthURL
	}
	if url == "" {
		return nil
	}

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("таймаут ожидания готовности (%s)", url)
}

// NewGigaSTT создаёт описание сервиса gigastt.
func NewGigaSTT(binPath string) *Service {
	return &Service{
		Name:      "gigastt",
		Cmd:       exec.Command(binPath, "serve"),
		ReadyURL:  "http://127.0.0.1:9876/ready",
		HealthURL: "http://127.0.0.1:9876/health",
	}
}

// NewSilero создаёт описание сервиса Silero TTS.
// ttsDir — путь к директории tts-server (например, "./tts-server").
// pythonPath — путь к python из venv (например, "./tts-server/.venv/bin/python").
func NewSilero(pythonPath, ttsDir string) *Service {
	cmd := exec.Command(pythonPath, "silero_server.py")
	cmd.Dir = ttsDir // <-- ключевой момент: рабочий каталог = tts-server
	return &Service{
		Name:      "silero",
		Cmd:       cmd,
		HealthURL: "http://127.0.0.1:9877/health",
	}
}