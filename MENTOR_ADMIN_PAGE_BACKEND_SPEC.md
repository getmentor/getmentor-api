# GetMentor API — Mentor Admin Backend Specification

**Go Backend Implementation Specification**

---

## 1. Objective

Implement backend API endpoints to support the **Mentor Admin web interface**, enabling mentors to:

* Authenticate via passwordless login (magic link/token)
* View and manage their mentoring requests
* Update request status according to workflow
* Decline requests with reason

This specification covers the **Go backend implementation** in `getmentor-api`.

---

## 2. Architecture Overview

### 2.1 New Components

```
getmentor-api/
├── internal/
│   ├── handlers/
│   │   └── mentor_auth_handler.go      # Authentication endpoints
│   │   └── mentor_requests_handler.go  # Request management endpoints
│   ├── middleware/
│   │   └── mentor_session.go           # Session validation middleware
│   ├── models/
│   │   └── mentor_session.go           # Session and auth models
│   │   └── client_request.go           # Extended client request model
│   ├── repository/
│   │   └── client_request_repository.go # Extended with new queries
│   └── services/
│       └── mentor_auth_service.go      # Authentication logic
│       └── mentor_requests_service.go  # Request management logic
├── pkg/
│   └── jwt/
│       └── jwt.go                      # JWT token utilities
```

### 2.2 Integration Points

* **Airtable**: Client Requests table (existing), Mentors table (existing)
* **Email**: Yandex Cloud Postbox (via Azure Functions webhook)
* **Frontend**: Next.js mentor admin pages

---

## 3. Authentication System

### 3.1 Flow Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Passwordless Authentication Flow                   │
└─────────────────────────────────────────────────────────────────────┘

1. Mentor enters email on /mentor/login
   │
   ▼
2. Frontend: POST /api/v1/auth/mentor/request-login
   │
   ▼
3. Backend validates email exists in Mentors table
   │
   ▼
4. Backend generates login token, stores in Airtable (MentorLoginToken field)
   │
   ▼
5. Backend triggers webhook → Azure Function sends email with magic link
   │
   ▼
6. Mentor clicks link → /mentor/auth/callback?token=xxx
   │
   ▼
7. Frontend: POST /api/v1/auth/mentor/verify
   │
   ▼
8. Backend validates token, clears it, creates JWT session
   │
   ▼
9. Backend sets HttpOnly cookie with JWT
   │
   ▼
10. Frontend redirects to /mentor (authenticated)
```

### 3.2 Token Strategy

**Login Token** (one-time, stored in Airtable):
* Format: `mtk_{random_32_chars}_{timestamp}`
* TTL: 15 minutes
* Single-use: cleared after successful verification
* Storage: `MentorLoginToken` field in Mentors table

**Session Token** (JWT in HttpOnly cookie):
* Algorithm: HS256
* TTL: 24 hours
* Payload: `{ mentor_id, airtable_id, email, exp, iat }`
* Cookie name: `mentor_session`
* Cookie flags: `HttpOnly`, `Secure` (production), `SameSite=Lax`

---

## 4. Data Models

### 4.1 Airtable Schema Changes

**Mentors Table** — Add fields:

| Field | Type | Description |
|-------|------|-------------|
| `MentorLoginToken` | Single line text | One-time login token |
| `MentorLoginTokenExp` | Date/time | Token expiration |
| `TelegramChatId` | Single line text | (May already exist) |

**Client Requests Table** — Existing fields used:

| Field | Type | Description |
|-------|------|-------------|
| `Name` | Single line text | Mentee name |
| `Email` | Email | Mentee email |
| `Telegram` | Single line text | Mentee telegram handle |
| `Description` | Long text | Request details |
| `Level` | Single select | Mentee experience level |
| `Status` | Single select | pending/contacted/working/done/declined/unavailable |
| `Created Time` | Created time | Auto-generated |
| `Last Modified Time` | Last modified time | Auto-generated |
| `Last Status Change` | Date/time | Manual update on status change |
| `Scheduled At` | Date/time | Scheduled meeting time |
| `Mentor` | Link to Mentors | Linked mentor record |
| `Review` | Long text | Mentee review |
| `ReviewFormUrl` | URL | Review form link |
| `DeclineReason` | Single select | (New) Reason for decline |
| `DeclineComment` | Long text | (New) Optional decline comment |

### 4.2 Go Models

```go
// internal/models/mentor_session.go

// MentorSession represents an authenticated mentor session
type MentorSession struct {
    MentorID   int    `json:"mentor_id"`
    AirtableID string `json:"airtable_id"`
    Email      string `json:"email"`
    Name       string `json:"name"`
    ExpiresAt  int64  `json:"exp"`
    IssuedAt   int64  `json:"iat"`
}

// RequestLoginRequest is the payload for requesting a login token
type RequestLoginRequest struct {
    Email string `json:"email" binding:"required,email,max=255"`
}

// RequestLoginResponse is returned after requesting login
type RequestLoginResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message,omitempty"`
}

// VerifyLoginRequest is the payload for verifying a login token
type VerifyLoginRequest struct {
    Token string `json:"token" binding:"required,min=20,max=100"`
}

// VerifyLoginResponse is returned after successful verification
type VerifyLoginResponse struct {
    Success bool           `json:"success"`
    Session *MentorSession `json:"session,omitempty"`
    Error   string         `json:"error,omitempty"`
}
```

```go
// internal/models/client_request.go

// RequestStatus represents the status of a client request
type RequestStatus string

const (
    StatusPending     RequestStatus = "pending"
    StatusContacted   RequestStatus = "contacted"
    StatusWorking     RequestStatus = "working"
    StatusDone        RequestStatus = "done"
    StatusDeclined    RequestStatus = "declined"
    StatusUnavailable RequestStatus = "unavailable"
)

// ActiveStatuses are statuses shown on the active requests page
var ActiveStatuses = []RequestStatus{StatusPending, StatusContacted, StatusWorking}

// PastStatuses are statuses shown on the past requests page
var PastStatuses = []RequestStatus{StatusDone, StatusDeclined, StatusUnavailable}

// DeclineReason represents predefined decline reasons
type DeclineReason string

const (
    DeclineNoTime       DeclineReason = "no_time"
    DeclineTopicMismatch DeclineReason = "topic_mismatch"
    DeclineHelpingOthers DeclineReason = "helping_others"
    DeclineOnBreak       DeclineReason = "on_break"
    DeclineOther         DeclineReason = "other"
)

// ClientRequest represents a mentee's request to a mentor
type ClientRequest struct {
    ID              string        `json:"id"`               // Airtable record ID
    Email           string        `json:"email"`
    Name            string        `json:"name"`
    Telegram        string        `json:"telegram"`
    Details         string        `json:"details"`          // Description field
    Level           string        `json:"level"`
    CreatedAt       time.Time     `json:"createdAt"`
    ModifiedAt      time.Time     `json:"modifiedAt"`
    StatusChangedAt time.Time     `json:"statusChangedAt"`
    ScheduledAt     *time.Time    `json:"scheduledAt"`      // nullable
    Review          *string       `json:"review"`           // nullable
    ReviewURL       *string       `json:"reviewUrl"`        // nullable
    Status          RequestStatus `json:"status"`
    MentorID        string        `json:"mentorId"`         // Airtable record ID
    DeclineReason   *string       `json:"declineReason"`    // nullable
    DeclineComment  *string       `json:"declineComment"`   // nullable
}

// UpdateStatusRequest is the payload for updating request status
type UpdateStatusRequest struct {
    Status RequestStatus `json:"status" binding:"required,oneof=pending contacted working done declined unavailable"`
}

// DeclineRequestPayload is the payload for declining a request
type DeclineRequestPayload struct {
    Reason  DeclineReason `json:"reason" binding:"required,oneof=no_time topic_mismatch helping_others on_break other"`
    Comment string        `json:"comment" binding:"max=1000"`
}

// ClientRequestsResponse is the response for listing requests
type ClientRequestsResponse struct {
    Requests []ClientRequest `json:"requests"`
    Total    int             `json:"total"`
}
```

---

## 5. API Endpoints

### 5.1 Authentication Endpoints

#### POST /api/v1/auth/mentor/request-login

Request a login token to be sent via email.

**Request:**
```json
{
    "email": "mentor@example.com"
}
```

**Response (200):**
```json
{
    "success": true,
    "message": "Ссылка для входа отправлена на вашу почту"
}
```

**Response (400 - validation):**
```json
{
    "error": "Validation failed",
    "details": [{"field": "email", "message": "Invalid email format"}]
}
```

**Response (404 - mentor not found):**
```json
{
    "success": false,
    "message": "Ментор с таким email не найден"
}
```

**Implementation Notes:**
* Validate email format
* Query Mentors table by Email field
* If not found, return 404 (don't reveal existence for security — optionally always return 200)
* Generate login token: `mtk_{crypto/rand 32 chars}_{unix_timestamp}`
* Calculate expiration: `now + 15 minutes`
* Update mentor record with `MentorLoginToken` and `MentorLoginTokenExp`
* Trigger webhook to send email (or call Azure Function directly)
* Rate limit: 2 requests per email per 5 minutes

---

#### POST /api/v1/auth/mentor/verify

Verify login token and create session.

**Request:**
```json
{
    "token": "mtk_abc123...xyz_1704067200"
}
```

**Response (200):**
```json
{
    "success": true,
    "session": {
        "mentor_id": 123,
        "airtable_id": "recABC123",
        "email": "mentor@example.com",
        "name": "Иван Иванов",
        "exp": 1704153600,
        "iat": 1704067200
    }
}
```

**Response (401 - invalid/expired token):**
```json
{
    "success": false,
    "error": "Недействительный или просроченный токен"
}
```

**Implementation Notes:**
* Query Mentors table by `MentorLoginToken` field
* Check if token matches and `MentorLoginTokenExp > now`
* If invalid: return 401
* If valid:
  * Clear `MentorLoginToken` and `MentorLoginTokenExp` in Airtable
  * Generate JWT with mentor data
  * Set HttpOnly cookie: `mentor_session={jwt}; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=86400`
  * Return session data

---

#### POST /api/v1/auth/mentor/logout

Clear session cookie.

**Request:** Empty body

**Response (200):**
```json
{
    "success": true
}
```

**Implementation Notes:**
* Clear cookie: `mentor_session=; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=0`
* No authentication required (idempotent)

---

### 5.2 Request Management Endpoints

All endpoints require valid `mentor_session` cookie.

#### GET /api/v1/mentor/requests

Get mentor's requests filtered by group.

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `group` | string | Yes | `active` or `past` |

**Response (200):**
```json
{
    "requests": [
        {
            "id": "recABC123",
            "email": "mentee@example.com",
            "name": "Пётр Петров",
            "telegram": "@petrov",
            "details": "Хочу разобраться в микросервисах...",
            "level": "Middle",
            "createdAt": "2024-01-15T10:30:00Z",
            "modifiedAt": "2024-01-16T09:00:00Z",
            "statusChangedAt": "2024-01-16T09:00:00Z",
            "scheduledAt": null,
            "review": null,
            "reviewUrl": null,
            "status": "pending",
            "mentorId": "recMENTOR123"
        }
    ],
    "total": 3
}
```

**Response (401 - not authenticated):**
```json
{
    "error": "Unauthorized"
}
```

**Implementation Notes:**
* Extract mentor's Airtable ID from JWT session
* Query Client Requests where `Mentor` linked record matches mentor's ID
* Filter by status:
  * `active`: pending, contacted, working
  * `past`: done, declined, unavailable
* Sort by `Created Time` ascending
* Return all matching requests (no server-side pagination)

---

#### GET /api/v1/mentor/requests/:id

Get single request details.

**Path Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Airtable record ID (rec...) |

**Response (200):**
```json
{
    "id": "recABC123",
    "email": "mentee@example.com",
    "name": "Пётр Петров",
    "telegram": "@petrov",
    "details": "Full request details...",
    "level": "Middle",
    "createdAt": "2024-01-15T10:30:00Z",
    "modifiedAt": "2024-01-16T09:00:00Z",
    "statusChangedAt": "2024-01-16T09:00:00Z",
    "scheduledAt": "2024-01-20T14:00:00Z",
    "review": "Отличный ментор!",
    "reviewUrl": "https://forms.google.com/...",
    "status": "done",
    "mentorId": "recMENTOR123"
}
```

**Response (404):**
```json
{
    "error": "Request not found"
}
```

**Response (403 - request belongs to different mentor):**
```json
{
    "error": "Access denied"
}
```

**Implementation Notes:**
* Fetch request by Airtable record ID
* Verify `Mentor` linked record matches authenticated mentor
* Return full request data

---

#### POST /api/v1/mentor/requests/:id/status

Update request status.

**Path Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Airtable record ID |

**Request:**
```json
{
    "status": "contacted"
}
```

**Response (200):**
```json
{
    "id": "recABC123",
    "status": "contacted",
    "statusChangedAt": "2024-01-17T12:00:00Z",
    ...
}
```

**Response (400 - invalid transition):**
```json
{
    "error": "Invalid status transition",
    "details": "Cannot transition from 'done' to 'contacted'"
}
```

**Implementation Notes:**
* Validate status transition (see workflow below)
* Update Airtable: `Status` field and `Last Status Change` field
* Return updated request

**Status Workflow Validation:**
```
pending    → contacted, declined
contacted  → working, declined
working    → done, declined
done       → (terminal - no transitions)
declined   → (terminal - no transitions)
unavailable → (terminal - no transitions)
```

---

#### POST /api/v1/mentor/requests/:id/decline

Decline request with reason.

**Path Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Airtable record ID |

**Request:**
```json
{
    "reason": "no_time",
    "comment": "К сожалению, сейчас очень загружен"
}
```

**Response (200):**
```json
{
    "id": "recABC123",
    "status": "declined",
    "statusChangedAt": "2024-01-17T12:00:00Z",
    ...
}
```

**Response (400 - cannot decline):**
```json
{
    "error": "Cannot decline request",
    "details": "Request with status 'done' cannot be declined"
}
```

**Implementation Notes:**
* Validate current status allows decline (not `done`)
* Update Airtable fields:
  * `Status` = "declined"
  * `Last Status Change` = now
  * `DeclineReason` = reason value
  * `DeclineComment` = comment (if provided)
* Optionally trigger notification to mentee (webhook)

---

## 6. Middleware

### 6.1 Session Middleware

```go
// internal/middleware/mentor_session.go

// MentorSessionMiddleware validates JWT session cookie and adds session to context
func MentorSessionMiddleware(jwtSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        cookie, err := c.Cookie("mentor_session")
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
            c.Abort()
            return
        }

        session, err := jwt.ValidateToken(cookie, jwtSecret)
        if err != nil {
            // Clear invalid cookie
            c.SetCookie("mentor_session", "", -1, "/", "", true, true)
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
            c.Abort()
            return
        }

        // Add session to context
        c.Set("mentor_session", session)
        c.Next()
    }
}

// GetMentorSession extracts session from context
func GetMentorSession(c *gin.Context) (*models.MentorSession, error) {
    val, exists := c.Get("mentor_session")
    if !exists {
        return nil, errors.New("session not found in context")
    }
    session, ok := val.(*models.MentorSession)
    if !ok {
        return nil, errors.New("invalid session type")
    }
    return session, nil
}
```

---

## 7. Service Layer

### 7.1 MentorAuthService Interface

```go
// internal/services/mentor_auth_service.go

type MentorAuthServiceInterface interface {
    RequestLogin(ctx context.Context, email string) (*models.RequestLoginResponse, error)
    VerifyLogin(ctx context.Context, token string) (*models.MentorSession, error)
}

type MentorAuthService struct {
    mentorRepo    repository.MentorRepositoryInterface
    config        *config.Config
    emailNotifier EmailNotifierInterface  // For sending login emails
}
```

### 7.2 MentorRequestsService Interface

```go
// internal/services/mentor_requests_service.go

type MentorRequestsServiceInterface interface {
    GetRequests(ctx context.Context, mentorAirtableID string, group string) ([]models.ClientRequest, error)
    GetRequestByID(ctx context.Context, mentorAirtableID string, requestID string) (*models.ClientRequest, error)
    UpdateStatus(ctx context.Context, mentorAirtableID string, requestID string, newStatus models.RequestStatus) (*models.ClientRequest, error)
    DeclineRequest(ctx context.Context, mentorAirtableID string, requestID string, payload *models.DeclineRequestPayload) (*models.ClientRequest, error)
}

type MentorRequestsService struct {
    requestRepo repository.ClientRequestRepositoryInterface
    config      *config.Config
}
```

---

## 8. Repository Layer

### 8.1 Extended Client Request Repository

```go
// internal/repository/client_request_repository.go

type ClientRequestRepositoryInterface interface {
    // Existing
    Create(ctx context.Context, req *models.ClientRequest) (string, error)

    // New methods
    GetByMentor(ctx context.Context, mentorAirtableID string, statuses []models.RequestStatus) ([]models.ClientRequest, error)
    GetByID(ctx context.Context, id string) (*models.ClientRequest, error)
    UpdateStatus(ctx context.Context, id string, status models.RequestStatus) error
    UpdateDecline(ctx context.Context, id string, reason models.DeclineReason, comment string) error
}
```

### 8.2 Extended Mentor Repository

```go
// internal/repository/mentor_repository.go

// Add methods:
GetByEmail(ctx context.Context, email string) (*models.Mentor, error)
GetByLoginToken(ctx context.Context, token string) (*models.Mentor, error)
SetLoginToken(ctx context.Context, airtableID string, token string, exp time.Time) error
ClearLoginToken(ctx context.Context, airtableID string) error
```

---

## 9. Configuration

### 9.1 New Environment Variables

```bash
# JWT Configuration
JWT_SECRET=<random-64-char-string>          # REQUIRED - Secret for signing JWTs
JWT_ISSUER=getmentor-api                    # Optional - JWT issuer claim

# Session Configuration
SESSION_TTL_HOURS=24                        # Session duration (default: 24)
LOGIN_TOKEN_TTL_MINUTES=15                  # Login token duration (default: 15)

# Cookie Configuration
COOKIE_DOMAIN=getmentor.dev                 # Cookie domain (production)
COOKIE_SECURE=true                          # Secure flag (true in production)

# Email Webhook (for login notifications)
LOGIN_EMAIL_WEBHOOK_URL=<azure-function-url>  # Webhook to trigger login email
```

### 9.2 Config Struct Extension

```go
// config/config.go

type Config struct {
    // ... existing fields ...

    // JWT
    JWTSecret  string `mapstructure:"JWT_SECRET" validate:"required,min=32"`
    JWTIssuer  string `mapstructure:"JWT_ISSUER"`

    // Session
    SessionTTLHours      int `mapstructure:"SESSION_TTL_HOURS"`
    LoginTokenTTLMinutes int `mapstructure:"LOGIN_TOKEN_TTL_MINUTES"`

    // Cookie
    CookieDomain string `mapstructure:"COOKIE_DOMAIN"`
    CookieSecure bool   `mapstructure:"COOKIE_SECURE"`

    // Webhooks
    LoginEmailWebhookURL string `mapstructure:"LOGIN_EMAIL_WEBHOOK_URL"`
}
```

---

## 10. Route Registration

```go
// cmd/api/main.go

// Authentication routes (public)
auth := router.Group("/api/v1/auth/mentor")
{
    auth.POST("/request-login", authRateLimiter.Middleware(), mentorAuthHandler.RequestLogin)
    auth.POST("/verify", mentorAuthHandler.VerifyLogin)
    auth.POST("/logout", mentorAuthHandler.Logout)
}

// Mentor admin routes (protected)
mentor := router.Group("/api/v1/mentor")
mentor.Use(middleware.MentorSessionMiddleware(cfg.JWTSecret))
{
    mentor.GET("/requests", mentorRequestsHandler.GetRequests)
    mentor.GET("/requests/:id", mentorRequestsHandler.GetRequestByID)
    mentor.POST("/requests/:id/status", mentorRequestsHandler.UpdateStatus)
    mentor.POST("/requests/:id/decline", mentorRequestsHandler.DeclineRequest)
}
```

---

## 11. Security Considerations

### 11.1 Authentication Security

* **Login tokens**: Single-use, short TTL (15 min), stored hashed in production
* **JWT secrets**: Minimum 256-bit, rotated periodically
* **Cookie flags**: HttpOnly (no JS access), Secure (HTTPS only), SameSite (CSRF protection)
* **Rate limiting**: Strict limits on login requests (2/5min per email)

### 11.2 Authorization Security

* **Ownership verification**: Always verify request belongs to authenticated mentor
* **Timing-safe comparison**: Use `subtle.ConstantTimeCompare` for token validation
* **No enumeration**: Don't reveal whether email exists (optional — return same response)

### 11.3 Input Validation

* **Email format**: RFC 5322 compliant
* **Token format**: Validate prefix and length
* **Status values**: Whitelist valid values
* **Reason values**: Whitelist valid values
* **Comment length**: Max 1000 characters

### 11.4 Logging

* **Log authentication events**: Login attempts, failures, successes
* **Don't log sensitive data**: Tokens, JWTs, passwords
* **Include request IDs**: For tracing

---

## 12. Email Integration

### 12.1 Login Email Template

Create new email template in `getmentor-func/lib/postbox/templates/`:

**File**: `mentor-login.json`
```json
{
    "subject": "Вход в личный кабинет GetMentor",
    "template": "mentor-login"
}
```

**Template content**:
```
Здравствуйте, {mentor_name}!

Вы запросили вход в личный кабинет ментора на GetMentor.

Нажмите на ссылку ниже, чтобы войти:
{login_url}

Ссылка действительна 15 минут.

Если вы не запрашивали вход, просто проигнорируйте это письмо.

---
GetMentor.dev — Платформа менторства
```

### 12.2 Webhook Payload

```json
{
    "type": "mentor_login",
    "mentor": {
        "email": "mentor@example.com",
        "name": "Иван Иванов"
    },
    "login_url": "https://getmentor.dev/mentor/auth/callback?token=mtk_..."
}
```

---

## 13. Testing Requirements

### 13.1 Unit Tests

* `MentorAuthService`: Token generation, validation, expiration
* `MentorRequestsService`: Status transitions, ownership checks
* `JWT utilities`: Token creation, validation, expiration
* `Middleware`: Cookie parsing, session extraction

### 13.2 Integration Tests

* Full login flow: request → verify → authenticated request
* Request management flow: list → view → update status
* Decline flow with reason

### 13.3 Test Cases

**Authentication:**
* Valid email → token sent
* Invalid email format → validation error
* Non-existent mentor → appropriate response
* Valid token → session created
* Expired token → 401
* Invalid token → 401
* Reused token → 401

**Requests:**
* List active requests → correct filter
* List past requests → correct filter
* Get own request → success
* Get other's request → 403
* Valid status transition → success
* Invalid status transition → 400
* Decline from valid status → success
* Decline from done → 400

---

## 14. Migration Steps

### 14.1 Airtable Changes

1. Add `MentorLoginToken` field to Mentors table (Single line text)
2. Add `MentorLoginTokenExp` field to Mentors table (Date/time)
3. Add `DeclineReason` field to Client Requests table (Single select with values: no_time, topic_mismatch, helping_others, on_break, other)
4. Add `DeclineComment` field to Client Requests table (Long text)
5. Ensure `Last Status Change` field exists in Client Requests table

### 14.2 Environment Setup

1. Generate JWT secret: `openssl rand -base64 64`
2. Add new environment variables to production
3. Update Azure Function to handle `mentor_login` webhook type

### 14.3 Deployment Order

1. Deploy Airtable schema changes
2. Deploy Azure Function updates (email template)
3. Deploy Go API updates
4. Deploy Frontend updates (already done)
5. Test end-to-end flow

---

## 15. Error Codes Reference

| HTTP Code | Error | Description |
|-----------|-------|-------------|
| 400 | Validation failed | Invalid request body or parameters |
| 400 | Invalid status transition | Status workflow violation |
| 400 | Cannot decline request | Attempting to decline done request |
| 401 | Unauthorized | Missing or invalid session |
| 401 | Session expired | JWT expired |
| 403 | Access denied | Request belongs to different mentor |
| 404 | Mentor not found | Email not in Mentors table |
| 404 | Request not found | Invalid request ID |
| 429 | Too many requests | Rate limit exceeded |
| 500 | Internal server error | Unexpected error |

---

## 16. Metrics

Add Prometheus metrics for monitoring:

```go
// Counters
mentor_auth_login_requests_total{status="success|failure"}
mentor_auth_verify_requests_total{status="success|failure|expired|invalid"}
mentor_requests_list_total{group="active|past"}
mentor_requests_status_updates_total{from_status, to_status}
mentor_requests_declines_total{reason}

// Histograms
mentor_auth_login_duration_seconds
mentor_auth_verify_duration_seconds
mentor_requests_list_duration_seconds
```

---

## 17. Summary

This specification defines the backend API for the Mentor Admin interface:

* **5 new endpoints** for authentication and request management
* **Session-based auth** using JWT in HttpOnly cookies
* **Passwordless login** via email magic links
* **Status workflow** enforcement on the backend
* **Ownership verification** for all request operations
* **Integration** with existing Airtable and notification systems

The implementation follows existing patterns in the `getmentor-api` codebase:
* Gin handlers with dependency injection
* Service layer for business logic
* Repository pattern for data access
* Custom error types
* Structured logging and metrics
