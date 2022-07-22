package processor

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type ffmpegConfig struct {
	ResolutionX    int
	ResolutionY    int
	FrameRate      string
	Codec          string
	Quality        int
	OutputFilename string
}

func (p *Processor) encodeWorkflow(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	logrus.Debugf("encoding workflow %s", cfg.Workflow.ID)
	w := cfg.Workflow
	output := &workflows.ProcessorOutput{
		Parameters: map[string]string{},
	}

	// setup output dir
	outputDir, err := os.MkdirTemp("", fmt.Sprintf("flow-output-%s", w.ID))
	if err != nil {
		return nil, err
	}
	// set output dir for upload
	output.OutputDir = outputDir

	startedAt := time.Now()

	ffmpegCfg, err := parseParameters(w)
	if err != nil {
		return nil, err
	}

	outputFilename := filepath.Join(outputDir, ffmpegCfg.OutputFilename)
	logrus.Debugf("encoding filename: %s", outputFilename)

	// workflow output info
	output.Parameters["framerate"] = ffmpegCfg.FrameRate
	output.Parameters["quality"] = fmt.Sprintf("%d", ffmpegCfg.Quality)
	output.Parameters["x"] = fmt.Sprintf("%d", ffmpegCfg.ResolutionX)
	output.Parameters["y"] = fmt.Sprintf("%d", ffmpegCfg.ResolutionY)
	output.Parameters["filename"] = ffmpegCfg.OutputFilename

	workflowDir := cfg.InputDir

	archiveFiles, err := filepath.Glob(filepath.Join(workflowDir, "*.zip"))
	if err != nil {
		return nil, err
	}
	if len(archiveFiles) > 0 {
		for _, af := range archiveFiles {
			if err := p.extractArchive(ctx, af, workflowDir); err != nil {
				return nil, err
			}
		}
	}

	logrus.Debugf("using ffmpeg %s", p.ffmpegPath)

	// ffmpeg -r 30 -i %04d.jpg -vcodec libx264 -crf 25 -x264-params keyint=30:scenecut=0 -preset veryslow video.mp4
	args := []string{
		"-r",
		ffmpegCfg.FrameRate,
		"-pattern_type",
		"glob",
		"-i",
		"*.png",
		"-vcodec",
		ffmpegCfg.Codec,
		"-crf",
		fmt.Sprintf("%d", ffmpegCfg.Quality),
		"-x264-params",
		fmt.Sprintf("keyint=%s:scenecut=0", ffmpegCfg.FrameRate),
		outputFilename,
	}

	logrus.Debugf("ffmpeg args: %v", args)
	c := exec.CommandContext(ctx, p.ffmpegPath, args...)
	c.Dir = workflowDir

	out, err := c.CombinedOutput()
	if err != nil {
		// copy log to tmp
		tmpLogPath := filepath.Join(outputDir, "error.log")
		if err := os.WriteFile(tmpLogPath, out, 0644); err != nil {
			logrus.WithError(err).Errorf("unable to save error log file for workflow %s", w.ID)
		}
		logrus.WithError(err).Errorf("error log available at %s", tmpLogPath)
		return nil, errors.Wrap(err, string(out))
	}
	logrus.Debugf("out: %s", out)
	output.FinishedAt = time.Now()
	output.Duration = output.FinishedAt.Sub(startedAt)
	output.Log = string(out)

	logrus.Debugf("workflow complete: %+v", output)

	return output, nil
}

func (p *Processor) extractArchive(ctx context.Context, archivePath string, outputDir string) error {
	af, err := os.Open(archivePath)
	if err != nil {
		return err
	}

	logrus.Debugf("checking content type for %s", archivePath)
	contentType, err := flow.GetContentType(af)
	if err != nil {
		return errors.Wrapf(err, "error getting content type for %s", archivePath)
	}
	switch contentType {
	case "application/zip":
		logrus.Debug("zip archive detected; extracting")
		a, err := zip.OpenReader(archivePath)
		if err != nil {
			return errors.Wrapf(err, "error reading zip archive %s", archivePath)
		}
		defer a.Close()

		for _, f := range a.File {
			fp := filepath.Join(outputDir, f.Name)
			if !strings.HasPrefix(fp, filepath.Clean(outputDir)+string(os.PathSeparator)) {
				logrus.Warnf("invalid path for archive file %s", f.Name)
				continue
			}
			if f.FileInfo().IsDir() {
				if err := os.MkdirAll(fp, 0750); err != nil {
					return errors.Wrapf(err, "error creating dir from archive for %s", f.Name)
				}
				continue
			}

			if err := os.MkdirAll(filepath.Dir(fp), 0750); err != nil {
				return errors.Wrapf(err, "error creating parent dir from archive for %s", f.Name)
			}

			pf, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return errors.Wrapf(err, "error creating file from archive for %s", f.Name)
			}

			fa, err := f.Open()
			if err != nil {
				return errors.Wrapf(err, "error opening file in archive: %s", f.Name)
			}

			if _, err := io.Copy(pf, fa); err != nil {
				return errors.Wrapf(err, "error extracting file from archive: %s", f.Name)
			}
			pf.Close()
			fa.Close()
		}
	}

	return nil
}

func parseParameters(w *api.Workflow) (*ffmpegConfig, error) {
	cfg := &ffmpegConfig{
		ResolutionX:    1920,
		ResolutionY:    1080,
		FrameRate:      "24",
		Codec:          "libx264",
		Quality:        23,
		OutputFilename: "video.mp4",
	}

	for k, v := range w.Parameters {
		switch strings.ToLower(k) {
		case "x":
			x, err := strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
			cfg.ResolutionX = x
		case "y":
			y, err := strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
			cfg.ResolutionY = y
		case "framerate":
			cfg.FrameRate = v
		case "codec":
			cfg.Codec = v
		case "quality":
			y, err := strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
			cfg.Quality = y
		case "filename":
			cfg.OutputFilename = v
		default:
			return nil, fmt.Errorf("unknown parameter specified: %s", k)
		}
	}

	return cfg, nil
}
