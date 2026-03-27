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
			// Application Default Credentials (recomendado no GCP)
			gcsClient, gcsErr = storage.NewClient(ctx)
		}
	})
	return gcsClient, gcsErr
}

// GCSEnabled retorna true se GCS_BUCKET estiver configurado.
func GCSEnabled() bool { return os.Getenv("GCS_BUCKET") != "" }

// UploadImageToGCS faz upload de imageBytes para o GCS e retorna a URL pública.
// Retorna ("", nil) se GCS_BUCKET não estiver definido (caller deve salvar localmente).
// O objeto é criado com ContentType image/jpeg.
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
		return "", fmt.Errorf("escrevendo no GCS: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("fechando writer GCS: %w", err)
	}

	return "https://storage.googleapis.com/" + bucket + "/" + objectName, nil
}
