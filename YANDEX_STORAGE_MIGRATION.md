# Yandex Object Storage Migration

## Overview

Migrated image uploads from Azure Blob Storage to Yandex Object Storage (S3-compatible). This eliminates the need for Azure Functions to migrate images from Airtable records, which broke after the PostgreSQL migration.

**Azure Blob Storage has been completely removed** from the codebase, including:
- `pkg/azure/` package
- `test/pkg/azure/` tests
- Azure configuration from `config/config.go`
- Azure metrics from `pkg/metrics/metrics.go`
- Azure SDK dependencies from `go.mod`

## Changes Made

### 1. Configuration (`config/config.go`)

Added new `YandexStorageConfig` struct with environment variables:
- `YANDEX_STORAGE_ACCESS_KEY_ID`
- `YANDEX_STORAGE_SECRET_ACCESS_KEY`
- `YANDEX_STORAGE_BUCKET_NAME`
- `YANDEX_STORAGE_ENDPOINT` (optional, defaults to `https://storage.yandexcloud.net`)
- `YANDEX_STORAGE_REGION` (optional, defaults to `ru-central1`)

### 2. Yandex Storage Client (`pkg/yandex/storage.go`)

Created new S3-compatible client using AWS SDK v2:
- `NewStorageClient()` - Initialize client with credentials
- `UploadImage()` - Upload base64-encoded images to Yandex
- `ValidateImageType()` - Validate image content types (jpeg, jpg, png, webp)
- `ValidateImageSize()` - Validate max size (10MB)

**Features:**
- Supports both plain base64 and data URI formats
- Constructs public URLs: `https://storage.yandexcloud.net/{bucket}/{key}`
- Prometheus metrics for duration and total operations

### 3. Metrics (`pkg/metrics/metrics.go`)

Added Yandex storage metrics:
- `yandex_storage_operation_duration_seconds` - Histogram
- `yandex_storage_operation_total` - Counter
- Labels: `operation`, `status`

### 4. Profile Service (`internal/services/profile_service.go`)

Updated `UploadPictureByMentorId()`:
- Replaced Azure client with Yandex client
- Uploads same image 3 times with paths:
  - `{slug}/full`
  - `{slug}/large`
  - `{slug}/small`
- Stores `/full` URL in database
- **Webhook trigger commented out** for future thumbnail generation

**Technical Debt:** Currently uploads same image 3 times. Future: generate actual thumbnails.

### 5. Registration Service (`internal/services/registration_service.go`)

Updated `uploadProfilePicture()`:
- Replaced Azure client with Yandex client
- Generates slug using `slug.GenerateMentorSlug(name, legacyID)`
- Uploads 3 sizes: `{slug}/full`, `{slug}/large`, `{slug}/small`
- Stores `/full` URL in database

### 6. Main Application (`cmd/api/main.go`)

- Removed Azure Storage client initialization
- Added Yandex Storage client initialization
- Updated ProfileService and RegistrationService to use Yandex client

### 7. Dependencies (`go.mod`)

Added AWS SDK v2 for S3:
- `github.com/aws/aws-sdk-go-v2/service/s3`
- `github.com/aws/aws-sdk-go-v2/credentials`
- `github.com/aws/aws-sdk-go-v2/aws`

## Tests

### Yandex Storage Tests (`pkg/yandex/storage_test.go`)

- `TestValidateImageType` - Valid/invalid content types
- `TestValidateImageSize` - Size validation (1KB to 11MB)
- `TestNewStorageClient_DefaultValues` - Default endpoint/region
- `TestUploadImage_Base64Decoding` - Base64 and data URI parsing
- `TestUploadImage_URLConstruction` - Public URL generation

### Service Tests (`test/internal/services/`)

**profile_service_test.go:**
- Upload logic with 3 sizes
- Validation failure handling
- Upload failure handling

**registration_service_test.go:**
- Slug generation from names (Latin, Cyrillic, special chars)
- Multi-size upload verification
- Upload failure handling
- Database update logic

## Migration Path

### Before
1. Backend uploads to Azure Blob Storage → `tmp/{legacyID}-{timestamp}.jpg`
2. Azure Functions watch Airtable records
3. When Airtable `Image` field changes → migrate to Yandex with thumbnails
4. **BROKEN** after PostgreSQL migration (no Airtable records)

### After
1. Backend uploads directly to Yandex Object Storage
2. Paths: `{slug}/full`, `{slug}/large`, `{slug}/small`
3. Stores `/full` URL in PostgreSQL `mentors.image`
4. No Azure Functions dependency for uploads

## Deployment

### Environment Variables Required

```bash
# Yandex Object Storage
YANDEX_STORAGE_ACCESS_KEY_ID=your-access-key
YANDEX_STORAGE_SECRET_ACCESS_KEY=your-secret-key
YANDEX_STORAGE_BUCKET_NAME=your-bucket-name
YANDEX_STORAGE_ENDPOINT=https://storage.yandexcloud.net  # optional
YANDEX_STORAGE_REGION=ru-central1                         # optional
```

### Testing

```bash
# Run all tests
go test ./...

# Run Yandex storage tests
go test ./pkg/yandex/... -v

# Run service tests
go test ./test/internal/services/... -v

# Build
go build -v ./...
```

## Future Improvements

1. **Thumbnail Generation**: Re-enable webhook trigger to Azure Functions for actual thumbnail creation
2. **Image Optimization**: Implement image compression before upload
3. **CDN Integration**: Add CDN URL prefix for better performance

## Backward Compatibility

- **Existing Azure URLs in database remain valid** - They are just URLs pointing to Azure storage
- New uploads go to Yandex
- Frontend already supports both Azure and Yandex URLs
- No frontend changes required

## Rollback Plan

If issues arise, revert the entire migration by:
1. Restore Azure package files from git history
2. Revert changes to services and main.go
3. Add Azure SDK dependencies back
4. Ensure Azure credentials are configured
5. Redeploy

Note: Since Azure has been completely removed, rollback requires restoring multiple files from git history.

## Monitoring

Watch these metrics in Grafana:
- `yandex_storage_operation_duration_seconds`
- `yandex_storage_operation_total{status="error"}`
- `getmentor_profile_picture_uploads_total{status="error"}`

## Notes

- Same image uploaded 3 times is intentional (tech debt documented)
- Webhook to MentorUpdatedTriggerURL is commented out but preserved for future use
- All tests pass ✅
- Project builds successfully ✅
