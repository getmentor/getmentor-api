package analytics

const (
	EventMenteeContactSubmitted      = "mentee_contact_submitted"
	EventMentorRegistrationSubmitted = "mentor_registration_submitted"
	EventReviewEligibilityChecked    = "review_eligibility_checked"
	EventReviewSubmitted             = "review_submitted"

	EventMentorAuthLoginRequested = "mentor_auth_login_requested"
	EventMentorAuthLoginVerified  = "mentor_auth_login_verified"
	EventAdminAuthLoginRequested  = "admin_auth_login_requested"
	EventAdminAuthLoginVerified   = "admin_auth_login_verified"

	EventMentorProfileUpdated         = "mentor_profile_updated"
	EventMentorProfilePictureUploaded = "mentor_profile_picture_uploaded"
	EventMentorRequestStatusUpdated   = "mentor_request_status_updated"
	EventMentorRequestDeclined        = "mentor_request_declined"

	EventAdminMentorModerationAction = "admin_mentor_moderation_action"
	EventAdminMentorStatusUpdated    = "admin_mentor_status_updated"
	EventAdminMentorProfileUpdated   = "admin_mentor_profile_updated"
	EventAdminMentorPictureUploaded  = "admin_mentor_picture_uploaded"
)
