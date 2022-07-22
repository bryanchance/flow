package processor

import (
	"net/http"
	"os"
)

type GPU struct {
	Vendor  string
	Product string
}

func getContentType(f *os.File) (string, error) {
	buffer := make([]byte, 512)
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}
	_, err := f.Read(buffer)
	if err != nil {
		return "", err
	}
	contentType := http.DetectContentType(buffer)
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}
	return contentType, nil
}
