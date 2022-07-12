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
package accounts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccountsSQLValueScan(t *testing.T) {
	a := &Account{
		ID:        "12345",
		Admin:     true,
		Username:  "test-user",
		FirstName: "Test",
		LastName:  "User",
	}

	v, err := a.Value()
	if err != nil {
		t.Fatal(err)
	}

	x := &Account{}
	if err := x.Scan(v.([]byte)); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, a.ID, x.ID, "expected ID to be equal")
	assert.Equal(t, a.Admin, x.Admin, "expected Admin to be equal")
	assert.Equal(t, a.Username, x.Username, "expected Username to be equal")
	assert.Equal(t, a.FirstName, x.FirstName, "expected FirstName to be equal")
	assert.Equal(t, a.LastName, x.LastName, "expected LastName to be equal")
}
