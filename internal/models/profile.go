package models

// SaveProfileRequest represents a mentor profile update request
type SaveProfileRequest struct {
	Name         string   `json:"name" binding:"required"`
	Job          string   `json:"job" binding:"required"`
	Workplace    string   `json:"workplace" binding:"required"`
	Experience   string   `json:"experience" binding:"required"`
	Price        string   `json:"price" binding:"required"`
	Tags         []string `json:"tags" binding:"required"`
	Description  string   `json:"description" binding:"required"`
	About        string   `json:"about" binding:"required"`
	Competencies string   `json:"competencies" binding:"required"`
	CalendarURL  string   `json:"calendarUrl"`
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
