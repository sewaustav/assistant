package local

import "context"

type Tool interface {
	Keywords() []string
	Execute(ctx context.Context, args map[string]string) (string, error)
}