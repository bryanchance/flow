package datastore

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/jobs/v1"
	"git.underland.io/ehazlett/fynca/pkg/tracing"
	minio "github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"

	"go.opentelemetry.io/otel/trace"
)

var (
	// default 5 minutes for latest render storage cache
	latestRenderCacheTTL = time.Second * 60 * 5
	// force 10 second cache on slices
	latestRenderSliceCacheTTL = time.Second * 10
)

func (d *Datastore) GetLatestRender(ctx context.Context, jobID string, frame int64, ttl time.Duration) (string, error) {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "GetLatestRender")
	defer span.End()

	cacheKey := getLatestRenderCacheKey(jobID, frame)
	// cache url and retrieve if less than the ttl
	cacheData, err := d.GetCacheObject(ctx, cacheKey)
	if err != nil {
		return "", err
	}

	// override ttl; cannot be indefinite
	if ttl == (time.Second * 0) {
		ttl = latestRenderCacheTTL
	}

	job, err := d.GetJob(ctx, jobID)
	if err != nil {
		return "", err
	}

	hasSlices := job.Request.RenderSlices > 0

	// if cached, return if not a sliced render
	// improve sliced render cache
	if cacheData != nil {
		// get composite if sliced
		if hasSlices && job.Status == api.JobStatus_RENDERING {
			return d.GetCompositeRender(ctx, jobID, frame, latestRenderSliceCacheTTL)
		}
		return string(cacheData), nil
	}

	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)

	renderPath := path.Join(namespace, fynca.S3RenderPath, jobID)
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    renderPath,
		Recursive: true,
	})

	latestRenderPath := ""
	for o := range objCh {
		if o.Err != nil {
			return "", o.Err
		}

		// TODO: support other file types
		if filepath.Ext(o.Key) != ".png" {
			continue
		}
		// ignore slice renders
		sliceMatch, err := regexp.MatchString(".*_slice-\\d+_\\d+\\.[png]", o.Key)
		if err != nil {
			return "", err
		}
		if sliceMatch {
			continue
		}
		frameMatch, err := regexp.MatchString(fmt.Sprintf(".*_%04d.[png]", frame), o.Key)
		if err != nil {
			return "", err
		}
		if frameMatch {
			latestRenderPath = o.Key
			break
		}
	}

	if latestRenderPath == "" {
		return "", nil
	}

	// get presigned url for archive
	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", "inline")

	// valid for 24 hours
	presignedURL, err := d.storageClient.PresignedGetObject(ctx, d.config.S3Bucket, latestRenderPath, ttl, reqParams)
	if err != nil {
		return "", err
	}

	if err := d.SetCacheObject(ctx, cacheKey, []byte(presignedURL.String()), ttl); err != nil {
		logrus.WithError(err).Warnf("error caching object for %s", latestRenderPath)
	}

	return presignedURL.String(), nil
}

func (d *Datastore) DeleteRenders(ctx context.Context, id string) error {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    path.Join(fynca.S3RenderPath, id),
		Recursive: true,
	})

	for o := range objCh {
		if o.Err != nil {
			return o.Err
		}

		if err := d.storageClient.RemoveObject(ctx, d.config.S3Bucket, o.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (d *Datastore) GetCompositeRender(ctx context.Context, jobID string, frame int64, ttl time.Duration) (string, error) {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "GetCompositeRender")
	defer span.End()

	cacheKey := getLatestRenderCompositeCacheKey(jobID, frame)
	// cache url and retrieve if less than the ttl
	cacheData, err := d.GetCacheObject(ctx, cacheKey)
	if err != nil {
		return "", err
	}
	if cacheData != nil {
		return string(cacheData), nil
	}

	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)

	data, err := d.CompositeFrameRender(ctx, jobID, frame)
	if err != nil {
		return "", err
	}
	buf := bytes.NewBuffer(data)

	compositePath := getFrameCompositePath(namespace, jobID, frame)
	logrus.Debugf("storing composite for %s (frame %d) at %s", jobID, frame, compositePath)

	if _, err := d.storageClient.PutObject(ctx, d.config.S3Bucket, compositePath, buf, int64(buf.Len()), minio.PutObjectOptions{ContentType: "image/png"}); err != nil {
		return "", err
	}

	// get presigned url for archive
	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", "inline")

	// valid for 24 hours
	presignedURL, err := d.storageClient.PresignedGetObject(ctx, d.config.S3Bucket, compositePath, ttl, reqParams)
	if err != nil {
		return "", err
	}

	if err := d.SetCacheObject(ctx, cacheKey, []byte(presignedURL.String()), ttl); err != nil {
		logrus.WithError(err).Warnf("error caching object for %s", compositePath)
	}

	return presignedURL.String(), nil
}

func (d *Datastore) CompositeFrameRender(ctx context.Context, jobID string, frame int64) ([]byte, error) {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "GetCompositeFrameRender")
	defer span.End()

	logrus.Debugf("compositing frame render for %s frame %d", jobID, frame)
	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)

	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("fynca-composite-%s", jobID))
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    fmt.Sprintf("%s/%s/%s", namespace, fynca.S3RenderPath, jobID),
		Recursive: true,
	})
	slices := []string{}
	for o := range objCh {
		if o.Err != nil {
			return nil, o.Err
		}

		// filter logs
		if filepath.Ext(o.Key) == ".log" {
			continue
		}

		// filter frame
		frameMatch, err := regexp.MatchString(fmt.Sprintf(".*_%04d.[png]", frame), o.Key)
		if err != nil {
			return nil, err
		}
		if !frameMatch {
			continue
		}
		slices = append(slices, o.Key)
	}

	for _, p := range slices {
		obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, p, minio.GetObjectOptions{})
		if err != nil {
			return nil, err
		}
		fileName := filepath.Base(p)
		f, err := os.Create(filepath.Join(tmpDir, fileName))
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(f, obj); err != nil {
			return nil, err
		}
		f.Close()
	}

	// process images
	frameSlices := []image.Image{}
	files, err := filepath.Glob(filepath.Join(tmpDir, fmt.Sprintf("*_%04d.*", frame)))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if ext := filepath.Ext(f); strings.ToLower(ext) != ".png" {
			logrus.Warnf("only .png files can be composited; skipping %s", f)
			continue
		}
		imgFile, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		img, err := png.Decode(imgFile)
		if err != nil {
			return nil, err
		}
		frameSlices = append(frameSlices, img)
		imgFile.Close()
	}
	if len(frameSlices) == 0 {
		// not rendered yet
		return nil, nil
	}

	bounds := frameSlices[0].Bounds()
	finalImage := image.NewRGBA(bounds)
	for _, img := range frameSlices {
		draw.Draw(finalImage, bounds, img, image.ZP, draw.Over)
	}

	finalFilePath := filepath.Join(tmpDir, fmt.Sprintf("%s_%04d.png", jobID, frame))
	final, err := os.Create(finalFilePath)
	if err != nil {
		return nil, err
	}
	// TODO: allow for configurable compression levels
	enc := &png.Encoder{
		CompressionLevel: png.NoCompression,
	}
	if err := enc.Encode(final, finalImage); err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(finalFilePath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (d *Datastore) ClearLatestRenderCache(ctx context.Context, jobID string, frame int64) error {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "ClearLatestRenderCache")
	defer span.End()

	logrus.Debugf("clearing render cache for job %s (frame %d)", jobID, frame)
	latestRenderKey := getLatestRenderCacheKey(jobID, frame)
	latestCompositeKey := getLatestRenderCompositeCacheKey(jobID, frame)

	keys := []string{
		latestRenderKey,
		latestCompositeKey,
	}
	logrus.Debugf("cache keys: %+v", keys)
	for _, k := range keys {
		if err := d.ClearCacheObject(ctx, k); err != nil {
			return err
		}
	}

	return nil
}

func getLatestRenderCacheKey(jobID string, frame int64) string {
	return fmt.Sprintf("%s-%d", jobID, frame)
}

func getLatestRenderCompositeCacheKey(jobID string, frame int64) string {
	return fmt.Sprintf("composite-%s-%d", jobID, frame)
}

func getFrameCompositePath(namespace, jobID string, frame interface{}) string {
	return path.Join(getStoragePath(namespace, jobID), fmt.Sprintf("composite_%04d.png", frame))
}
