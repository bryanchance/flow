// Copyright 2022 Evan Hazlett
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package fynca

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
