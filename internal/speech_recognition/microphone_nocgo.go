//go:build !windows && !cgo

package speech_recognition

import "fmt"

// Без CGO на Unix-системах нет доступа к miniaudio, поэтому сборка остаётся
// рабочей, но голосовой режим честно сообщает, чего не хватает.
func newCaptureBackend(func([]byte)) (captureBackend, error) {
	return nil, fmt.Errorf("захват микрофона недоступен: сборка без CGO. Установи gcc и пересобери с CGO_ENABLED=1")
}
