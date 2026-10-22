package local

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aaaton/golem/v4"
	"github.com/aaaton/golem/v4/dicts/ru"
)

type entry struct {
	tool     Tool
	lemmas   map[string]struct{}
	keywords []string
}

type Handler struct {
	entries []entry
	lemm    *golem.Lemmatizer
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
		h.entries = append(h.entries, entry{
			tool:     t,
			lemmas:   h.lemmatizeSet(kw),
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

	var best *entry
	bestScore := 0

	for i := range h.entries {
		e := &h.entries[i]
		score := 0
		for l := range e.lemmas {
			if _, ok := textLemmas[l]; ok {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = e
		}
	}

	if best == nil || bestScore == 0 {
		return "Не понял команду", nil
	}

	log.Printf("[local] команда найдена, score=%d, keywords=%v", bestScore, best.keywords)
	return best.tool.Execute(ctx, nil)
}

func (h *Handler) lemmatizeSet(words []string) map[string]struct{} {
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,!?;:"))
		if w == "" {
			continue
		}
		lemma := h.lemm.Lemma(w)
		if lemma == "" {
			lemma = w
		}
		set[lemma] = struct{}{}
	}
	return set
}