package worker

import "strings"

var (
	blenderExecutableName      = "blender.exe"
	blenderCommandPlatformArgs = []string{}
)

func getPythonOutputDir(outputDir string) string {
	return strings.ReplaceAll(outputDir, "\\", "/")
}
