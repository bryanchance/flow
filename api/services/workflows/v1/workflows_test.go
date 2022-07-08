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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowsSQLValueScan(t *testing.T) {
	w := &Workflow{
		ID:   "12345",
		Type: "dev.flow.test",
	}

	v, err := w.Value()
	if err != nil {
		t.Fatal(err)
	}

	x := &Workflow{}
	if err := x.Scan(v.([]byte)); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, w.ID, x.ID, "expected ID to be equal")
	assert.Equal(t, w.Type, x.Type, "expected Type to be equal")
}
