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
	"path"
	"path/filepath"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/workflows/v1"
	minio "github.com/minio/minio-go/v7"
)

func (p *Processor) unpackWorkflowInput(ctx context.Context, w *api.Workflow, dest string) error {
	mc, err := fynca.GetMinioClient(p.config)
	if err != nil {
		return err
	}

	oCh := mc.ListObjects(ctx, p.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    w.InputPath,
		Recursive: true,
	})
	for o := range oCh {
		filename := path.Base(o.Key)
		localPath := filepath.Join(dest, filename)
		if err := mc.FGetObject(ctx, p.config.S3Bucket, o.Key, localPath, minio.GetObjectOptions{}); err != nil {
			return err
		}
	}

	return nil
}
