package worker

import (
	"strings"

	"github.com/jaypipes/ghw"
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
	return outputDir
}
