package workflows

import (
	"strings"

	"github.com/jaypipes/ghw"
)

func getGPUs() ([]*GPU, error) {
	gpu, err := ghw.GPU()
	if err != nil {
		return nil, err
	}
	gpus := []*GPU{}
	for _, card := range gpu.GraphicsCards {
		if card.DeviceInfo == nil {
			continue
		}
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
		if card.DeviceInfo != nil && strings.Index(strings.ToLower(strings.TrimSpace(card.DeviceInfo.Vendor.Name)), "nvidia") > -1 {
			return true, nil
		}
	}

	return false, nil
}

func getPythonOutputDir(outputDir string) string {
	return outputDir
}
