package cli

import (
	"fmt"
)

// PrintResponse выводит ответ агента.
func PrintResponse(text string) {
	// \r — чтобы перезаписать строку с приглашением, если оно было.
	fmt.Printf("\r[assistant] %s\n", text)
}

// PrintError выводит ошибку.
func PrintError(err error) {
	fmt.Printf("\r[error] %v\n", err)
}

// PrintInfo выводит служебное сообщение.
func PrintInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("\r[info] %s\n", msg)
}

// PrintEcho выводит то, что пользователь сказал/написал (для голосового режима).
func PrintEcho(text string) {
	fmt.Printf("\r[you] %s\n", text)
}