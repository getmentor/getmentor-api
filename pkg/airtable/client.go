package airtable

import (
	"fmt"
	"time"

	"github.com/mehanizm/airtable"
	"github.com/sony/gobreaker"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/circuitbreaker"
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

// Client represents an Airtable client with circuit breaker protection
type Client struct {
	client         *airtable.Client
	baseID         string
	workOffline    bool
	circuitBreaker *gobreaker.CircuitBreaker
}

// NewClient creates a new Airtable client using mehanizm/airtable library
func NewClient(apiKey, baseID string, workOffline bool) (*Client, error) {
	// Initialize circuit breaker with default config
	cbConfig := circuitbreaker.DefaultConfig("airtable")
	cb := circuitbreaker.NewCircuitBreaker(cbConfig)

	if workOffline {
		logger.Info("Airtable client initialized in offline mode")
		return &Client{
			client:         nil,
			baseID:         baseID,
			workOffline:    true,
			circuitBreaker: cb,
		}, nil
	}

	// Validate credentials
	if apiKey == "" {
		return nil, fmt.Errorf("empty API key provided")
	}
	if baseID == "" {
		return nil, fmt.Errorf("empty base ID provided")
	}

	// Create client with modern airtable library (supports PAT tokens)
	client := airtable.NewClient(apiKey)

	logger.Info("Airtable client initialized",
		zap.String("base_id", baseID),
		zap.String("library", "mehanizm/airtable@v0.3.4"))

	return &Client{
		client:         client,
		baseID:         baseID,
		workOffline:    workOffline,
		circuitBreaker: cb,
	}, nil
}

// GetAllMentors fetches all approved mentors from Airtable with circuit breaker protection
func (c *Client) GetAllMentors() ([]*models.Mentor, error) {
	if c.workOffline {
		logger.Info("Returning test data in offline mode")
		return c.getTestMentors(), nil
	}

	// Execute the request through the circuit breaker
	return circuitbreaker.ExecuteWithFallback(
		c.circuitBreaker,
		func() ([]*models.Mentor, error) {
			return c.fetchAllMentors()
		},
		func() ([]*models.Mentor, error) {
			// Fallback: return empty list and log warning
			logger.Warn("Circuit breaker open for Airtable, returning empty mentor list")
			return []*models.Mentor{}, nil
		},
	)
}

// fetchAllMentors performs the actual Airtable API call
func (c *Client) fetchAllMentors() ([]*models.Mentor, error) {
	start := time.Now()
	operation := "getAllMentors"

	table := c.client.GetTable(c.baseID, MentorsTableName)

	// Fetch records from the view
	records, err := table.GetRecords().
		FromView(MentorsViewName).
		ReturnFields(
			"Id", "Alias", "Name", "Description", "JobTitle", "Workplace",
			"Details", "About", "Competencies", "Experience", "Price",
			"Done Sessions Count", "Image_Attachment", "Image", "Tags",
			"SortOrder", "OnSite", "Status", "AuthToken", "Calendly Url", "Is New",
		).
		Do()

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall("airtable", operation, "error", duration, zap.Error(err))
		return nil, fmt.Errorf("failed to fetch mentors from Airtable: %w", err)
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall("airtable", operation, "success", duration, zap.Int("count", len(records.Records)))

	// Convert to mentor models
	mentors := make([]*models.Mentor, 0, len(records.Records))
	for i := range records.Records {
		mentor := models.AirtableRecordToMentor(records.Records[i])
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
	// Just fetch all mentors and filter - simpler than using the API filter
	mentors, err := c.GetAllMentors()
	if err != nil {
		return nil, err
	}

	for _, mentor := range mentors {
		if mentor.AirtableID == recordID {
			return mentor, nil
		}
	}

	return nil, fmt.Errorf("mentor with record ID %s not found", recordID)
}

// UpdateMentor updates a mentor record in Airtable
func (c *Client) UpdateMentor(recordID string, updates map[string]interface{}) error {
	if c.workOffline {
		logger.Info("Skipping Airtable update in offline mode", zap.String("record_id", recordID))
		return nil
	}

	start := time.Now()
	operation := "updateMentor"

	table := c.client.GetTable(c.baseID, MentorsTableName)

	records := &airtable.Records{
		Records: []*airtable.Record{
			{
				ID:     recordID,
				Fields: updates,
			},
		},
	}

	_, err := table.UpdateRecordsPartial(records)

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

	table := c.client.GetTable(c.baseID, ClientRequestsTableName)

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

	records := &airtable.Records{
		Records: []*airtable.Record{
			{
				Fields: fields,
			},
		},
	}

	_, err := table.AddRecords(records)

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

// GetAllTags fetches all tags from Airtable with circuit breaker protection
func (c *Client) GetAllTags() (map[string]string, error) {
	if c.workOffline {
		return c.getTestTags(), nil
	}

	// Execute through circuit breaker with fallback
	return circuitbreaker.ExecuteWithFallback(
		c.circuitBreaker,
		func() (map[string]string, error) {
			return c.fetchAllTags()
		},
		func() (map[string]string, error) {
			// Fallback: return empty map
			logger.Warn("Circuit breaker open for Airtable, returning empty tags map")
			return make(map[string]string), nil
		},
	)
}

// fetchAllTags performs the actual Airtable API call
func (c *Client) fetchAllTags() (map[string]string, error) {
	start := time.Now()
	operation := "getAllTags"

	table := c.client.GetTable(c.baseID, TagsTableName)

	records, err := table.GetRecords().Do()

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall("airtable", operation, "error", duration, zap.Error(err))
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall("airtable", operation, "success", duration, zap.Int("count", len(records.Records)))

	// Convert to map: tag name -> record ID
	tagsMap := make(map[string]string, len(records.Records))
	for _, record := range records.Records {
		if name, ok := record.Fields["Name"].(string); ok {
			tagsMap[name] = record.ID
		}
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
