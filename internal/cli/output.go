package cli

import "fmt"

// clearLine стирает текущую строку и возвращает курсор в начало.
// \033[2K — очистить всю строку, \r — вернуться в начало.
const clearLine = "\033[2K\r"

// PrintResponse выводит ответ агента.
func PrintResponse(text string) {
	fmt.Printf("%s[assistant] %s\n", clearLine, text)
}

// PrintUser выводит то, что сказал/написал пользователь.
// В голосовом режиме — это распознанный текст, в текстовом — введённый.
func PrintUser(text string) {
	fmt.Printf("%s[you] %s\n", clearLine, text)
}

// PrintInfo выводит служебное сообщение для пользователя.
func PrintInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[info] %s\n", clearLine, msg)
}

// PrintError выводит ошибку для пользователя.
func PrintError(err error) {
	fmt.Printf("%s[error] %v\n", clearLine, err)
}