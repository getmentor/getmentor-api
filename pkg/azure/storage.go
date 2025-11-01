package azure

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"go.uber.org/zap"
)

// StorageClient represents an Azure Blob Storage client
type StorageClient struct {
	containerClient *container.Client
	containerName   string
	storageDomain   string
}

// NewStorageClient creates a new Azure Storage client
func NewStorageClient(connectionString, containerName, storageDomain string) (*StorageClient, error) {
	serviceClient, err := azblob.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure service client: %w", err)
	}

	containerClient := serviceClient.ServiceClient().NewContainerClient(containerName)

	// Create container if it doesn't exist
	ctx := context.Background()
	accessType := container.PublicAccessTypeBlob
	_, err = containerClient.Create(ctx, &container.CreateOptions{
		Access: &accessType,
	})
	if err != nil && !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	logger.Info("Azure Storage client initialized",
		zap.String("container", containerName),
		zap.String("domain", storageDomain),
	)

	return &StorageClient{
		containerClient: containerClient,
		containerName:   containerName,
		storageDomain:   storageDomain,
	}, nil
}

// UploadImage uploads an image to Azure Blob Storage
func (s *StorageClient) UploadImage(imageData, fileName, contentType string) (string, error) {
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
		metrics.AzureStorageRequestDuration.WithLabelValues(operation, "error").Observe(metrics.MeasureDuration(start))
		metrics.AzureStorageRequestTotal.WithLabelValues(operation, "error").Inc()
		return "", fmt.Errorf("failed to decode base64 image: %w", err)
	}

	// Upload to Azure
	blobClient := s.containerClient.NewBlockBlobClient(fileName)
	ctx := context.Background()

	_, err = blobClient.UploadBuffer(ctx, imageBytes, &azblob.UploadBufferOptions{
		HTTPHeaders: &blob.HTTPHeaders{
			BlobContentType: &contentType,
		},
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AzureStorageRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AzureStorageRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall("azure_storage", operation, "error", duration,
			zap.Error(err),
			zap.String("file_name", fileName),
		)
		return "", fmt.Errorf("failed to upload image to Azure: %w", err)
	}

	metrics.AzureStorageRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AzureStorageRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall("azure_storage", operation, "success", duration,
		zap.String("file_name", fileName),
		zap.Int("size_bytes", len(imageBytes)),
	)

	// Construct public URL
	imageURL := fmt.Sprintf("https://%s/%s/%s", s.storageDomain, s.containerName, fileName)

	return imageURL, nil
}

// GenerateFileName generates a unique filename for a mentor's profile picture
func (s *StorageClient) GenerateFileName(mentorID int, originalFileName string) string {
	timestamp := time.Now().Unix()
	ext := filepath.Ext(originalFileName)
	if ext == "" {
		ext = ".jpg"
	}
	return fmt.Sprintf("tmp/%d-%d%s", mentorID, timestamp, ext)
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
