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
	"fmt"
	"testing"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/stretchr/testify/assert"
)

func TestParseParametersDefaults(t *testing.T) {
	w := &api.Workflow{
		Parameters: map[string]string{},
	}

	cfg, err := parseParameters(w)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, cfg.RenderEngine, "CYCLES", "expected default render engine CYCLES")
	assert.Equal(t, cfg.RenderSamples, 100, "expected default render samples")
}

func TestParseParametersCustom(t *testing.T) {
	testEngine := "EEVEE"
	testFrame := 2
	testEnableGPU := true
	testSamples := 50
	testX := 720
	testY := 480
	testScale := 25

	w := &api.Workflow{
		Parameters: map[string]string{
			"engine":  testEngine,
			"frame":   fmt.Sprintf("%v", testFrame),
			"gpu":     fmt.Sprintf("%+v", testEnableGPU),
			"samples": fmt.Sprintf("%+v", testSamples),
			"x":       fmt.Sprintf("%d", testX),
			"y":       fmt.Sprintf("%d", testY),
			"scale":   fmt.Sprintf("%v", testScale),
		},
	}

	cfg, err := parseParameters(w)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, cfg.RenderEngine, testEngine, "unexpected engine")
	assert.Equal(t, cfg.RenderFrame, testFrame, "unexpected frame")
	assert.Equal(t, cfg.RenderUseGPU, testEnableGPU, "unexpected GPU")
	assert.Equal(t, cfg.RenderSamples, testSamples, "unexpected samples")
	assert.Equal(t, cfg.ResolutionX, testX, "unexpected resolution X")
	assert.Equal(t, cfg.ResolutionY, testY, "unexpected resolution Y")
	assert.Equal(t, cfg.ResolutionScale, testScale, "unexpected scale")
}
