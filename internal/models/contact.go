package models

// ContactMentorRequest represents a contact form submission
type ContactMentorRequest struct {
	Name             string `json:"name" binding:"required"`
	Email            string `json:"email" binding:"required,email"`
	Experience       string `json:"experience"`
	MentorAirtableID string `json:"mentorAirtableId" binding:"required"`
	Intro            string `json:"intro" binding:"required"`
	TelegramUsername string `json:"telegramUsername" binding:"required"`
	RecaptchaToken   string `json:"recaptchaToken" binding:"required"`
}

// ContactMentorResponse represents the response after submitting a contact form
type ContactMentorResponse struct {
	Success     bool   `json:"success"`
	CalendarURL string `json:"calendar_url,omitempty"`
	Error       string `json:"error,omitempty"`
}

// ClientRequest represents a client request record in Airtable
type ClientRequest struct {
	Email       string
	Name        string
	Level       string
	MentorID    string // Airtable record ID
	Description string
	Telegram    string
}

// ReCAPTCHAResponse represents Google's ReCAPTCHA verification response
type ReCAPTCHAResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}
