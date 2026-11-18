package tools

import "testing"

func TestFindSite(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"ютуб", "https://youtube.com"},
		{"Открой ютуб", "https://youtube.com"},
		{"карты", "https://yandex.ru/maps"},
		{"Погода", "https://yandex.ru/pogoda"},
		{"телеграм", "https://web.telegram.org"},
		{"неизвестное приложение", ""},
	}

	for _, tc := range cases {
		got, ok := findSite(tc.text)
		if tc.want == "" {
			if ok {
				t.Errorf("findSite(%q) = %q, ожидалось, что сайт не найден", tc.text, got)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Errorf("findSite(%q) = %q, %v; ожидалось %q", tc.text, got, ok, tc.want)
		}
	}
}
