package local

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/aaaton/golem/v4"
	"github.com/aaaton/golem/v4/dicts/ru"
)

type entry struct {
	tool     Tool
	lemmas   map[string]int
	keywords []string
}

type Handler struct {
	entries []entry
	lemm    *golem.Lemmatizer
}

// ArgParser — необязательный интерфейс: инструмент умеет вытаскивать
// аргументы из обычной фразы («открой телеграм» → app=телеграм).
type ArgParser interface {
	ParseArgs(text string) map[string]string
}

func New(tools []Tool) (*Handler, error) {
	lemm, err := golem.New(ru.New())
	if err != nil {
		return nil, fmt.Errorf("не удалось загрузить словарь: %w", err)
	}

	h := &Handler{lemm: lemm}
	for _, t := range tools {
		kw := t.Keywords()
		if len(kw) == 0 {
			continue
		}
		weights := make(map[string]int)
		for _, phrase := range kw {
			// Ключевые фразы могут состоять из нескольких слов.
			for _, word := range strings.Fields(phrase) {
				lemma := h.lemma(word)
				if lemma == "" {
					continue
				}
				weight := len([]rune(word))
				if weight < 2 {
					weight = 2
				}
				if weights[lemma] < weight {
					weights[lemma] = weight
				}
			}
		}
		if len(weights) == 0 {
			continue
		}
		h.entries = append(h.entries, entry{
			tool:     t,
			lemmas:   weights,
			keywords: kw,
		})
	}
	log.Printf("[local] зарегистрировано команд: %d", len(h.entries))
	return h, nil
}

func (h *Handler) Handle(ctx context.Context, text string) (string, error) {
	textLemmas := h.lemmatizeSet(strings.Fields(text))
	if len(textLemmas) == 0 {
		return "", fmt.Errorf("пустой текст")
	}

	best := -1
	bestScore := 0
	for i := range h.entries {
		e := &h.entries[i]
		score := 0
		matched := 0
		for lemma, weight := range e.lemmas {
			if _, ok := textLemmas[lemma]; ok {
				score += weight
				matched++
			}
		}
		if matched == 0 {
			continue
		}
		// Небольшой бонус за количество совпавших слов.
		score += matched * 2
		if score > bestScore {
			bestScore = score
			best = i
		}
	}

	if best < 0 || bestScore < 4 {
		return "Не понял команду. Скажи «помощь», чтобы увидеть список команд.", nil
	}

	e := &h.entries[best]
	args := map[string]string(nil)
	if p, ok := e.tool.(ArgParser); ok {
		args = p.ParseArgs(text)
	}

	log.Printf("[local] команда найдена, score=%d, keywords=%v, args=%v", bestScore, e.keywords, args)
	return e.tool.Execute(ctx, args)
}

// List возвращает описания команд для CLI.
func (h *Handler) List() []string {
	var out []string
	for _, e := range h.entries {
		out = append(out, strings.Join(e.keywords, ", "))
	}
	sort.Strings(out)
	return out
}

func (h *Handler) lemmatizeSet(words []string) map[string]struct{} {
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		lemma := h.lemma(w)
		if lemma == "" {
			continue
		}
		set[lemma] = struct{}{}
	}
	return set
}

func (h *Handler) lemma(word string) string {
	word = strings.ToLower(strings.Trim(word, ".,!?;:«»\"'()"))
	if word == "" {
		return ""
	}
	lemma := h.lemm.Lemma(word)
	if lemma == "" {
		lemma = word
	}
	return lemma
}
