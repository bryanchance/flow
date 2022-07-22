package flow

import (
	"context"
	"fmt"
)

// GetNamespaceFromContext returns the namespace from the specified context
func GetNamespaceFromContext(ctx context.Context) (string, error) {
	namespace := ctx.Value(CtxNamespaceKey)
	if namespace == nil {
		return "", fmt.Errorf("unable to get namespace from context")
	}

	return namespace.(string), nil
}

// GetTokenFromContext returns the first auth token from the context
func GetTokenFromContext(ctx context.Context) (string, error) {
	t := ctx.Value(CtxTokenKey)
	if t == nil {
		return "", fmt.Errorf("unable to get token from context")
	}

	tokens := t.([]string)
	return tokens[0], nil
}
