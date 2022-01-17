package worker

import (
	"strings"

	"github.com/jaypipes/ghw"
)

var (
	blenderExecutableName      = "blender.exe"
	blenderCommandPlatformArgs = []string{}
)

func getCPUThreads() (uint32, error) {
	cpu, err := ghw.CPU()
	if err != nil {
		return uint32(0), err
	}

	return cpu.TotalThreads, nil
}

func getMemory() (int64, error) {
	mem, err := ghw.Memory()
	if err != nil {
		return int64(0), err
	}

	return mem.TotalUsableBytes, nil
}

func getGPUs() ([]*GPU, error) {
	gpu, err := ghw.GPU()
	if err != nil {
		return nil, err
	}
	gpus := []*GPU{}
	for _, card := range gpu.GraphicsCards {
		gpus = append(gpus, &GPU{
			Vendor:  card.DeviceInfo.Vendor.Name,
			Product: card.DeviceInfo.Product.Name,
		})
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
