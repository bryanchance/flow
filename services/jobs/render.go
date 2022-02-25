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
package render

import (
	"context"

	api "github.com/fynca/fynca/api/services/jobs/v1"
	"github.com/fynca/fynca/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
)

func (s *service) GetLatestRender(ctx context.Context, req *api.GetLatestRenderRequest) (*api.GetLatestRenderResponse, error) {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "GetLatestRender")
	defer span.End()

	url, err := s.ds.GetLatestRender(ctx, req.ID, req.Frame, req.TTL)
	if err != nil {
		return nil, err
	}

	return &api.GetLatestRenderResponse{
		Url:   url,
		Frame: req.Frame,
	}, nil
}
