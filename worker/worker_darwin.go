package worker

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var (
	blenderExecutableName      = "blender"
	blenderCommandPlatformArgs = []string{}
)

func getCPUs() (uint32, error) {
	cpus, err := cpu.Counts(true)
	if err != nil {
		return 0, err
	}
	return uint32(cpus), nil
}

func getMemory() (int64, error) {
	m, err := mem.VirtualMemory()
	if err != nil {
		return -1, err
	}
	return int64(m.Total), nil
}

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
