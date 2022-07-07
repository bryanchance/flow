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

var (
	blenderExecutableName      = "blender"
	blenderCommandPlatformArgs = []string{}
)

func getGPUs() ([]*GPU, error) {
	// GPU not supported on darwin
	return nil, nil
}

func gpuEnabled() (bool, error) {
	// GPU not supported on darwin
	return false, nil
}

func getPythonOutputDir(outputDir string) string {
	return outputDir
}
