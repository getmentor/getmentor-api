package airtable

import (
	"fmt"
	"time"

	"github.com/fabioberger/airtable-go"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"go.uber.org/zap"
)

const (
	MentorsTableName        = "Mentors"
	MentorsViewName         = "All Approved"
	ClientRequestsTableName = "Client Requests"
	TagsTableName           = "Tags"
)

// Client represents an Airtable client
type Client struct {
	client      *airtable.Client
	baseID      string
	workOffline bool
}

// NewClient creates a new Airtable client
func NewClient(apiKey, baseID string, workOffline bool) (*Client, error) {
	if workOffline {
		logger.Info("Airtable client initialized in offline mode")
		return &Client{
			client:      nil,
			baseID:      baseID,
			workOffline: true,
		}, nil
	}

	client, err := airtable.New(apiKey, baseID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Airtable client: %w", err)
	}

	logger.Info("Airtable client initialized", zap.String("base_id", baseID))
	return &Client{
		client:      client,
		baseID:      baseID,
		workOffline: workOffline,
	}, nil
}

// GetAllMentors fetches all approved mentors from Airtable
func (c *Client) GetAllMentors() ([]*models.Mentor, error) {
	if c.workOffline {
		logger.Info("Returning test data in offline mode")
		return c.getTestMentors(), nil
	}

	start := time.Now()
	operation := "getAllMentors"

	// Define fields to fetch
	fields := []string{
		"Id", "Alias", "Name", "Description", "JobTitle", "Workplace",
		"Details", "About", "Competencies", "Experience", "Price",
		"Done Sessions Count", "Image_Attachment", "Image", "Tags",
		"SortOrder", "OnSite", "Status", "AuthToken", "Calendly Url", "Is New",
	}

	records := []models.AirtableRecord{}

	// List records using the view
	err := c.client.ListRecords(MentorsTableName, &records, airtable.ListParameters{
		View:   MentorsViewName,
		Fields: fields,
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall("airtable", operation, "error", duration, zap.Error(err))
		return nil, fmt.Errorf("failed to fetch mentors from Airtable: %w", err)
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall("airtable", operation, "success", duration, zap.Int("count", len(records)))

	// Convert to mentor models
	mentors := make([]*models.Mentor, 0, len(records))
	for _, record := range records {
		mentor := record.ToMentor()
		mentors = append(mentors, mentor)
	}

	return mentors, nil
}

// GetMentorByID fetches a mentor by numeric ID
func (c *Client) GetMentorByID(id int) (*models.Mentor, error) {
	mentors, err := c.GetAllMentors()
	if err != nil {
		return nil, err
	}

	for _, mentor := range mentors {
		if mentor.ID == id {
			return mentor, nil
		}
	}

	return nil, fmt.Errorf("mentor with ID %d not found", id)
}

// GetMentorBySlug fetches a mentor by slug
func (c *Client) GetMentorBySlug(slug string) (*models.Mentor, error) {
	mentors, err := c.GetAllMentors()
	if err != nil {
		return nil, err
	}

	for _, mentor := range mentors {
		if mentor.Slug == slug {
			return mentor, nil
		}
	}

	return nil, fmt.Errorf("mentor with slug %s not found", slug)
}

// GetMentorByRecordID fetches a mentor by Airtable record ID
func (c *Client) GetMentorByRecordID(recordID string) (*models.Mentor, error) {
	if c.workOffline {
		return nil, fmt.Errorf("GetMentorByRecordID not supported in offline mode")
	}

	start := time.Now()
	operation := "getMentorByRecordID"

	var record models.AirtableRecord
	err := c.client.GetRecord(MentorsTableName, recordID, &record)

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall("airtable", operation, "error", duration, zap.Error(err))
		return nil, fmt.Errorf("failed to fetch mentor by record ID: %w", err)
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall("airtable", operation, "success", duration)

	return record.ToMentor(), nil
}

// UpdateMentor updates a mentor record in Airtable
func (c *Client) UpdateMentor(recordID string, updates map[string]interface{}) error {
	if c.workOffline {
		logger.Info("Skipping Airtable update in offline mode", zap.String("record_id", recordID))
		return nil
	}

	start := time.Now()
	operation := "updateMentor"

	err := c.client.UpdateRecord(MentorsTableName, recordID, updates, nil)

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall("airtable", operation, "error", duration, zap.Error(err))
		return fmt.Errorf("failed to update mentor: %w", err)
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall("airtable", operation, "success", duration, zap.String("record_id", recordID))

	return nil
}

// UpdateMentorImage updates a mentor's profile image
func (c *Client) UpdateMentorImage(recordID, imageURL string) error {
	updates := map[string]interface{}{
		"Image_Attachment": imageURL,
	}

	return c.UpdateMentor(recordID, updates)
}

// CreateClientRequest creates a new client request in Airtable
func (c *Client) CreateClientRequest(req *models.ClientRequest) error {
	if c.workOffline {
		logger.Info("Skipping client request creation in offline mode")
		return nil
	}

	start := time.Now()
	operation := "createClientRequest"

	fields := map[string]interface{}{
		"Email":       req.Email,
		"Name":        req.Name,
		"Description": req.Description,
		"Telegram":    req.Telegram,
		"Mentor":      []string{req.MentorID},
	}

	if req.Level != "" {
		fields["Level"] = req.Level
	}

	var result interface{}
	err := c.client.CreateRecord(ClientRequestsTableName, fields, &result)

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall("airtable", operation, "error", duration, zap.Error(err))
		return fmt.Errorf("failed to create client request: %w", err)
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall("airtable", operation, "success", duration)

	return nil
}

// GetAllTags fetches all tags from Airtable
func (c *Client) GetAllTags() (map[string]string, error) {
	if c.workOffline {
		return c.getTestTags(), nil
	}

	start := time.Now()
	operation := "getAllTags"

	type TagRecord struct {
		ID     string
		Fields struct {
			Name string `json:"Name"`
		}
	}

	var records []TagRecord
	err := c.client.ListRecords(TagsTableName, &records, airtable.ListParameters{})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall("airtable", operation, "error", duration, zap.Error(err))
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall("airtable", operation, "success", duration, zap.Int("count", len(records)))

	// Convert to map: tag name -> record ID
	tagsMap := make(map[string]string, len(records))
	for _, record := range records {
		tagsMap[record.Fields.Name] = record.ID
	}

	return tagsMap, nil
}

// getTestMentors returns test data for offline mode
func (c *Client) getTestMentors() []*models.Mentor {
	return []*models.Mentor{
		{
			ID:           1,
			AirtableID:   "rec123",
			Slug:         "test-mentor",
			Name:         "Test Mentor",
			Job:          "Senior Developer",
			Workplace:    "Test Company",
			Description:  "Test description",
			About:        "Test about",
			Competencies: "Test competencies",
			Experience:   "5-10",
			Price:        "1000 руб",
			MenteeCount:  5,
			PhotoURL:     "https://example.com/photo.jpg",
			Tags:         []string{"Backend", "Frontend"},
			SortOrder:    1,
			IsVisible:    true,
			Sponsors:     "none",
			CalendarType: "calendly",
			IsNew:        false,
			AuthToken:    "test-token",
			CalendarURL:  "https://calendly.com/test",
		},
	}
}

// getTestTags returns test tags for offline mode
func (c *Client) getTestTags() map[string]string {
	return map[string]string{
		"Backend":  "rec1",
		"Frontend": "rec2",
	}
}
