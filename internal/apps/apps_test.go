package apps

import "testing"

func TestNormalizeAndStem(t *testing.T) {
	cases := map[string]string{
		"Музыку":           "музык",
		"Музыка":           "музык",
		"Настройки":        "настройк",
		"Telegram Desktop": "telegramdesktop",
		"Google Chrome":    "googlechrome",
	}
	for input, want := range cases {
		if got := normalize(input); got != want {
			t.Errorf("normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTranslit(t *testing.T) {
	if got := translit("телеграм"); got != "telegram" {
		t.Fatalf("translit = %q", got)
	}
	if got := translit("калькулятор"); got != "kalkulyator" {
		t.Fatalf("translit = %q", got)
	}
}

func TestMatchScoreRussianQuery(t *testing.T) {
	app := App{Name: "Telegram Desktop", Aliases: []string{"телеграм"}}
	score := matchScore(normalize("телеграм"), translit(normalize("телеграм")), app)
	if score < 3 {
		t.Fatalf("ожидалось совпадение, score=%d", score)
	}
}
