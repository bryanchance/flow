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
package processor

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	blenderConfigTemplate = `import bpy
import os

prop = bpy.context.preferences.addons['cycles'].preferences
prop.get_devices()

# set output path
filename = bpy.path.basename(bpy.data.filepath)
filename = os.path.splitext(filename)[0]

if filename:
    current_dir = os.getcwd()

{{ if .RenderUseGPU }}
# TODO: detect OPTIX support and use
for compute_device_type in ('CUDA', 'OPENCL', 'NONE'):
    try:
        prop.compute_device_type = compute_device_type
        print("Enabled device " + compute_device_type)
        break
    except TypeError:
        pass
{{ else }}
prop.compute_device_type = 'NONE'
{{ end }}

for scene in bpy.data.scenes:
    scene.cycles.device = '{{ if .RenderUseGPU }}GPU{{ else }}CPU{{ end }}'
    scene.cycles.samples = {{ .RenderSamples }}
    scene.render.engine = '{{ .RenderEngine }}'
    scene.render.resolution_x = {{ .ResolutionX }}
    scene.render.resolution_y = {{ .ResolutionY }}
    scene.render.resolution_percentage = {{ .ResolutionScale }}
    scene.render.filepath = '{{ .OutputDir }}_'
    # TODO: make output format, color mode, etc. configurable
    scene.render.image_settings.file_format = 'PNG'
    scene.render.image_settings.color_mode ='RGBA'
`
)

type blenderConfig struct {
	RenderEngine    string
	RenderFrame     int
	RenderUseGPU    bool
	RenderSamples   int
	ResolutionX     int
	ResolutionY     int
	ResolutionScale int
	OutputDir       string
}

func (p *Processor) renderWorkflow(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	w := cfg.Workflow
	output := &workflows.ProcessorOutput{
		Parameters: map[string]string{},
	}

	startedAt := time.Now()

	blenderCfg, err := parseParameters(w)
	if err != nil {
		return nil, err
	}

	// workflow output info
	output.Parameters["engine"] = blenderCfg.RenderEngine
	output.Parameters["frame"] = fmt.Sprintf("%d", blenderCfg.RenderFrame)
	output.Parameters["gpu"] = fmt.Sprintf("%v", blenderCfg.RenderUseGPU)
	output.Parameters["samples"] = fmt.Sprintf("%d", blenderCfg.RenderSamples)
	output.Parameters["x"] = fmt.Sprintf("%d", blenderCfg.ResolutionX)
	output.Parameters["y"] = fmt.Sprintf("%d", blenderCfg.ResolutionY)
	output.Parameters["scale"] = fmt.Sprintf("%d", blenderCfg.ResolutionScale)

	// render with blender
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("flow-%s", w.ID))
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	// setup output dir
	outputDir, err := os.MkdirTemp("", fmt.Sprintf("flow-output-%s", w.ID))
	if err != nil {
		return nil, err
	}
	// set output dir for upload
	output.OutputDir = outputDir

	workflowDir := cfg.InputDir

	blenderOutputDir := getPythonOutputDir(filepath.Join(outputDir, "render"))
	blenderCfg.OutputDir = blenderOutputDir

	// find .blend
	projectFilePath := ""
	projectFiles, err := filepath.Glob(filepath.Join(workflowDir, "*.blend"))
	if err != nil {
		return nil, err
	}
	archiveFiles, err := filepath.Glob(filepath.Join(workflowDir, "*.zip"))
	if err != nil {
		return nil, err
	}
	// find .blend first, then .zip
	if len(projectFiles) > 0 {
		projectFilePath = projectFiles[0]
	}
	if len(archiveFiles) > 0 && projectFilePath == "" {
		projectFilePath = archiveFiles[0]
	}
	if projectFilePath == "" {
		return nil, fmt.Errorf("unable to find a blender project in the workflow (*.blend or .zip)")
	}

	projectFile, err := os.Open(projectFilePath)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("checking content type for %s", projectFile.Name())
	contentType, err := getContentType(projectFile)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting content type for %s", projectFile.Name())
	}

	switch contentType {
	case "application/zip":
		logrus.Debug("zip archive detected; extracting")
		a, err := zip.OpenReader(projectFilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading zip archive %s", projectFilePath)
		}
		defer a.Close()

		for _, f := range a.File {
			fp := filepath.Join(tmpDir, f.Name)
			if !strings.HasPrefix(fp, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
				logrus.Warnf("invalid path for archive file %s", f.Name)
				continue
			}
			if f.FileInfo().IsDir() {
				if err := os.MkdirAll(fp, 0750); err != nil {
					return nil, errors.Wrapf(err, "error creating dir from archive for %s", f.Name)
				}
				continue
			}

			if err := os.MkdirAll(filepath.Dir(fp), 0750); err != nil {
				return nil, errors.Wrapf(err, "error creating parent dir from archive for %s", f.Name)
			}

			pf, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return nil, errors.Wrapf(err, "error creating file from archive for %s", f.Name)
			}

			fa, err := f.Open()
			if err != nil {
				return nil, errors.Wrapf(err, "error opening file in archive: %s", f.Name)
			}

			if _, err := io.Copy(pf, fa); err != nil {
				return nil, errors.Wrapf(err, "error extracting file from archive: %s", f.Name)
			}
			pf.Close()
			fa.Close()
		}

		// find blender project
		projectFiles, err := filepath.Glob(filepath.Join(tmpDir, "*.blend"))
		if err != nil {
			return nil, err
		}

		if len(projectFiles) == 0 {
			return nil, errors.New("Blender project file (.blend) not found in zip archive")
		}

		projectFilePath = projectFiles[0]
		if len(projectFiles) > 1 {
			logrus.Warnf("multiple blender (.blend) projects found; using first detected: %s", projectFilePath)
		}
	}

	logrus.Debugf("using blender config: %+v", blenderCfg)
	blenderConfigData, err := generateBlenderRenderConfig(blenderCfg)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("blender config: %+v", blenderCfg)

	blenderConfigPath := filepath.Join(tmpDir, "render.py")

	if err := ioutil.WriteFile(blenderConfigPath, []byte(blenderConfigData), 0644); err != nil {
		return nil, err
	}

	logrus.Debugf("using blender %s", p.blenderPath)
	args := []string{
		"-b",
		"--factory-startup",
		projectFilePath,
		"-P",
		blenderConfigPath,
		"--debug-python",
		"-f",
		fmt.Sprintf("%d", blenderCfg.RenderFrame),
	}
	args = append(args, blenderCommandPlatformArgs...)

	logrus.Debugf("blender args: %v", args)
	c := exec.CommandContext(ctx, p.blenderPath, args...)

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
	output.FinishedAt = time.Now()
	output.Duration = output.FinishedAt.Sub(startedAt)
	output.Log = string(out)

	logrus.Debugf("workflow complete: %+v", output)

	return output, nil
}

func parseParameters(w *api.Workflow) (*blenderConfig, error) {
	cfg := &blenderConfig{
		RenderEngine:    "CYCLES",
		RenderFrame:     1,
		RenderUseGPU:    false,
		RenderSamples:   100,
		ResolutionX:     1920,
		ResolutionY:     1080,
		ResolutionScale: 100,
	}

	for k, v := range w.Parameters {
		switch strings.ToLower(k) {
		case "engine":
			cfg.RenderEngine = strings.ToUpper(v)
		case "frame":
			x, err := strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
			cfg.RenderFrame = x
		case "gpu":
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, err
			}
			cfg.RenderUseGPU = b
		case "samples":
			x, err := strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
			cfg.RenderSamples = x
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
		case "scale":
			x, err := strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
			cfg.ResolutionScale = x
		default:
			return nil, fmt.Errorf("unknown parameter specified: %s", k)
		}
	}

	return cfg, nil
}

func generateBlenderRenderConfig(cfg *blenderConfig) (string, error) {
	t, err := template.New("blender").Parse(blenderConfigTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, cfg); err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}
