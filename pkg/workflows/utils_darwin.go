package workflows

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
