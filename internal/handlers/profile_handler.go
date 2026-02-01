package handlers

import (
	"github.com/getmentor/getmentor-api/internal/services"
)

type ProfileHandler struct {
	service services.ProfileServiceInterface
}

func NewProfileHandler(service services.ProfileServiceInterface) *ProfileHandler {
	return &ProfileHandler{service: service}
}
