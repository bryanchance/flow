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
	"context"
	"os"
	"path"
	"path/filepath"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/workflows/v1"
	minio "github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

func (p *Processor) uploadOutputDir(ctx context.Context, w *api.Workflow, outputDir string) ([]*api.WorkflowOutputArtifact, error) {
	mc, err := fynca.GetMinioClient(p.config)
	if err != nil {
		return nil, err
	}

	artifacts := []*api.WorkflowOutputArtifact{}

	files, err := filepath.Glob(filepath.Join(outputDir, "*"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		logrus.Debugf("uploading output file %s", f)
		rf, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		defer rf.Close()

		contentType, err := getContentType(rf)
		if err != nil {
			return nil, err
		}

		fs, err := rf.Stat()
		if err != nil {
			return nil, err
		}
		if fs.IsDir() {
			continue
		}

		objectPath := path.Join(w.Namespace, fynca.S3WorkflowPath, w.ID, filepath.Base(f))
		if _, err := mc.PutObject(ctx, p.config.S3Bucket, objectPath, rf, fs.Size(), minio.PutObjectOptions{ContentType: contentType}); err != nil {
			return nil, err
		}

		artifacts = append(artifacts, &api.WorkflowOutputArtifact{
			Name:        filepath.Base(f),
			ContentType: contentType,
			StoragePath: objectPath,
		})
	}

	return artifacts, nil
}
