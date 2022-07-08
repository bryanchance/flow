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

import "github.com/pkg/errors"

var (
	// ErrAccountExists is returned if an account already exists
	ErrAccountExists = errors.New("an account with that username already exists")
	// ErrAccountDoesNotExist is returned when an account cannot be found
	ErrAccountDoesNotExist = errors.New("account does not exist")
	// ErrNamespaceDoesNotExist is returned when an namespace cannot be found
	ErrNamespaceDoesNotExist = errors.New("namespace does not exist")
	// ErrWorkflowDoesNotExist is returned when a workflow cannot be found
	ErrWorkflowDoesNotExist = errors.New("workflow does not exist")
	// ErrServiceTokenDoesNotExist is returned when a service token cannot be found
	ErrServiceTokenDoesNotExist = errors.New("service token does not exist")
	// ErrAPITokenDoesNotExist is returned when an api token cannot be found
	ErrAPITokenDoesNotExist = errors.New("api token does not exist")
)
