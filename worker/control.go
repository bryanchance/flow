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
package worker

import (
	api "github.com/fynca/fynca/api/services/workers/v1"
	"github.com/fynca/fynca/version"
	"github.com/gogo/protobuf/proto"
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
			r := api.ControlWorkerRequest{}
			if err := proto.Unmarshal(msg.Value(), &r); err != nil {
				logrus.WithError(err).Errorf("error unmarshaling object %s", msg.Key())
				continue
			}

			logrus.Debugf("processing control worker request from (%s): %+v", msg.Created(), r.Message)

			switch v := r.Message.(type) {
			case *api.ControlWorkerRequest_Pause:
				logrus.Infof("received pause control message from %s", r.Requestor)
				w.jobLock.Lock()
				w.paused = true
				w.jobLock.Unlock()
			case *api.ControlWorkerRequest_Resume:
				logrus.Infof("received resume control message from %s", r.Requestor)
				w.jobLock.Lock()
				w.paused = false
				w.jobLock.Unlock()
			case *api.ControlWorkerRequest_Stop:
				logrus.Infof("received stop control message from %s", r.Requestor)
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
			case *api.ControlWorkerRequest_Update:
				logrus.Infof("received update message by %s to update from %s", r.Requestor, v.Update.URL)
				if err := kv.Purge(msg.Key()); err != nil {
					logrus.WithError(err).Errorf("error deleting control message %s", msg.Key())
				}
				if err := w.update(v.Update.URL); err != nil {
					if err == ErrNoUpdateNeeded {
						logrus.Infof("worker is at the latest version %s", version.Version)
						continue
					}
					logrus.WithError(err).Error("error updating worker")
				}
				if err := w.Stop(); err != nil {
					logrus.WithError(err).Error("error stopping worker")
				}
			default:
				logrus.Warnf("unknown control message: %v", v)
			}

		}
	}
}
