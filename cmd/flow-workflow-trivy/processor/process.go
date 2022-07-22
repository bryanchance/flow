package processor

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type trivyConfig struct {
	Action string
	Target string
}

func (p *Processor) processWorkflow(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	logrus.Debugf("processing workflow %s", cfg.Workflow.ID)
	w := cfg.Workflow
	output := &workflows.ProcessorOutput{
		Parameters: map[string]string{},
	}
	workflowDir := cfg.InputDir

	// setup output dir
	outputDir, err := os.MkdirTemp("", fmt.Sprintf("flow-output-%s", w.ID))
	if err != nil {
		return nil, err
	}
	// set output dir for upload
	output.OutputDir = outputDir

	startedAt := time.Now()

	trivyCfg, err := parseParameters(w)
	if err != nil {
		return nil, err
	}

	outputFilename := filepath.Join(outputDir, "trivy.log")

	if trivyCfg.Action == "" {
		return nil, fmt.Errorf("action must be specified")
	}

	if trivyCfg.Target == "" {
		logrus.Infof("target not specified; using workflow input dir")
		trivyCfg.Target = cfg.InputDir
		// check for archive (zip, tar.gz, etc) and extract
		if err := p.extractArchives(ctx, cfg.InputDir); err != nil {
			return nil, err
		}
	}

	// workflow output info
	output.Parameters["action"] = trivyCfg.Action
	output.Parameters["target"] = trivyCfg.Target

	args := []string{
		trivyCfg.Action,
		"--no-progress",
		fmt.Sprintf("--output=%s", outputFilename),
		trivyCfg.Target,
	}

	logrus.Debugf("trivy args: %v", args)
	c := exec.CommandContext(ctx, p.trivyPath, args...)
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

func (p *Processor) extractArchives(ctx context.Context, inputDir string) error {
	// TODO
	tarballArchives, err := filepath.Glob(filepath.Join(inputDir, "*.tar.gz"))
	if err != nil {
		return err
	}

	for _, tb := range tarballArchives {
		f, err := os.Open(tb)
		if err != nil {
			return err
		}
		logrus.Debugf("extracting tar.gz: %s to %s", tb, inputDir)
		if err := p.extractTarGz(ctx, f, inputDir); err != nil {
			return err
		}
	}

	zipArchives, err := filepath.Glob(filepath.Join(inputDir, "*.zip"))
	if err != nil {
		return err
	}

	for _, z := range zipArchives {
		logrus.Debugf("extracting zip: %s to %s", z, inputDir)
		if err := p.extractZip(ctx, z, inputDir); err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) extractTarGz(ctx context.Context, r io.Reader, dest string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if header == nil {
			continue
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			f.Close()
		}
	}

	return nil
}

func (p *Processor) extractZip(ctx context.Context, archivePath, dest string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		fp := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fp, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fp), os.ModePerm); err != nil {
			return err
		}

		df, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		af, err := f.Open()
		if err != nil {
			return err
		}

		if _, err := io.Copy(df, af); err != nil {
			return err
		}

		df.Close()
		af.Close()
	}

	return nil
}

func parseParameters(w *api.Workflow) (*trivyConfig, error) {
	cfg := &trivyConfig{}

	for k, v := range w.Parameters {
		switch strings.ToLower(k) {
		case "action":
			cfg.Action = v
		case "target":
			cfg.Target = v
		default:
			return nil, fmt.Errorf("unknown parameter specified: %s", k)
		}
	}

	return cfg, nil
}
