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
	"fmt"
	"strings"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

type GPU struct {
	Vendor  string
	Product string
}

func (h *WorkflowHandler) getProcessorInfo() (*api.ProcessorInfo, error) {
	gpus, err := getGPUs()
	if err != nil {
		return nil, err
	}

	gpuInfo := []string{}
	for _, gpu := range gpus {
		gpuInfo = append(gpuInfo, fmt.Sprintf("%s: %s", gpu.Vendor, gpu.Product))
	}

	cpus, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}

	loadStats, err := load.Avg()
	if err != nil {
		return nil, err
	}

	m, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	// set processor scope
	scope := &api.ProcessorScope{}
	switch strings.ToLower(h.cfg.Namespace) {
	case "", "global":
		scope.Scope = &api.ProcessorScope_Global{
			Global: true,
		}
	default:
		scope.Scope = &api.ProcessorScope_Namespace{
			Namespace: h.cfg.Namespace,
		}
	}

	return &api.ProcessorInfo{
		ID:              h.cfg.ID,
		Type:            h.cfg.Type,
		MaxWorkflows:    h.cfg.MaxWorkflows,
		CPUs:            uint32(cpus),
		MemoryTotal:     int64(m.Total),
		MemoryAvailable: int64(m.Available),
		GPUs:            gpuInfo,
		Load1:           loadStats.Load1,
		Load5:           loadStats.Load5,
		Load15:          loadStats.Load15,
		Scope:           scope,
	}, nil
}
