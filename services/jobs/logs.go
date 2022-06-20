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

	api "github.com/ehazlett/flow/api/services/jobs/v1"
)

func (s *service) JobLog(ctx context.Context, r *api.JobLogRequest) (*api.JobLogResponse, error) {
	log, err := s.ds.GetJobLog(ctx, r.ID)
	if err != nil {
		return nil, err
	}

	return &api.JobLogResponse{
		JobLog: log,
	}, nil
}

func (s *service) RenderLog(ctx context.Context, r *api.RenderLogRequest) (*api.RenderLogResponse, error) {
	log, err := s.ds.GetRenderLog(ctx, r.ID, r.Frame, r.Slice)
	if err != nil {
		return nil, err
	}

	return &api.RenderLogResponse{
		RenderLog: log,
	}, nil
}
