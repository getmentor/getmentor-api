package handlers

import (
	"net/http"
	"strconv"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

type MentorHandler struct {
	service *services.MentorService
}

func NewMentorHandler(service *services.MentorService) *MentorHandler {
	return &MentorHandler{service: service}
}

func (h *MentorHandler) GetPublicMentors(c *gin.Context) {
	mentors, err := h.service.GetAllMentors(models.FilterOptions{
		OnlyVisible: true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch mentors"})
		return
	}

	// Convert to public format
	publicMentors := make([]models.PublicMentorResponse, 0, len(mentors))
	for _, mentor := range mentors {
		publicMentors = append(publicMentors, mentor.ToPublicResponse("https://гетментор.рф"))
	}

	c.JSON(http.StatusOK, gin.H{"mentors": publicMentors})
}

func (h *MentorHandler) GetPublicMentorByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	mentor, err := h.service.GetMentorByID(id, models.FilterOptions{OnlyVisible: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mentor not found"})
		return
	}

	publicMentor := mentor.ToPublicResponse("https://гетментор.рф")
	c.JSON(http.StatusOK, publicMentor)
}

func (h *MentorHandler) GetInternalMentors(c *gin.Context) {
	// Parse query params
	forceRefresh := c.Query("force_reset_cache") == "true"
	id := c.Query("id")
	slug := c.Query("slug")
	rec := c.Query("rec")

	// Parse body params
	var body struct {
		OnlyVisible    bool `json:"only_visible"`
		ShowHidden     bool `json:"show_hidden"`
		DropLongFields bool `json:"drop_long_fields"`
	}
	_ = c.ShouldBindJSON(&body)

	opts := models.FilterOptions{
		OnlyVisible:    body.OnlyVisible,
		ShowHidden:     body.ShowHidden,
		DropLongFields: body.DropLongFields,
		ForceRefresh:   forceRefresh,
	}

	// Single mentor lookup
	if id != "" {
		mentorID, err := strconv.Atoi(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
			return
		}
		mentor, err := h.service.GetMentorByID(mentorID, opts)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mentor not found"})
			return
		}
		c.JSON(http.StatusOK, mentor)
		return
	}

	if slug != "" {
		mentor, err := h.service.GetMentorBySlug(slug, opts)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mentor not found"})
			return
		}
		c.JSON(http.StatusOK, mentor)
		return
	}

	if rec != "" {
		mentor, err := h.service.GetMentorByRecordID(rec, opts)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mentor not found"})
			return
		}
		c.JSON(http.StatusOK, mentor)
		return
	}

	// Return all mentors
	mentors, err := h.service.GetAllMentors(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch mentors"})
		return
	}

	c.JSON(http.StatusOK, mentors)
}
