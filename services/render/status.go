package render

import (
	"bytes"
	"context"
	"time"

	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"github.com/gogo/protobuf/jsonpb"
	nats "github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

func (s *service) jobStatusListener() {
	js, err := s.natsClient.JetStream()
	if err != nil {
		logrus.Fatal(err)
	}

	sub, err := js.PullSubscribe(s.config.NATSJobStatusSubject, finca.ServerQueueGroupName)
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
			switch v := jobResult.Result.(type) {
			case *api.JobResult_FrameJob:
				j := v.FrameJob
				jobID = j.ID
				logrus.Infof("frame %d for job %s complete", j.RenderFrame, j.ID)
			case *api.JobResult_SliceJob:
				j := v.SliceJob
				jobID = j.ID
				logrus.Infof("slice %d (frame %d) for job %s complete", j.RenderSliceIndex, j.RenderFrame, j.ID)
			default:
				logrus.Errorf("unknown job result type %+T", v)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			logrus.Debugf("job complete for frame %d; resolving final job", jobResult.RenderFrame)
			if err := s.ds.ResolveJob(ctx, jobID); err != nil {
				cancel()
				logrus.WithError(err).Errorf("error updating final job for %s", jobID)
				continue
			}
			cancel()
			// ack message to finish
			m.Ack()
		}
	}
}
