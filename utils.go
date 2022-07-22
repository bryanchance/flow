package flow

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"

	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
)

// GenerateHash generates a sha256 hash of the string
func GenerateHash(v ...string) string {
	hash := sha256.New()
	for _, x := range v {
		hash.Write([]byte(x))
	}
	return hex.EncodeToString(hash.Sum(nil)[:])
}

// GetMinioClient returns a MinIO client using the specified flow.Config
func GetMinioClient(cfg *Config) (*minio.Client, error) {
	return minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(cfg.S3AccessID, cfg.S3AccessKey, ""),
		Secure: cfg.S3UseSSL,
	})
}

// GetContentType returns the content type of the specified file
func GetContentType(f *os.File) (string, error) {
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
