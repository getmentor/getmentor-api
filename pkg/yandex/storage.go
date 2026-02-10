package yandex

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"go.uber.org/zap"
)

// StorageClient represents a Yandex Object Storage client (S3-compatible)
type StorageClient struct {
	s3Client   *s3.Client
	bucketName string
	endpoint   string
}

// NewStorageClient creates a new Yandex Object Storage client using S3 SDK
func NewStorageClient(accessKeyID, secretAccessKey, bucketName, endpoint, region string) (*StorageClient, error) {
	// Default endpoint if not provided
	if endpoint == "" {
		endpoint = "https://storage.yandexcloud.net"
	}

	// Default region if not provided
	if region == "" {
		region = "ru-central1"
	}

	// Create S3 client configured for Yandex Object Storage
	s3Client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpoint),
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"", // session token not needed
		),
	})

	logger.Info("Yandex Object Storage client initialized",
		zap.String("bucket", bucketName),
		zap.String("endpoint", endpoint),
		zap.String("region", region),
	)

	return &StorageClient{
		s3Client:   s3Client,
		bucketName: bucketName,
		endpoint:   endpoint,
	}, nil
}

// UploadImage uploads an image to Yandex Object Storage
// Returns the public URL of the uploaded image
func (s *StorageClient) UploadImage(ctx context.Context, imageData, key, contentType string) (string, error) {
	start := time.Now()
	operation := "uploadImage"

	// Decode base64 image data
	var imageBytes []byte
	var err error

	// Handle data URI format (data:image/png;base64,...)
	if strings.HasPrefix(imageData, "data:") {
		parts := strings.SplitN(imageData, ",", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid data URI format")
		}
		imageBytes, err = base64.StdEncoding.DecodeString(parts[1])
	} else {
		imageBytes, err = base64.StdEncoding.DecodeString(imageData)
	}

	if err != nil {
		metrics.YandexStorageRequestDuration.WithLabelValues(operation, "error").Observe(metrics.MeasureDuration(start))
		metrics.YandexStorageRequestTotal.WithLabelValues(operation, "error").Inc()
		return "", fmt.Errorf("failed to decode base64 image: %w", err)
	}

	// Upload to Yandex Object Storage
	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(imageBytes),
		ContentType: aws.String(contentType),
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.YandexStorageRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.YandexStorageRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "yandex_storage", operation, "error", duration,
			zap.Error(err),
			zap.String("key", key),
		)
		return "", fmt.Errorf("failed to upload image to Yandex: %w", err)
	}

	metrics.YandexStorageRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.YandexStorageRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall(ctx, "yandex_storage", operation, "success", duration,
		zap.String("key", key),
		zap.Int("size_bytes", len(imageBytes)),
	)

	// Construct public URL
	// Format: https://storage.yandexcloud.net/{bucket}/{key}
	imageURL := fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucketName, key)

	return imageURL, nil
}

// ValidateImageType validates the image content type
func (s *StorageClient) ValidateImageType(contentType string) error {
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/webp": true,
	}

	if !validTypes[strings.ToLower(contentType)] {
		return fmt.Errorf("invalid file type: %s. Allowed types: jpeg, jpg, png, webp", contentType)
	}

	return nil
}

// ValidateImageSize validates the image size (max 10MB)
func (s *StorageClient) ValidateImageSize(imageData string) error {
	const maxSize = 10 * 1024 * 1024 // 10MB

	// Decode to check size
	var imageBytes []byte
	var err error

	if strings.HasPrefix(imageData, "data:") {
		parts := strings.SplitN(imageData, ",", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid data URI format")
		}
		imageBytes, err = base64.StdEncoding.DecodeString(parts[1])
	} else {
		imageBytes, err = base64.StdEncoding.DecodeString(imageData)
	}

	if err != nil {
		return fmt.Errorf("failed to decode image for size validation: %w", err)
	}

	if len(imageBytes) > maxSize {
		return fmt.Errorf("file too large: %d bytes (max %d bytes)", len(imageBytes), maxSize)
	}

	return nil
}
