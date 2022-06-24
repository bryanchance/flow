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
package workflows

import (
	"strings"

	"github.com/jaypipes/ghw"
)

func getGPUs() ([]*GPU, error) {
	gpu, err := ghw.GPU()
	if err != nil {
		return nil, err
	}
	gpus := []*GPU{}
	for _, card := range gpu.GraphicsCards {
		vendor := strings.ToLower(card.DeviceInfo.Vendor.Name)
		if strings.Contains(vendor, "nvidia") || strings.Contains(vendor, "amd") {
			gpus = append(gpus, &GPU{
				Vendor:  card.DeviceInfo.Vendor.Name,
				Product: card.DeviceInfo.Product.Name,
			})
		}
	}

	return gpus, nil
}

func gpuEnabled() (bool, error) {
	gpu, err := ghw.GPU()
	if err != nil {
		return false, err
	}

	for _, card := range gpu.GraphicsCards {
		if strings.Index(strings.ToLower(strings.TrimSpace(card.DeviceInfo.Vendor.Name)), "nvidia") > -1 {
			return true, nil
		}
	}

	return false, nil
}

func getPythonOutputDir(outputDir string) string {
	return strings.ReplaceAll(outputDir, "\\", "/")
}
