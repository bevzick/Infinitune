package correlation

import (
	"context"

	"github.com/google/uuid"
)

const (
	MetadataKey  = "x-request-id"
	maximumIDLen = 128
)

type contextKey struct{}

func WithID(ctx context.Context, id string) context.Context {
	if !Valid(id) {
		id = uuid.NewString()
	}
	return context.WithValue(ctx, contextKey{}, id)
}

func ID(ctx context.Context) string {
	value, _ := ctx.Value(contextKey{}).(string)
	return value
}

func Normalize(value string) string {
	if Valid(value) {
		return value
	}
	return uuid.NewString()
}

func Valid(value string) bool {
	if value == "" || len(value) > maximumIDLen {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' ||
			character == '.' || character == ':' {
			continue
		}
		return false
	}
	return true
}
