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

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/jobs/v1"
	"github.com/sirupsen/logrus"
)

func (s *service) GetJobArchive(ctx context.Context, r *api.GetJobArchiveRequest) (*api.GetJobArchiveResponse, error) {
	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)
	// check if archive request is in db
	jobArchive, err := s.ds.GetJobArchiveStatus(ctx, r.ID)
	if err != nil {
		return nil, err
	}

	// if not in db, issue a ds.CreateJobArchive and store the job ID in the db (/archivejobs/<job.ID>)
	if jobArchive != nil {
		return &api.GetJobArchiveResponse{
			JobArchive: jobArchive,
		}, nil
	}

	// signal archive creation
	s.jobArchiveCh <- &jobArchiveRequest{Namespace: namespace, ID: r.ID}

	jobArchive = &api.JobArchive{}

	// return archive request either new or from existing db.  if in db and done there will be an archive url
	return &api.GetJobArchiveResponse{
		JobArchive: jobArchive,
	}, nil
}

func (s *service) jobArchiveListener() {
	for {
		select {
		case <-s.stopCh:
			return
		case req := <-s.jobArchiveCh:
			logrus.Debugf("creating job archive for %s", req.ID)
			ctx := context.WithValue(context.Background(), fynca.CtxNamespaceKey, req.Namespace)
			go func() {
				if err := s.ds.CreateJobArchive(ctx, req.ID); err != nil {
					logrus.WithError(err).Errorf("error creating archive for job %s", req.ID)
					return
				}
			}()
		}
	}

}
