package models

// WebhookPayload represents an Airtable webhook payload
type WebhookPayload struct {
	RecordID string            `json:"recordId"`
	Fields   map[string]interface{} `json:"fields"`
}

// RevalidateNextJSRequest represents a request to trigger Next.js ISR revalidation
type RevalidateNextJSRequest struct {
	Secret string `json:"secret"`
	Slug   string `json:"slug"`
}

// RevalidateNextJSResponse represents the response from Next.js revalidation
type RevalidateNextJSResponse struct {
	Revalidated bool   `json:"revalidated"`
	Error       string `json:"error,omitempty"`
}
