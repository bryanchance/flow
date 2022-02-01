package worker

import (
	"bytes"

	api "git.underland.io/ehazlett/fynca/api/services/render/v1"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

func (w *Worker) workerControlListener() {
	nc, err := nats.Connect(w.config.NATSURL)
	if err != nil {
		logrus.Fatalf("error connecting to nats on %s", w.config.NATSURL)
	}

	js, err := nc.JetStream()
	if err != nil {
		logrus.Fatal("error getting stream context")
	}

	kv, err := js.KeyValue(w.config.NATSKVBucketWorkerControl)
	if err != nil {
		logrus.Fatalf("error getting kv %s from nats", w.config.NATSKVBucketWorkerControl)
	}

	kw, err := kv.Watch(w.id, nats.IgnoreDeletes())
	if err != nil {
		logrus.Fatalf("error watching kv for updates: %s", err)
	}

	kvCh := kw.Updates()

	for msg := range kvCh {
		if msg == nil {
			continue
		}
		if msg.Key() == w.id {
			r := &api.ControlWorkerRequest{}
			buf := bytes.NewBuffer(msg.Value())
			if err := jsonpb.Unmarshal(buf, r); err != nil {
				logrus.WithError(err).Errorf("error unmarshaling object %s", msg.Key())
				continue
			}

			logrus.Debugf("processing control worker request from (%s): %+v", msg.Created(), r.Message)

			switch v := r.Message.(type) {
			case *api.ControlWorkerRequest_Stop:
				logrus.Info("received stop control message")
				if err := kv.Purge(msg.Key()); err != nil {
					logrus.WithError(err).Errorf("error deleting control message %s", msg.Key())
				}
				if err := kw.Stop(); err != nil {
					logrus.WithError(err).Error("error stopping kv watcher")
				}
				if err := w.Stop(); err != nil {
					logrus.WithError(err).Error("error stopping worker")
					return
				}
			default:
				logrus.Warnf("unknown control message: %v", v)
			}

		}
	}
}
