package worker

import (
	"git.underland.io/ehazlett/fynca/version"
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
