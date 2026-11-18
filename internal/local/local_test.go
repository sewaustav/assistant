package local

import (
	"context"
	"testing"
)

type fakeTool struct {
	keywords []string
	args     map[string]string
}

func (f *fakeTool) Keywords() []string { return f.keywords }
func (f *fakeTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	f.args = args
	return "ok", nil
}
func (f *fakeTool) ParseArgs(text string) map[string]string {
	return map[string]string{"app": "телеграм"}
}

func TestHandleMatchesPhraseAndPassesArgs(t *testing.T) {
	tool := &fakeTool{keywords: []string{"открой", "запусти"}}
	h, err := New([]Tool{tool})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := h.Handle(context.Background(), "открой мне телеграм")
	if err != nil {
		t.Fatal(err)
	}
	if resp != "ok" {
		t.Fatalf("resp = %q", resp)
	}
	if tool.args["app"] != "телеграм" {
		t.Fatalf("args = %v", tool.args)
	}
}

func TestHandleUnknown(t *testing.T) {
	tool := &fakeTool{keywords: []string{"скопируй"}}
	h, err := New([]Tool{tool})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := h.Handle(context.Background(), "расскажи анекдот")
	if err != nil {
		t.Fatal(err)
	}
	if resp == "ok" {
		t.Fatal("неизвестная фраза не должна совпадать")
	}
}
