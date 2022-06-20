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
	"github.com/ehazlett/flow/version"
	"github.com/pkg/errors"
	"github.com/sanbornm/go-selfupdate/selfupdate"
	"github.com/sirupsen/logrus"
)

var (
	// ErrNoUpdateNeeded is returned when the worker is at the latest version
	ErrNoUpdateNeeded = errors.New("worker is at the latest version")
)

func (w *Worker) update(url string) error {
	w.jobLock.Lock()
	defer w.jobLock.Unlock()

	updater := &selfupdate.Updater{
		CurrentVersion: version.Version,
		ApiURL:         url,
		BinURL:         url,
		DiffURL:        url,
		Dir:            "/",
		CmdName:        version.Name + "-worker",
	}

	newVersion, err := updater.UpdateAvailable()
	if err != nil {
		return err
	}

	if newVersion == "" {
		return ErrNoUpdateNeeded
	}

	if err := updater.Update(); err != nil {
		return err
	}

	logrus.Infof("successfully updated to %s", newVersion)

	return nil
}
