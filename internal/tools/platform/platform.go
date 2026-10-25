package platform

var (
	OpenBrowser func(url string) error
	SendHotkey  func(keys string) error
)