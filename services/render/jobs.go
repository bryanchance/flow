package render

import (
	"context"
	"strings"

	api "git.underland.io/ehazlett/fynca/api/services/render/v1"
	"git.underland.io/ehazlett/fynca/datastore"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (s *service) ListJobs(ctx context.Context, r *api.ListJobsRequest) (*api.ListJobsResponse, error) {
	jobOpts := []datastore.JobOpt{}
	if r.ExcludeFrames {
		jobOpts = append(jobOpts, datastore.WithExcludeJobFrames)
	}
	if r.ExcludeSlices {
		jobOpts = append(jobOpts, datastore.WithExcludeJobSlices)
	}

	jobs, err := s.ds.GetJobs(ctx, jobOpts...)
	if err != nil {
		return nil, err
	}

	return &api.ListJobsResponse{
		Jobs: jobs,
	}, nil
}

func (s *service) GetJob(ctx context.Context, r *api.GetJobRequest) (*api.GetJobResponse, error) {
	job, err := s.ds.GetJob(ctx, r.ID)
	if err != nil {
		return nil, err
	}

	return &api.GetJobResponse{
		Job: job,
	}, nil
}

func (s *service) DeleteJob(ctx context.Context, r *api.DeleteJobRequest) (*ptypes.Empty, error) {
	job, err := s.ds.GetJob(ctx, r.ID)
	if err != nil {
		return empty, errors.Wrapf(err, "error getting job %s from datastore", r.ID)
	}

	js, err := s.natsClient.JetStream()
	if err != nil {
		return empty, err
	}

	// check nats for the job and delete
	for _, frameJob := range job.FrameJobs {
		if frameJob.SequenceID != 0 {
			if err := js.DeleteMsg(s.config.NATSJobStreamName, frameJob.SequenceID); err != nil {
				// ignore missing
				if !strings.Contains(err.Error(), "no message found") {
					return empty, err
				}
			}
		}
		for _, sliceJob := range frameJob.SliceJobs {
			if sliceJob.SequenceID != 0 {
				if err := js.DeleteMsg(s.config.NATSJobStreamName, sliceJob.SequenceID); err != nil {
					// ignore missing
					if !strings.Contains(err.Error(), "no message found") {
						return empty, err
					}
				}
			}
		}
	}

	// delete from datastore
	if err := s.ds.DeleteJob(ctx, r.ID); err != nil {
		return empty, errors.Wrapf(err, "error deleting job %s from datastore", r.ID)
	}

	logrus.Infof("deleted job %s", r.ID)
	return empty, nil
}
