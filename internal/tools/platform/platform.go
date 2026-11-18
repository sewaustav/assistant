// Package platform выполняет действия, зависящие от ОС: открыть ссылку,
// нажать горячую клавишу, заблокировать экран и т.п.
//
// Каждая ОС регистрирует свои реализации в init(). Обёртки ниже страхуют от
// nil-функций и возвращают понятную ошибку вместо паники.
package platform

import (
	"errors"
	"os"
)

// ErrUnsupported возвращается, если действие не реализовано для ОС.
var ErrUnsupported = errors.New("действие не поддерживается на этой платформе")

// PathExists проверяет, существует ли файл.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var (
	openBrowser func(url string) error
	openPath    func(path string) error
	sendHotkey  func(keys string) error
	typeText    func(text string) error
	lockScreen  func() error
	screenshot  func() (string, error)
)

// OpenBrowser открывает URL в браузере по умолчанию.
func OpenBrowser(url string) error {
	if openBrowser == nil {
		return ErrUnsupported
	}
	return openBrowser(url)
}

// OpenPath открывает файл или папку средствами ОС.
func OpenPath(path string) error {
	if openPath == nil {
		return ErrUnsupported
	}
	return openPath(path)
}

// SendHotkey нажимает комбинацию в активном окне.
// Формат: "ctrl+shift+t", "alt+F4", "Delete", "volumeup".
// На macOS ctrl автоматически становится Command.
func SendHotkey(keys string) error {
	if sendHotkey == nil {
		return ErrUnsupported
	}
	return sendHotkey(keys)
}

// TypeText печатает текст в активное поле ввода.
func TypeText(text string) error {
	if typeText == nil {
		return ErrUnsupported
	}
	return typeText(text)
}

// LockScreen блокирует экран.
func LockScreen() error {
	if lockScreen == nil {
		return ErrUnsupported
	}
	return lockScreen()
}

// Screenshot делает снимок экрана и возвращает путь к файлу.
func Screenshot() (string, error) {
	if screenshot == nil {
		return "", ErrUnsupported
	}
	return screenshot()
}
