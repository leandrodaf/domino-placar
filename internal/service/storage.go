package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

var (
	gcsOnce   sync.Once
	gcsClient *storage.Client
	gcsErr    error
)

func getGCSClient() (*storage.Client, error) {
	gcsOnce.Do(func() {
		ctx := context.Background()
		credsJSON := os.Getenv("GCS_CREDENTIALS")
		if credsJSON != "" {
			gcsClient, gcsErr = storage.NewClient(ctx, option.WithCredentialsJSON([]byte(credsJSON)))
		} else {
			// Application Default Credentials (recommended on GCP)
			gcsClient, gcsErr = storage.NewClient(ctx)
		}
	})
	return gcsClient, gcsErr
}

// GCSEnabled returns true if GCS_BUCKET is configured.
func GCSEnabled() bool { return os.Getenv("GCS_BUCKET") != "" }

// UploadImageToGCS uploads imageBytes to GCS and returns the public URL.
// Returns ("", nil) if GCS_BUCKET is not set (caller should save locally).
// The object is created with ContentType image/jpeg.
func UploadImageToGCS(imageBytes []byte, objectName string) (string, error) {
	bucket := os.Getenv("GCS_BUCKET")
	if bucket == "" {
		return "", nil
	}

	client, err := getGCSClient()
	if err != nil {
		return "", fmt.Errorf("GCS client: %w", err)
	}

	ctx := context.Background()
	obj := client.Bucket(bucket).Object(objectName)
	w := obj.NewWriter(ctx)
	w.ContentType = "image/jpeg"

	if _, err := io.Copy(w, bytes.NewReader(imageBytes)); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("writing to GCS: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("closing GCS writer: %w", err)
	}

	return "https://storage.googleapis.com/" + bucket + "/" + objectName, nil
}
