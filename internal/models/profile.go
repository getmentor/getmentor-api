package models

// SaveProfileRequest represents a mentor profile update request
// SECURITY: Max length validation to prevent resource exhaustion attacks
type SaveProfileRequest struct {
	Name         string   `json:"name" binding:"required,max=100"`
	Job          string   `json:"job" binding:"required,max=200"`
	Workplace    string   `json:"workplace" binding:"required,max=200"`
	Experience   string   `json:"experience" binding:"required,max=50"`
	Price        string   `json:"price" binding:"required,max=100"`
	Tags         []string `json:"tags" binding:"required,max=10,dive,max=50"`
	Description  string   `json:"description" binding:"required,max=5000"`
	About        string   `json:"about" binding:"required,max=10000"`
	Competencies string   `json:"competencies" binding:"required,max=5000"`
	CalendarURL  string   `json:"calendarUrl" binding:"omitempty,url,max=500"`
}

// SaveProfileResponse represents the response after updating a profile
type SaveProfileResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// UploadProfilePictureRequest represents a profile picture upload request
type UploadProfilePictureRequest struct {
	Image       string `json:"image" binding:"required"`
	FileName    string `json:"fileName" binding:"required"`
	ContentType string `json:"contentType" binding:"required"`
}

// UploadProfilePictureResponse represents the response after uploading a profile picture
type UploadProfilePictureResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message,omitempty"`
	ImageURL string `json:"imageUrl,omitempty"`
	Error    string `json:"error,omitempty"`
}
