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
package datastore

import (
	"context"
	"testing"
	"time"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/jobs/v1"
)

func getContext() context.Context {
	return context.WithValue(context.Background(), fynca.CtxNamespaceKey, "testing")
}

func TestJobsFramesQueued(t *testing.T) {
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 0,
		},
		FrameJobs: []*api.FrameJob{
			{
				Status: api.JobStatus_QUEUED,
			},
		},
	}

	ctx := getContext()

	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	expectedStatus := api.JobStatus_QUEUED

	if v := job.FrameJobs[0]; v.Status != expectedStatus {
		t.Fatalf("expected status %s; received %s", expectedStatus, v.Status)
	}

	if job.Status != expectedStatus {
		t.Fatalf("expected status %s; received %s", expectedStatus, job.Status)
	}
}

func TestJobsFramesRendering(t *testing.T) {
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 0,
		},
		FrameJobs: []*api.FrameJob{
			{
				Status: api.JobStatus_RENDERING,
			},
		},
	}

	ctx := getContext()
	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	expectedStatus := api.JobStatus_RENDERING

	if v := job.FrameJobs[0]; v.Status != expectedStatus {
		t.Fatalf("expected status %s; received %s", expectedStatus, v.Status)
	}

	if job.Status != expectedStatus {
		t.Fatalf("expected status %s; received %s", expectedStatus, job.Status)
	}
}

func TestJobsFramesDurationSingle(t *testing.T) {
	started := time.Now()
	finished := started.Add(time.Second * 5)
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 0,
		},
		FrameJobs: []*api.FrameJob{
			{
				Status:     api.JobStatus_RENDERING,
				StartedAt:  started,
				FinishedAt: finished,
			},
		},
	}

	ctx := getContext()

	expectedDuration := time.Second * 5
	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if job.Duration != expectedDuration {
		t.Fatalf("expected duration %s; received %s", expectedDuration, job.Duration)
	}
}

func TestJobsFramesDurationMultiple(t *testing.T) {
	started := time.Now()
	finished := started.Add(time.Second * 5)
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 0,
		},
		FrameJobs: []*api.FrameJob{
			{
				Status:     api.JobStatus_RENDERING,
				StartedAt:  started,
				FinishedAt: finished,
			},
			{
				Status:     api.JobStatus_RENDERING,
				StartedAt:  started,
				FinishedAt: finished,
			},
			{
				Status:     api.JobStatus_RENDERING,
				StartedAt:  started,
				FinishedAt: finished,
			},
		},
	}

	ctx := getContext()

	expectedDuration := time.Second * 15
	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if job.Duration != expectedDuration {
		t.Fatalf("expected duration %s; received %s", expectedDuration, job.Duration)
	}
}

func TestJobsFramesStartedFinished(t *testing.T) {
	started := time.Now()
	finished := started.Add(time.Second * 5)
	finishedLast := finished.Add(time.Second * 5)
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 0,
		},
		FrameJobs: []*api.FrameJob{
			{
				Status:     api.JobStatus_RENDERING,
				StartedAt:  started,
				FinishedAt: finished,
			},
			{
				Status:     api.JobStatus_RENDERING,
				StartedAt:  started,
				FinishedAt: finishedLast,
			},
			{
				Status:     api.JobStatus_RENDERING,
				StartedAt:  started,
				FinishedAt: finished,
			},
		},
	}

	ctx := getContext()

	expectedDuration := time.Second * 20
	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if job.Duration != expectedDuration {
		t.Fatalf("expected duration %s; received %s", expectedDuration, job.Duration)
	}

	if job.StartedAt != started {
		t.Fatalf("expected job start of %s; received %s", started, job.StartedAt)
	}

	if job.FinishedAt != finishedLast {
		t.Fatalf("expected job finish of %s; received %s", finishedLast, job.FinishedAt)
	}
}

func TestJobsSlicesQueued(t *testing.T) {
	s1 := &api.SliceJob{
		Status: api.JobStatus_QUEUED,
	}
	s2 := &api.SliceJob{
		Status: api.JobStatus_QUEUED,
	}
	sliceJobs := []*api.SliceJob{s1, s2}
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 2,
		},
		FrameJobs: []*api.FrameJob{
			{
				SliceJobs: sliceJobs,
			},
		},
	}

	ctx := getContext()

	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if v := job.FrameJobs[0]; v.Status != api.JobStatus_QUEUED {
		t.Fatalf("expected frame status %s; received %s", api.JobStatus_QUEUED, v.Status)
	}

	if job.Status != api.JobStatus_QUEUED {
		t.Fatalf("expected status %s; received %s", api.JobStatus_QUEUED, job.Status)
	}
}

func TestJobsSlicesQueuedAndRendering(t *testing.T) {
	s1 := &api.SliceJob{
		Status: api.JobStatus_QUEUED,
	}
	s2 := &api.SliceJob{
		Status: api.JobStatus_RENDERING,
	}
	sliceJobs := []*api.SliceJob{s1, s2}
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 2,
		},
		FrameJobs: []*api.FrameJob{
			{
				SliceJobs: sliceJobs,
			},
		},
	}

	ctx := getContext()

	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if v := job.FrameJobs[0]; v.Status != api.JobStatus_RENDERING {
		t.Fatalf("expected frame status %s; received %s", api.JobStatus_RENDERING, v.Status)
	}
	if job.Status != api.JobStatus_RENDERING {
		t.Fatalf("expected status %s; received %s", api.JobStatus_RENDERING, job.Status)
	}
}

func TestJobsSlicesRenderingAndQueued(t *testing.T) {
	s1 := &api.SliceJob{
		Status: api.JobStatus_RENDERING,
	}
	s2 := &api.SliceJob{
		Status: api.JobStatus_QUEUED,
	}
	sliceJobs := []*api.SliceJob{s1, s2}
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 2,
		},
		FrameJobs: []*api.FrameJob{
			{
				SliceJobs: sliceJobs,
			},
		},
	}

	ctx := getContext()

	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if v := job.FrameJobs[0]; v.Status != api.JobStatus_RENDERING {
		t.Fatalf("expected frame status %s; received %s", api.JobStatus_RENDERING, v.Status)
	}

	if job.Status != api.JobStatus_RENDERING {
		t.Fatalf("expected status %s; received %s", api.JobStatus_RENDERING, job.Status)
	}
}

func TestJobsSlicesQueuedAndError(t *testing.T) {
	s1 := &api.SliceJob{
		Status: api.JobStatus_QUEUED,
	}
	s2 := &api.SliceJob{
		Status: api.JobStatus_ERROR,
	}
	sliceJobs := []*api.SliceJob{s1, s2}
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 2,
		},
		FrameJobs: []*api.FrameJob{
			{
				SliceJobs: sliceJobs,
			},
		},
	}

	ctx := getContext()

	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if v := job.FrameJobs[0]; v.Status != api.JobStatus_ERROR {
		t.Fatalf("expected frame status %s; received %s", api.JobStatus_ERROR, v.Status)
	}

	if job.Status != api.JobStatus_ERROR {
		t.Fatalf("expected status %s; received %s", api.JobStatus_ERROR, job.Status)
	}
}

func TestJobsSlicesErrorAndQueued(t *testing.T) {
	s1 := &api.SliceJob{
		Status: api.JobStatus_ERROR,
	}
	s2 := &api.SliceJob{
		Status: api.JobStatus_QUEUED,
	}
	sliceJobs := []*api.SliceJob{s1, s2}
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 2,
		},
		FrameJobs: []*api.FrameJob{
			{
				SliceJobs: sliceJobs,
			},
		},
	}

	ctx := getContext()

	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if v := job.FrameJobs[0]; v.Status != api.JobStatus_ERROR {
		t.Fatalf("expected frame status %s; received %s", api.JobStatus_ERROR, v.Status)
	}

	if job.Status != api.JobStatus_ERROR {
		t.Fatalf("expected status %s; received %s", api.JobStatus_ERROR, job.Status)
	}
}

func TestJobsSlicesQueuedAndFinished(t *testing.T) {
	s1 := &api.SliceJob{
		Status: api.JobStatus_QUEUED,
	}
	s2 := &api.SliceJob{
		Status: api.JobStatus_FINISHED,
	}
	sliceJobs := []*api.SliceJob{s1, s2}
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 2,
		},
		FrameJobs: []*api.FrameJob{
			{
				SliceJobs: sliceJobs,
			},
		},
	}

	ctx := getContext()

	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if v := job.FrameJobs[0]; v.Status != api.JobStatus_RENDERING {
		t.Fatalf("expected frame status %s; received %s", api.JobStatus_RENDERING, v.Status)
	}

	if job.Status != api.JobStatus_RENDERING {
		t.Fatalf("expected status %s; received %s", api.JobStatus_RENDERING, job.Status)
	}
}

func TestJobsSlicesFinishedAndQueued(t *testing.T) {
	s1 := &api.SliceJob{
		Status: api.JobStatus_FINISHED,
	}
	s2 := &api.SliceJob{
		Status: api.JobStatus_QUEUED,
	}
	sliceJobs := []*api.SliceJob{s1, s2}
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 2,
		},
		FrameJobs: []*api.FrameJob{
			{
				SliceJobs: sliceJobs,
			},
		},
	}

	ctx := getContext()

	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if v := job.FrameJobs[0]; v.Status != api.JobStatus_RENDERING {
		t.Fatalf("expected frame status %s; received %s", api.JobStatus_RENDERING, v.Status)
	}

	if job.Status != api.JobStatus_RENDERING {
		t.Fatalf("expected status %s; received %s", api.JobStatus_RENDERING, job.Status)
	}
}

func TestJobsSlicesRendering(t *testing.T) {
	s1 := &api.SliceJob{
		Status: api.JobStatus_RENDERING,
	}
	s2 := &api.SliceJob{
		Status: api.JobStatus_RENDERING,
	}
	sliceJobs := []*api.SliceJob{s1, s2}
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 2,
		},
		FrameJobs: []*api.FrameJob{
			{
				SliceJobs: sliceJobs,
			},
		},
	}

	ctx := getContext()

	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if v := job.FrameJobs[0]; v.Status != api.JobStatus_RENDERING {
		t.Fatalf("expected frame status %s; received %s", api.JobStatus_RENDERING, v.Status)
	}

	if job.Status != api.JobStatus_RENDERING {
		t.Fatalf("expected status %s; received %s", api.JobStatus_RENDERING, job.Status)
	}
}

func TestJobsSlicesFinished(t *testing.T) {
	s1 := &api.SliceJob{
		Status: api.JobStatus_FINISHED,
	}
	s2 := &api.SliceJob{
		Status: api.JobStatus_FINISHED,
	}
	sliceJobs := []*api.SliceJob{s1, s2}
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 2,
		},
		FrameJobs: []*api.FrameJob{
			{
				SliceJobs: sliceJobs,
			},
		},
	}

	ctx := getContext()

	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if v := job.FrameJobs[0]; v.Status != api.JobStatus_FINISHED {
		t.Fatalf("expected frame status %s; received %s", api.JobStatus_FINISHED, v.Status)
	}

	if job.Status != api.JobStatus_FINISHED {
		t.Fatalf("expected status %s; received %s", api.JobStatus_FINISHED, job.Status)
	}
}

func TestJobsSlicesDuration(t *testing.T) {
	started := time.Now()
	finished := started.Add(time.Second * 5)
	s1 := &api.SliceJob{
		Status:     api.JobStatus_FINISHED,
		StartedAt:  started,
		FinishedAt: finished,
	}
	s2 := &api.SliceJob{
		Status:     api.JobStatus_FINISHED,
		StartedAt:  started,
		FinishedAt: finished,
	}
	sliceJobs := []*api.SliceJob{s1, s2}
	job := &api.Job{
		Request: &api.JobRequest{
			RenderSlices: 2,
		},
		FrameJobs: []*api.FrameJob{
			{
				SliceJobs: sliceJobs,
			},
		},
	}

	ctx := getContext()

	expectedDuration := time.Second * 10
	if err := resolveJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	if job.Duration != expectedDuration {
		t.Fatalf("expected duration %s; received %s", expectedDuration, job.Duration)
	}
}
