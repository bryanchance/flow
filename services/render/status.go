package render

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/render/v1"
	"github.com/gogo/protobuf/jsonpb"
	nats "github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

var (
	ackWaitJobStatus = time.Second * 30
)

func (s *service) jobStatusListener() {
	js, err := s.natsClient.JetStream()
	if err != nil {
		logrus.Fatal(err)
	}

	sub, err := js.PullSubscribe(s.config.NATSJobStatusSubject, fynca.ServerQueueGroupName, nats.AckWait(ackWaitJobStatus))
	if err != nil {
		logrus.Fatal(err)
	}

	s.serverQueueSubscription = sub

	msgCh := make(chan *nats.Msg)

	go func() {
		for {
			msgs, err := sub.Fetch(1)
			if err != nil {
				// skip invalid sub errors when exiting
				if !sub.IsValid() {
					return
				}
				if err == nats.ErrTimeout {
					// ignore NextMsg timeouts
					continue
				}
				logrus.WithError(err).Warn("error getting message")
				continue
			}

			msgCh <- msgs[0]
		}
	}()

	for {
		select {
		case <-s.stopCh:
			return
		case m := <-msgCh:
			jobResult := &api.JobResult{}
			buf := bytes.NewBuffer(m.Data)
			if err := jsonpb.Unmarshal(buf, jobResult); err != nil {
				logrus.WithError(err).Error("error unmarshaling api.JobResult from message")
				continue
			}
			jobID := ""
			frame := int64(0)
			slice := int64(-1)
			switch v := jobResult.Result.(type) {
			case *api.JobResult_FrameJob:
				j := v.FrameJob
				jobID = j.ID
				frame = j.RenderFrame
				if err := s.ds.ClearLatestRenderCache(context.Background(), j.ID, j.RenderFrame); err != nil {
					logrus.WithError(err).Warnf("error clearing render cache for job %s (frame %d)", j.ID, j.RenderFrame)
				}
				logrus.Infof("frame %d for job %s complete", j.RenderFrame, j.ID)
			case *api.JobResult_SliceJob:
				j := v.SliceJob
				jobID = j.ID
				frame = j.RenderFrame
				slice = j.RenderSliceIndex
				// clear render cache
				if err := s.ds.ClearLatestRenderCache(context.Background(), j.ID, j.RenderFrame); err != nil {
					logrus.WithError(err).Warnf("error clearing render cache for job %s (frame %d)", j.ID, j.RenderFrame)
				}
				logrus.Infof("slice %d (frame %d) for job %s complete", j.RenderSliceIndex, j.RenderFrame, j.ID)
			default:
				logrus.Errorf("unknown job result type %+T", v)
				continue
			}

			logMessage := fmt.Sprintf("Job %s completed successfully", jobID)
			if jobResult.Status == api.JobStatus_ERROR {
				logMessage = jobResult.Error
			}

			logrus.Debugf("job status update setting namespace: %s", jobResult.Namespace)
			uCtx := context.WithValue(context.Background(), fynca.CtxNamespaceKey, jobResult.Namespace)
			ctx, cancel := context.WithTimeout(uCtx, time.Second*10)
			// upload result to minio
			if err := s.ds.UpdateJobLog(ctx, &api.JobLog{
				ID:        jobID,
				Namespace: jobResult.Namespace,
				Log:       logMessage,
			}); err != nil {
				cancel()
				logrus.WithError(err).Errorf("error updating job log for job %s frame %d (slice %d)", jobID, frame, slice)
			}

			logrus.Debugf("job complete for frame %d; resolving final job", jobResult.RenderFrame)
			if err := s.ds.ResolveJob(ctx, jobID); err != nil {
				cancel()
				logrus.WithError(err).Errorf("error updating final job for %s", jobID)
				continue
			}
			cancel()

			logrus.Info(logMessage)
			// ack message to finish
			m.Ack()
		}
	}
}
