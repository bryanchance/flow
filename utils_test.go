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
package flow

import "testing"

func TestGenerateHashBasic(t *testing.T) {
	expected := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	v := GenerateHash("test")
	if v != expected {
		t.Fatalf("expected %q; received %q", expected, v)
	}
}

func TestGenerateHashMultiple(t *testing.T) {
	expected := "ecd71870d1963316a97e3ac3408c9835ad8cf0f3c1bc703527c30265534f75ae"
	v := GenerateHash("test", "123")
	if v != expected {
		t.Fatalf("expected %q; received %q", expected, v)
	}
}
