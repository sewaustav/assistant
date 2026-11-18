package lifecycle

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Service описывает управляемый дочерний процесс.
// Cmd — фабрика: каждый вызов Start() создаёт новый *exec.Cmd.
type Service struct {
	Name         string
	CmdFactory   func() *exec.Cmd
	ReadyURL     string
	HealthURL    string
	ReadyTimeout time.Duration
	Env          []string

	mu      sync.Mutex
	cmd     *exec.Cmd     // текущий запущенный процесс (nil если не запущен)
	done    chan struct{} // закрывается, когда процесс завершился
	started bool
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

// Registered сообщает, зарегистрирован ли сервис.
func (m *Manager) Registered(name string) bool {
	_, ok := m.services[name]
	return ok
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

	// Каждый раз создаём новый Cmd через фабрику.
	cmd := svc.CmdFactory()
	if len(svc.Env) > 0 {
		cmd.Env = append(os.Environ(), svc.Env...)
	}
	configureProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить %s: %w", name, err)
	}

	svc.cmd = cmd
	svc.done = make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(svc.done)
	}()

	if err := waitReady(ctx, svc); err != nil {
		killProcessTree(cmd, svc.done)
		svc.cmd = nil
		svc.done = nil
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

	if !svc.started || svc.cmd == nil {
		log.Printf("[lifecycle] %s не запущен", name)
		return nil
	}

	log.Printf("[lifecycle] Остановка %s...", name)
	killProcessTree(svc.cmd, svc.done)
	svc.cmd = nil
	svc.done = nil
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
	timeout := svc.ReadyTimeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-svc.done:
			return fmt.Errorf("процесс завершился до готовности")
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
func NewGigaSTT(binPath string, extraArgs []string) *Service {
	args := append([]string{"serve"}, extraArgs...)
	return &Service{
		Name: "gigastt",
		CmdFactory: func() *exec.Cmd {
			return exec.Command(binPath, args...)
		},
		ReadyURL:     "http://127.0.0.1:9876/ready",
		HealthURL:    "http://127.0.0.1:9876/health",
		ReadyTimeout: 10 * time.Minute,
	}
}

// NewSilero создаёт описание сервиса Silero TTS.
func NewSilero(pythonPath, ttsDir string) *Service {
	return &Service{
		Name: "silero",
		CmdFactory: func() *exec.Cmd {
			cmd := exec.Command(pythonPath, "silero_server.py")
			cmd.Dir = ttsDir
			return cmd
		},
		HealthURL:    "http://127.0.0.1:9877/health",
		ReadyTimeout: 10 * time.Minute,
		Env:          []string{"PYTHONUNBUFFERED=1"},
	}
}
