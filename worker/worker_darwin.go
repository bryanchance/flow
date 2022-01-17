package worker

var (
	blenderExecutableName      = "blender"
	blenderCommandPlatformArgs = []string{}
)

func getCPUThreads() (uint32, error) {
	return uint32(0), nil
}

func getMemory() (int64, error) {
	return int64(0), nil
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
