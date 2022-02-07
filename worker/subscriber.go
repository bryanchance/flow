package worker

import (
	"fmt"
	"sort"
	"time"

	"git.underland.io/ehazlett/fynca"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// subscriber manages subscribing to multiple subjects
type subscriber struct {
	config     *fynca.Config
	natsClient *nats.Conn
	subs       map[string]*nats.Subscription
}

func newSubscriber(cfg *fynca.Config) (*subscriber, error) {
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, errors.Wrap(err, "error connecting to nats")
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, errors.Wrap(err, "error getting jetstream context")
	}

	subs := map[string]*nats.Subscription{}

	for x, subject := range []string{
		fynca.QueueSubjectJobPriorityUrgent,
		fynca.QueueSubjectJobPriorityNormal,
		fynca.QueueSubjectJobPriorityAnimation,
		fynca.QueueSubjectJobPriorityLow,
	} {
		logrus.Debugf("subscriber: subscribing to subject %s", subject)
		sub, err := js.PullSubscribe(subject, fmt.Sprintf("%s-%s", fynca.WorkerQueueGroupName, subject), nats.AckWait(cfg.GetJobTimeout()))
		if err != nil {
			return nil, errors.Wrapf(err, "error subscribing to nats subject %s", subject)
		}
		subs[fmt.Sprintf("%d-%s", x, subject)] = sub
	}

	return &subscriber{
		config:     cfg,
		natsClient: nc,
		subs:       subs,
	}, nil
}

// nextMessage returns the next message based on the queue priority order
func (s *subscriber) nextMessage() (*nats.Msg, error) {
	// sort by order
	keys := make([]string, 0)
	for k, _ := range s.subs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, subject := range keys {
		sub := s.subs[subject]
		msgs, err := sub.Fetch(1, nats.MaxWait(1*time.Second))
		if err != nil {
			// if stop has been called the subscription will be drained and closed
			// ignore the subscription error and exit
			if !sub.IsValid() {
				continue
			}
			if err == nats.ErrTimeout {
				// ignore NextMsg timeouts
				continue
			}
			return nil, err
		}
		logrus.Debugf("received job on subject %s", subject)
		m := msgs[0]
		return m, nil
	}
	return nil, nil
}

func (s *subscriber) stop() {
	for _, sub := range s.subs {
		sub.Unsubscribe()
		sub.Drain()
	}
}
