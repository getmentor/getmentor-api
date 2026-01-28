package airtable

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/circuitbreaker"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/getmentor/getmentor-api/pkg/retry"
	"github.com/mehanizm/airtable"
	"github.com/sony/gobreaker"
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
func (c *Client) GetAllMentors(ctx context.Context) ([]*models.Mentor, error) {
	if c.workOffline {
		logger.Info("Returning test data in offline mode")
		return c.getTestMentors(), nil
	}

	// Execute the request through the circuit breaker
	return circuitbreaker.ExecuteWithFallback(
		c.circuitBreaker,
		func() ([]*models.Mentor, error) {
			return c.fetchAllMentors(ctx)
		},
		func() ([]*models.Mentor, error) {
			// Fallback: return empty list and log warning
			logger.Warn("Circuit breaker open for Airtable, returning empty mentor list")
			return []*models.Mentor{}, nil
		},
	)
}

// fetchAllMentors performs the actual Airtable API call with retry logic
func (c *Client) fetchAllMentors(ctx context.Context) ([]*models.Mentor, error) {
	start := time.Now()
	operation := "getAllMentors"

	// Use retry logic with context timeout
	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	records, err := retry.DoWithResult(retryCtx, retryConfig, operation, func() (*airtable.Records, error) {
		table := c.client.GetTable(c.baseID, MentorsTableName)

		// Fetch ALL records from the view using manual pagination
		var allMentorRecords []*airtable.Record
		offset := ""

		for {
			query := table.GetRecords().
				FromView(MentorsViewName).
				PageSize(100). // Maximum page size to minimize API requests
				ReturnFields(
					"Id", "Alias", "Name", "Description", "JobTitle", "Workplace",
					"Details", "About", "Competencies", "Experience", "Price",
					"Done Sessions Count", "Tags", "SortOrder", "OnSite", "Status",
					"AuthToken", "Calendly Url", "Is New",
				)

			// Add offset for subsequent pages
			if offset != "" {
				query = query.WithOffset(offset)
			}

			records, err := query.Do()
			if err != nil {
				return nil, fmt.Errorf("failed to fetch mentors from Airtable: %w", err)
			}

			// Append records from this page
			allMentorRecords = append(allMentorRecords, records.Records...)

			// Check if there are more pages
			if records.Offset == "" {
				break
			}
			offset = records.Offset
		}

		// Return all records in Records wrapper
		return &airtable.Records{
			Records: allMentorRecords,
		}, nil
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return nil, err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall(ctx, "airtable", operation, "success", duration, zap.Int("count", len(records.Records)))

	// Convert to mentor models
	mentors := make([]*models.Mentor, 0, len(records.Records))
	for i := range records.Records {
		mentor := models.AirtableRecordToMentor(records.Records[i])
		mentors = append(mentors, mentor)
	}

	sort.Slice(mentors, func(i, j int) bool {
		return mentors[i].SortOrder < mentors[j].SortOrder
	})

	return mentors, nil
}

// GetMentorByID fetches a mentor by numeric ID
func (c *Client) GetMentorByID(ctx context.Context, id int) (*models.Mentor, error) {
	mentors, err := c.GetAllMentors(ctx)
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
func (c *Client) GetMentorBySlug(ctx context.Context, slug string) (*models.Mentor, error) {
	mentors, err := c.GetAllMentors(ctx)
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
func (c *Client) GetMentorByRecordID(ctx context.Context, recordID string) (*models.Mentor, error) {
	// Just fetch all mentors and filter - simpler than using the API filter
	mentors, err := c.GetAllMentors(ctx)
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

// UpdateMentor updates a mentor record in Airtable with retry logic
func (c *Client) UpdateMentor(ctx context.Context, recordID string, updates map[string]interface{}) error {
	if c.workOffline {
		logger.Info("Skipping Airtable update in offline mode", zap.String("record_id", recordID))
		return nil
	}

	start := time.Now()
	operation := "updateMentor"

	// Use retry logic with context timeout
	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	err := retry.Do(retryCtx, retryConfig, operation, func() error {
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
		if err != nil {
			return fmt.Errorf("failed to update mentor: %w", err)
		}

		return nil
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall(ctx, "airtable", operation, "success", duration, zap.String("record_id", recordID))

	return nil
}

// UpdateMentorImage updates a mentor's profile image
func (c *Client) UpdateMentorImage(ctx context.Context, recordID, imageURL string) error {
	updates := map[string]interface{}{
		"Image_Attachment": []map[string]string{{"url": imageURL}},
	}

	return c.UpdateMentor(ctx, recordID, updates)
}

// CreateMentor creates a new mentor record in Airtable with retry logic
// Returns: recordID (Airtable rec*), mentorID (numeric ID), error
func (c *Client) CreateMentor(ctx context.Context, fields map[string]interface{}) (string, int, error) {
	if c.workOffline {
		logger.Info("Skipping mentor creation in offline mode")
		return "rec_dev_test", 9999, nil
	}

	start := time.Now()
	operation := "createMentor"

	// Use retry logic with context timeout
	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	var recordID string
	var mentorID int

	err := retry.Do(retryCtx, retryConfig, operation, func() error {
		table := c.client.GetTable(c.baseID, MentorsTableName)

		records := &airtable.Records{
			Records: []*airtable.Record{
				{
					Fields: fields,
				},
			},
		}

		createdRecords, err := table.AddRecords(records)
		if err != nil {
			return fmt.Errorf("failed to create mentor: %w", err)
		}

		if len(createdRecords.Records) == 0 {
			return fmt.Errorf("no record returned from Airtable")
		}

		createdRecord := createdRecords.Records[0]
		recordID = createdRecord.ID

		// Extract mentor ID from created record fields
		switch id := createdRecord.Fields["Id"].(type) {
		case float64:
			mentorID = int(id)
		case int:
			mentorID = id
		default:
			return fmt.Errorf("mentor ID not found in created record")
		}

		return nil
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return "", 0, err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall(ctx, "airtable", operation, "success", duration,
		zap.String("record_id", recordID),
		zap.Int("mentor_id", mentorID))

	return recordID, mentorID, nil
}

// CreateClientRequest creates a new client request in Airtable with retry logic
// Returns: recordID (Airtable rec*), error
func (c *Client) CreateClientRequest(ctx context.Context, req *models.ClientRequest) (string, error) {
	if c.workOffline {
		logger.Info("Skipping client request creation in offline mode")
		return "rec_dev_test_request", nil
	}

	start := time.Now()
	operation := "createClientRequest"

	// Use retry logic with context timeout
	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	var recordID string

	err := retry.Do(retryCtx, retryConfig, operation, func() error {
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

		createdRecords, err := table.AddRecords(records)
		if err != nil {
			return fmt.Errorf("failed to create client request: %w", err)
		}

		if len(createdRecords.Records) == 0 {
			return fmt.Errorf("no record returned from Airtable")
		}

		recordID = createdRecords.Records[0].ID

		return nil
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return "", err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall(ctx, "airtable", operation, "success", duration, zap.String("record_id", recordID))

	return recordID, nil
}

// GetAllTags fetches all tags from Airtable with circuit breaker protection
func (c *Client) GetAllTags(ctx context.Context) (map[string]string, error) {
	if c.workOffline {
		return c.getTestTags(), nil
	}

	// Execute through circuit breaker with fallback
	return circuitbreaker.ExecuteWithFallback(
		c.circuitBreaker,
		func() (map[string]string, error) {
			return c.fetchAllTags(ctx)
		},
		func() (map[string]string, error) {
			// Fallback: return empty map
			logger.Warn("Circuit breaker open for Airtable, returning empty tags map")
			return make(map[string]string), nil
		},
	)
}

// fetchAllTags performs the actual Airtable API call with retry logic
func (c *Client) fetchAllTags(ctx context.Context) (map[string]string, error) {
	start := time.Now()
	operation := "getAllTags"

	// Use retry logic with context timeout
	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	records, err := retry.DoWithResult(retryCtx, retryConfig, operation, func() (*airtable.Records, error) {
		table := c.client.GetTable(c.baseID, TagsTableName)

		// Fetch ALL records using manual pagination
		var allTagRecords []*airtable.Record
		offset := ""

		for {
			query := table.GetRecords().
				PageSize(100) // Maximum page size to minimize API requests

			// Add offset for subsequent pages
			if offset != "" {
				query = query.WithOffset(offset)
			}

			records, err := query.Do()
			if err != nil {
				return nil, fmt.Errorf("failed to fetch tags: %w", err)
			}

			// Append records from this page
			allTagRecords = append(allTagRecords, records.Records...)

			// Check if there are more pages
			if records.Offset == "" {
				break
			}
			offset = records.Offset
		}

		// Return all records in Records wrapper
		return &airtable.Records{
			Records: allTagRecords,
		}, nil
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return nil, err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall(ctx, "airtable", operation, "success", duration, zap.Int("count", len(records.Records)))

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

// GetMentorByEmail fetches a mentor by email address
func (c *Client) GetMentorByEmail(ctx context.Context, email string) (*models.Mentor, error) {
	if c.workOffline {
		// Return test mentor with matching email and eligible status
		testMentors := c.getTestMentors()
		if len(testMentors) > 0 {
			testMentors[0].AirtableID = "rec_test_mentor"
			testMentors[0].Status = "active"
			return testMentors[0], nil
		}
		return nil, fmt.Errorf("mentor not found")
	}

	start := time.Now()
	operation := "getMentorByEmail"

	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	records, err := retry.DoWithResult(retryCtx, retryConfig, operation, func() (*airtable.Records, error) {
		table := c.client.GetTable(c.baseID, MentorsTableName)

		// Filter by email AND eligible status (active or inactive)
		// This handles duplicate profiles where only one is valid
		filterFormula := fmt.Sprintf("AND({Email} = '%s', OR({Status} = 'active', {Status} = 'inactive'))", email)

		query := table.GetRecords().
			WithFilterFormula(filterFormula).
			PageSize(1).
			ReturnFields(
				"Id", "Alias", "Name", "Email", "JobTitle", "Workplace",
				"Details", "About", "Experience", "Price", "Status",
				"AuthToken", "Calendly Url", "MentorLoginToken", "MentorLoginTokenExp",
			)

		return query.Do()
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return nil, err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()

	if len(records.Records) == 0 {
		return nil, fmt.Errorf("mentor with email %s not found", email)
	}

	mentor := models.AirtableRecordToMentor(records.Records[0])
	return mentor, nil
}

// GetMentorByLoginToken fetches a mentor by login token
func (c *Client) GetMentorByLoginToken(ctx context.Context, token string) (*models.Mentor, string, time.Time, error) {
	if c.workOffline {
		testMentors := c.getTestMentors()
		if len(testMentors) > 0 {
			testMentors[0].Status = "active"
			return testMentors[0], token, time.Now().Add(15 * time.Minute), nil
		}
		return nil, "", time.Time{}, fmt.Errorf("mentor not found")
	}

	start := time.Now()
	operation := "getMentorByLoginToken"

	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	records, err := retry.DoWithResult(retryCtx, retryConfig, operation, func() (*airtable.Records, error) {
		table := c.client.GetTable(c.baseID, MentorsTableName)

		query := table.GetRecords().
			WithFilterFormula(fmt.Sprintf("{MentorLoginToken} = '%s'", token)).
			PageSize(1).
			ReturnFields(
				"Id", "Alias", "Name", "Email", "JobTitle", "Workplace",
				"Details", "About", "Experience", "Price", "Status",
				"AuthToken", "Calendly Url", "MentorLoginToken", "MentorLoginTokenExp",
			)

		return query.Do()
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return nil, "", time.Time{}, err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()

	if len(records.Records) == 0 {
		return nil, "", time.Time{}, fmt.Errorf("mentor with token not found")
	}

	record := records.Records[0]
	mentor := models.AirtableRecordToMentor(record)

	// Extract token and expiration
	storedToken := ""
	if t, ok := record.Fields["MentorLoginToken"].(string); ok {
		storedToken = t
	}

	var tokenExp time.Time
	if exp, ok := record.Fields["MentorLoginTokenExp"].(string); ok && exp != "" {
		parsedTime, parseErr := time.Parse(time.RFC3339, exp)
		if parseErr == nil {
			tokenExp = parsedTime
		}
	}

	return mentor, storedToken, tokenExp, nil
}

// SetMentorLoginToken sets the login token for a mentor
func (c *Client) SetMentorLoginToken(ctx context.Context, recordID, token string, expiration time.Time) error {
	if c.workOffline {
		logger.Info("Skipping login token update in offline mode", zap.String("record_id", recordID))
		return nil
	}

	updates := map[string]interface{}{
		"MentorLoginToken":    token,
		"MentorLoginTokenExp": expiration.Format(time.RFC3339),
	}

	return c.UpdateMentor(ctx, recordID, updates)
}

// ClearMentorLoginToken clears the login token for a mentor
func (c *Client) ClearMentorLoginToken(ctx context.Context, recordID string) error {
	if c.workOffline {
		logger.Info("Skipping login token clear in offline mode", zap.String("record_id", recordID))
		return nil
	}

	updates := map[string]interface{}{
		"MentorLoginToken":    "",
		"MentorLoginTokenExp": nil,
	}

	return c.UpdateMentor(ctx, recordID, updates)
}

// GetClientRequestsByMentor fetches all client requests for a specific mentor
func (c *Client) GetClientRequestsByMentor(ctx context.Context, mentorAirtableID string, statuses []models.RequestStatus) ([]*models.MentorClientRequest, error) {
	if c.workOffline {
		return c.getTestClientRequests(), nil
	}

	start := time.Now()
	operation := "getClientRequestsByMentor"

	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	// Build status filter
	statusFilters := make([]string, 0, len(statuses))
	for _, s := range statuses {
		statusFilters = append(statusFilters, fmt.Sprintf("{Status} = '%s'", s))
	}
	statusFormula := "OR(" + joinStrings(statusFilters, ", ") + ")"
	filterFormula := fmt.Sprintf("AND({Mentor Id}='%s', %s)", mentorAirtableID, statusFormula)

	records, err := retry.DoWithResult(retryCtx, retryConfig, operation, func() (*airtable.Records, error) {
		table := c.client.GetTable(c.baseID, ClientRequestsTableName)

		var allRecords []*airtable.Record
		offset := ""

		for {
			query := table.GetRecords().
				WithFilterFormula(filterFormula).
				PageSize(100).
				WithSort(struct {
					FieldName string
					Direction string
				}{FieldName: "Created Time", Direction: "asc"}).
				ReturnFields(
					"Email", "Name", "Telegram", "Description", "Level",
					"Status", "Created Time", "Last Modified Time", "Last Status Change",
					"Scheduled At", "Review", "Review2", "ReviewFormUrl", "Mentor",
					"DeclineReason", "DeclineComment",
				)

			if offset != "" {
				query = query.WithOffset(offset)
			}

			recs, err := query.Do()
			if err != nil {
				return nil, fmt.Errorf("failed to fetch client requests: %w", err)
			}

			allRecords = append(allRecords, recs.Records...)

			if recs.Offset == "" {
				break
			}
			offset = recs.Offset
		}

		return &airtable.Records{Records: allRecords}, nil
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return nil, err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall(ctx, "airtable", operation, "success", duration, zap.Int("count", len(records.Records)))

	requests := make([]*models.MentorClientRequest, 0, len(records.Records))
	for _, record := range records.Records {
		requests = append(requests, models.AirtableRecordToMentorClientRequest(record))
	}

	return requests, nil
}

// GetClientRequestByID fetches a single client request by ID
func (c *Client) GetClientRequestByID(ctx context.Context, recordID string) (*models.MentorClientRequest, error) {
	if c.workOffline {
		requests := c.getTestClientRequests()
		if len(requests) > 0 {
			requests[0].ID = recordID
			return requests[0], nil
		}
		return nil, fmt.Errorf("request not found")
	}

	start := time.Now()
	operation := "getClientRequestByID"

	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	record, err := retry.DoWithResult(retryCtx, retryConfig, operation, func() (*airtable.Record, error) {
		table := c.client.GetTable(c.baseID, ClientRequestsTableName)
		return table.GetRecord(recordID)
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return nil, err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()

	return models.AirtableRecordToMentorClientRequest(record), nil
}

// UpdateClientRequestStatus updates the status of a client request
func (c *Client) UpdateClientRequestStatus(ctx context.Context, recordID string, status models.RequestStatus) error {
	if c.workOffline {
		logger.Info("Skipping client request status update in offline mode", zap.String("record_id", recordID))
		return nil
	}

	start := time.Now()
	operation := "updateClientRequestStatus"

	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	err := retry.Do(retryCtx, retryConfig, operation, func() error {
		table := c.client.GetTable(c.baseID, ClientRequestsTableName)

		records := &airtable.Records{
			Records: []*airtable.Record{
				{
					ID: recordID,
					Fields: map[string]interface{}{
						"Status":             string(status),
						"Last Status Change": time.Now().Format(time.RFC3339),
					},
				},
			},
		}

		_, err := table.UpdateRecordsPartial(records)
		if err != nil {
			return fmt.Errorf("failed to update client request status: %w", err)
		}

		return nil
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall(ctx, "airtable", operation, "success", duration, zap.String("record_id", recordID))

	return nil
}

// UpdateClientRequestDecline updates a client request with decline info
func (c *Client) UpdateClientRequestDecline(ctx context.Context, recordID string, reason models.DeclineReason, comment string) error {
	if c.workOffline {
		logger.Info("Skipping client request decline update in offline mode", zap.String("record_id", recordID))
		return nil
	}

	start := time.Now()
	operation := "updateClientRequestDecline"

	retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	retryConfig := retry.AirtableConfig()

	err := retry.Do(retryCtx, retryConfig, operation, func() error {
		table := c.client.GetTable(c.baseID, ClientRequestsTableName)

		fields := map[string]interface{}{
			"Status":             string(models.StatusDeclined),
			"Last Status Change": time.Now().Format(time.RFC3339),
			"DeclineReason":      string(reason),
		}

		if comment != "" {
			fields["DeclineComment"] = comment
		}

		records := &airtable.Records{
			Records: []*airtable.Record{
				{
					ID:     recordID,
					Fields: fields,
				},
			},
		}

		_, err := table.UpdateRecordsPartial(records)
		if err != nil {
			return fmt.Errorf("failed to update client request decline: %w", err)
		}

		return nil
	})

	duration := metrics.MeasureDuration(start)

	if err != nil {
		metrics.AirtableRequestDuration.WithLabelValues(operation, "error").Observe(duration)
		metrics.AirtableRequestTotal.WithLabelValues(operation, "error").Inc()
		logger.LogAPICall(ctx, "airtable", operation, "error", duration, zap.Error(err))
		return err
	}

	metrics.AirtableRequestDuration.WithLabelValues(operation, "success").Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues(operation, "success").Inc()
	logger.LogAPICall(ctx, "airtable", operation, "success", duration, zap.String("record_id", recordID))

	return nil
}

// getTestClientRequests returns test client requests for offline mode
func (c *Client) getTestClientRequests() []*models.MentorClientRequest {
	now := time.Now()
	return []*models.MentorClientRequest{
		{
			ID:              "rec_test_request_1",
			Email:           "mentee@example.com",
			Name:            "Test Mentee",
			Telegram:        "@testmentee",
			Details:         "I want to learn about Go programming",
			Level:           "Junior",
			CreatedAt:       now.Add(-24 * time.Hour),
			ModifiedAt:      now,
			StatusChangedAt: now,
			Status:          models.StatusPending,
			MentorID:        "rec_test_mentor",
		},
	}
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
