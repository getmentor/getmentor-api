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

// decodeBase64Image decodes a base64-encoded image string, handling both raw base64
// and data URI format (data:image/png;base64,...). Returns the decoded bytes.
func decodeBase64Image(imageData string) ([]byte, error) {
	if strings.HasPrefix(imageData, "data:") {
		parts := strings.SplitN(imageData, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid data URI format")
		}
		return base64.StdEncoding.DecodeString(parts[1])
	}
	return base64.StdEncoding.DecodeString(imageData)
}

// UploadImage uploads an image to Yandex Object Storage
// Returns the public URL of the uploaded image
func (s *StorageClient) UploadImage(ctx context.Context, imageData, key, contentType string) (string, error) {
	start := time.Now()
	operation := "uploadImage"

	// Decode base64 image data
	imageBytes, err := decodeBase64Image(imageData)
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
	imageBytes, err := decodeBase64Image(imageData)
	if err != nil {
		return fmt.Errorf("failed to decode image for size validation: %w", err)
	}

	if len(imageBytes) > maxSize {
		return fmt.Errorf("file too large: %d bytes (max %d bytes)", len(imageBytes), maxSize)
	}

	return nil
}

// UploadImageAllSizes uploads the same image in 3 sizes (full, large, small) synchronously
// NOTE: Currently uploads same image 3 times (tech debt - future: generate thumbnails)
// Validates image type and size before uploading. Returns the URL of the 'full' size image
func (s *StorageClient) UploadImageAllSizes(ctx context.Context, imageData, slug, contentType string) (string, error) {
	// Validate image type
	if err := s.ValidateImageType(contentType); err != nil {
		return "", err
	}

	// Validate image size
	if err := s.ValidateImageSize(imageData); err != nil {
		return "", err
	}

	sizes := []string{"full", "large", "small"}
	var fullImageURL string

	for _, size := range sizes {
		// Generate key: {slug}/{size} (e.g., "john-doe/full")
		key := fmt.Sprintf("%s/%s", slug, size)

		// Upload to Yandex
		imageURL, err := s.UploadImage(ctx, imageData, key, contentType)
		if err != nil {
			return "", fmt.Errorf("failed to upload image size %s: %w", size, err)
		}

		// Store the 'full' URL to return
		if size == "full" {
			fullImageURL = imageURL
		}

		logger.Info("Uploaded image size to Yandex",
			zap.String("slug", slug),
			zap.String("size", size),
			zap.String("url", imageURL))
	}

	return fullImageURL, nil
}

// UploadImageAllSizesAsync uploads the same image in 3 sizes (full, large, small) asynchronously
// NOTE: Currently uploads same image 3 times (tech debt - future: generate thumbnails)
// This is non-blocking and returns immediately. Errors are logged but not returned.
// Use this when you don't need to wait for upload completion (e.g., during registration)
func (s *StorageClient) UploadImageAllSizesAsync(ctx context.Context, imageData, slug, contentType, mentorID string) {
	// Detach from the HTTP request context so the upload isn't cancelled
	// when the handler returns the response to the client.
	bgCtx := context.WithoutCancel(ctx)
	go func() {
		fullImageURL, err := s.UploadImageAllSizes(bgCtx, imageData, slug, contentType)
		if err != nil {
			logger.Error("Failed to upload profile picture asynchronously",
				zap.Error(err),
				zap.String("mentor_id", mentorID),
				zap.String("slug", slug))
		} else {
			logger.Info("Profile picture uploaded successfully during registration",
				zap.String("mentor_id", mentorID),
				zap.String("slug", slug),
				zap.String("full_image_url", fullImageURL))
		}
	}()
}
