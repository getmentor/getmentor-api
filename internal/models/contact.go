package models

// ContactMentorRequest represents a contact form submission
type ContactMentorRequest struct {
	Name             string `json:"name" binding:"required,min=2,max=100"`
	Email            string `json:"email" binding:"required,email,max=255"`
	Experience       string `json:"experience" binding:"omitempty,oneof=Junior Middle Senior Менеджер 'Менеджер менеджеров' C-level"`
	MentorAirtableID string `json:"mentorAirtableId" binding:"required,startswith=rec"`
	Intro            string `json:"intro" binding:"required,min=10,max=4000"`
	TelegramUsername string `json:"telegramUsername" binding:"required,max=50"`
	RecaptchaToken   string `json:"recaptchaToken" binding:"required,min=20"`
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
