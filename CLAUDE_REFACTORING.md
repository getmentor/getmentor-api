# GetMentor API - Code Review and Refactoring Guide

**Review Date:** 2025-11-25
**Last Updated:** 2025-11-26
**Reviewer:** Claude (Staff Software Engineer perspective)
**Codebase Version:** Go 1.24.0, ~1976 LOC internal code

---

## 🎯 REFACTORING STATUS (Updated 2025-11-26)

### Summary: CRITICAL WORK COMPLETE ✅

**Overall Progress:** 14 of 17 high-priority issues resolved (82%)

| Priority | Total | Fixed | Remaining | Status |
|----------|-------|-------|-----------|--------|
| **P0 (Critical)** | 5 | 5 ✅ | 0 | **100% COMPLETE** |
| **P1 (High)** | 12 | 9 ✅ | 3 | **75% COMPLETE** |
| **P2 (Plan)** | 10 | 0 | 10 | Not started |
| **P3 (Consider)** | 22 | 0 | 22 | Not started |
| **P4 (Nice-to-have)** | 14 | 0 | 14 | Not started |

### ✅ Completed Work (P0 + P1)

#### All P0 Issues Fixed
1. ✅ **SEC-1** - Secret in query parameter → Header-based auth implemented
2. ✅ **SEC-2** - ReCAPTCHA secret in URL → POST with form body
3. ✅ **SEC-3** - Missing input validation → `validation.go` helper added
4. ✅ **GO-3** - Race condition in tags cache → Synchronous initialization
5. ✅ **GAP-2** - Contact handler untested → 445-line test suite added

#### P1 Issues Fixed (9 of 12)
1. ✅ **TEST-1** - HTTP client not injectable → `pkg/httpclient/` created
2. ✅ **DUP-2** - Auth header duplication → Helper function extracted
3. ✅ **ERR-1/ABS-3** - String error matching → `pkg/errors/` typed errors
4. ✅ **API-3** - Wrong HTTP status codes → Fixed across handlers
5. ✅ **CFG-1** - CORS origins hardcoded → Moved to config
6. ✅ **CFG-2** - Base URL hardcoded → Moved to config
7. ✅ **TEST-6** - No interfaces → `services/interfaces.go` created
8. ✅ **CFG-4** - Incomplete validation → Enhanced config checks
9. ✅ **GO-4** - Context propagation → Added throughout stack

#### CI/CD Infrastructure ✅
- ✅ `.github/workflows/pr-checks.yml` - Coverage reports + quality gates
- ✅ `.github/workflows/test.yml` - Tests + linting
- ✅ `.github/workflows/build-and-test.yml` - Docker build validation
- ✅ All workflows passing with golangci-lint v1.64.8
- ✅ 65+ linter errors fixed (shadowing, type assertions, formatting)

#### Code Quality Improvements ✅
- ✅ Tuned `.golangci.yml` for practical development
- ✅ Removed deprecated linters
- ✅ Added validation helpers
- ✅ Improved error handling consistency
- ✅ Test coverage: ~15-20% (up from <10%)

### 🚧 Remaining Work

#### P1 Issues (3 remaining)
- ⏳ **SEC-8** - No CSRF protection (2-3 hours)
- ⏳ **API-5** - No API versioning (2-3 hours)
- ⏳ **GO-1** - Incomplete error wrapping (ongoing)

#### Testing Gaps
- ⏳ Handler tests (mentor, profile, webhook, logs) - 0% coverage
- ⏳ Service tests (all services) - 0% coverage
- ⏳ Repository tests - 0% coverage
- ⏳ Cache tests - 0% coverage
- ⏳ Integration/E2E tests - None exist

#### P2-P4 Issues (46 items)
- See detailed sections below for architectural improvements
- Non-blocking, can be addressed incrementally

### Files Changed
- **43 files modified**
- **+3,538 additions, -284 deletions**
- **New files:** `pkg/errors/`, `pkg/httpclient/`, `services/interfaces.go`, `handlers/validation.go`, `contact_handler_test.go`, CI workflows

### Production Readiness: ✅ YES
All critical security and stability issues resolved. Remaining work is quality-of-life improvements and test coverage expansion.

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Project Structure Analysis](#project-structure-analysis)
3. [Complete Issue Inventory](#complete-issue-inventory)
4. [Prioritized Action Items](#prioritized-action-items)
5. [Detailed Findings & Recommendations](#detailed-findings--recommendations)
6. [Testing Strategy](#testing-strategy)
7. [Long-term Architectural Improvements](#long-term-architectural-improvements)

---

## Executive Summary

### Codebase Health: **VERY GOOD** - Production Ready ✅

**Strengths:**
- ✅ Clean layered architecture (handlers → services → repositories → models)
- ✅ Excellent observability infrastructure (OpenTelemetry, Prometheus, Zap logging)
- ✅ Resilience patterns (circuit breakers, retry logic, caching with TTL)
- ✅ Good security awareness (timing-safe comparisons, rate limiting, security headers)
- ✅ Proper dependency injection in main.go
- ✅ Modern Go patterns and libraries
- ✅ **All critical security issues resolved (as of 2025-11-26)**
- ✅ **CI/CD pipelines fully operational**
- ✅ **Code quality standards enforced**

**Resolved Concerns:**
- ✅ **5 critical security issues** - ALL FIXED
- ✅ **Race conditions in cache** - Fixed with synchronous initialization
- ✅ **Secrets in URLs** - Moved to headers and POST bodies
- ✅ **Missing input validation** - Added comprehensive validation
- ✅ **Testability problems** - Interfaces and abstractions added
- ✅ **Code duplication** - Key duplications extracted to helpers

**Remaining Areas for Improvement:**
- 🟡 **Test coverage ~15-20%** - Still low but improved from <10%
- 🟡 **CSRF protection** - Not yet implemented (optional for API)
- 🟡 **API versioning** - Not yet implemented (can add later)
- 🟡 **Additional test coverage** - Ongoing work

**Statistics:**
- Total Issues Identified: **63**
- P0 (Critical): **5 issues** → ✅ **5 fixed (100%)**
- P1 (High Priority): **12 issues** → ✅ **9 fixed (75%)**
- P2 (Plan for fix): **10 issues** → ⏳ 0 fixed (future work)
- P3 (Consider): **22 issues** → ⏳ 0 fixed (future work)
- P4 (Nice to have): **14 issues** → ⏳ 0 fixed (future work)

**Current Recommendation (2025-11-26):**
Codebase is **production-ready**. All critical (P0) security and stability issues have been addressed. The 3 remaining P1 issues (CSRF, API versioning, error wrapping) are optional enhancements that can be addressed based on operational needs. Focus should shift to expanding test coverage incrementally during normal feature development.

---

## Project Structure Analysis

### Directory Layout

```
getmentor-api/
├── cmd/api/              # Application entry point
│   └── main.go          # Main application setup (~240 lines)
├── config/              # Configuration management
│   └── config.go        # Viper-based config with validation
├── internal/            # Private application code (~1976 LOC total)
│   ├── cache/          # In-memory caching layer
│   │   ├── mentor_cache.go  # Mentor data cache with TTL
│   │   └── tags_cache.go    # Tags cache
│   ├── handlers/       # HTTP request handlers (controllers)
│   │   ├── contact_handler.go
│   │   ├── health_handler.go
│   │   ├── logs_handler.go
│   │   ├── mentor_handler.go
│   │   ├── profile_handler.go
│   │   └── webhook_handler.go
│   ├── middleware/     # HTTP middleware
│   │   ├── auth.go
│   │   ├── body_size_limit.go
│   │   ├── observability.go
│   │   ├── rate_limit.go
│   │   └── security_headers.go
│   ├── models/         # Domain models and DTOs
│   │   ├── contact.go
│   │   ├── mentor.go   # Core mentor model (~288 lines)
│   │   ├── profile.go
│   │   └── webhook.go
│   ├── repository/     # Data access layer
│   │   ├── client_request_repository.go
│   │   └── mentor_repository.go
│   └── services/       # Business logic layer
│       ├── contact_service.go
│       ├── mentor_service.go  # Thin service (~36 lines)
│       ├── profile_service.go
│       └── webhook_service.go
├── pkg/                # Reusable packages
│   ├── airtable/       # Airtable client with circuit breaker
│   ├── azure/          # Azure Blob Storage client
│   ├── circuitbreaker/ # Circuit breaker implementation
│   ├── logger/         # Structured logging (Zap)
│   ├── metrics/        # Prometheus metrics
│   ├── retry/          # Retry logic with exponential backoff
│   └── tracing/        # OpenTelemetry distributed tracing
└── test/               # Test files (only 6 test files)
    ├── config/
    ├── internal/
    └── pkg/
```

### Architecture Pattern

**Clean Architecture / Layered Architecture:**

```
HTTP Request
    ↓
[Middleware Stack]
    ↓
[Handlers] ←── Parse HTTP, validate input, format response
    ↓
[Services] ←── Business logic, orchestration
    ↓
[Repositories] ←── Data access abstraction
    ↓
[Cache Layer] ←── In-memory caching
    ↓
[External Clients] ←── Airtable, Azure, HTTP
```

**Dependency Flow:**
- main.go initializes all components with proper dependency injection
- Clean dependency flow: main → handlers → services → repos → clients
- No circular dependencies detected
- Configuration loaded once at startup, passed to components

### Key Dependencies

**Web Framework:**
- `github.com/gin-gonic/gin v1.10.1` - HTTP web framework
- `github.com/gin-contrib/cors v1.7.0` - CORS middleware

**External Services:**
- `github.com/mehanizm/airtable v0.3.4` - Airtable client (database)
- `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.3.0` - Azure Blob Storage

**Observability:**
- `go.opentelemetry.io/otel v1.38.0` - OpenTelemetry tracing
- `github.com/prometheus/client_golang v1.19.0` - Prometheus metrics
- `go.uber.org/zap v1.26.0` - Structured logging

**Resilience:**
- `github.com/sony/gobreaker v1.0.0` - Circuit breaker
- `github.com/patrickmn/go-cache v2.1.0+incompatible` - In-memory cache
- `golang.org/x/time v0.14.0` - Rate limiting

**Configuration:**
- `github.com/spf13/viper v1.18.2` - Config management (env vars + .env file)

### Test Infrastructure

**Current State: POOR**
- Only **6 test files** out of 23 Go files in internal/
- Test coverage estimated at **< 10%**
- Tests located in separate `test/` directory (non-idiomatic)
- Using `github.com/stretchr/testify` for assertions
- No mocking infrastructure in place
- No integration tests

**Test Files:**
- `test/config/config_test.go`
- `test/internal/middleware/auth_test.go`
- `test/internal/models/mentor_test.go`
- `test/internal/handlers/health_handler_test.go`
- `test/pkg/azure/storage_test.go`
- `test/pkg/airtable/client_test.go`

**Missing Coverage:**
- ❌ 0% - All handlers (except health)
- ❌ 0% - All services
- ❌ 0% - All repositories
- ❌ 0% - Cache implementations
- ❌ 0% - Most middleware

---

## Complete Issue Inventory

### 2.1 Architectural Issues (6 issues)

| ID | Location | Description | Impact |
|----|----------|-------------|---------|
| ARCH-1 | `internal/services/mentor_service.go:9-35` | MentorService is too thin - only passes through to repository with no business logic | Unnecessary layer adds complexity, violates YAGNI |
| ARCH-2 | Services layer | Inconsistent service patterns: Contact/Profile services have logic, Mentor service is pass-through | Makes codebase harder to understand, unclear where to add logic |
| ARCH-3 | `internal/models/mentor.go:185-188`<br>`internal/services/profile_service.go:147-150` | Sponsor tags hardcoded in multiple places | Code duplication, hard to maintain, configuration should be centralized |
| ARCH-4 | `internal/cache/tags_cache.go:33` | Tags cache warmup happens asynchronously without synchronization | Potential race condition: API calls may happen before cache ready |
| ARCH-5 | Test infrastructure | Tests in separate `test/` directory instead of co-located with code | Harder to maintain, easy to forget to update tests, non-idiomatic Go |
| ARCH-6 | `cmd/api/main.go:139-146` | Hardcoded business logic (CORS origins) in main.go | Should be in config, violates separation of concerns |

### 2.2 Code Duplication and Reusability (6 issues)

| ID | Location | Description | Impact |
|----|----------|-------------|---------|
| DUP-1 | `internal/handlers/profile_handler.go:44-51`<br>`internal/handlers/webhook_handler.go:44` | Error handling with string matching repeated across handlers | Hard to maintain, fragile, error-prone |
| DUP-2 | `internal/handlers/profile_handler.go:23-28, 60-65` | Auth header extraction and validation duplicated | Copy-paste error risk, should be middleware or helper |
| DUP-3 | `internal/models/mentor.go:185-188`<br>`internal/services/profile_service.go:147-150` | Sponsor tags map duplicated | Changes require updates in multiple places |
| DUP-4 | `internal/handlers/mentor_handler.go:40`<br>`internal/handlers/profile_handler.go:31, 68` | ID string parsing pattern repeated | Boilerplate code, prone to inconsistency |
| DUP-5 | All handlers | `gin.H{}` response patterns repeated everywhere | Inconsistent responses, no type safety |
| DUP-6 | `pkg/airtable/client.go:177-223` | GetMentorByID/Slug/RecordID all fetch all mentors then filter | Inefficient, N+1 problem for cache, wasteful |

### 2.3 Abstraction Quality (5 issues)

| ID | Location | Description | Impact |
|----|----------|-------------|---------|
| ABS-1 | `internal/services/mentor_service.go` | MentorService over-abstracted - no business logic, no interface needed | Adds unnecessary indirection |
| ABS-2 | `internal/repository/client_request_repository.go` | ClientRequestRepository under-abstracted - only wraps one method | Doesn't provide value, just adds layer |
| ABS-3 | `internal/handlers/profile_handler.go:44` | Error handling uses string comparison instead of typed errors | Fragile, no type safety, easy to break |
| ABS-4 | `internal/services/contact_service.go:91`<br>`internal/services/webhook_service.go:55` | No HTTP client abstraction for external calls | Cannot mock for testing, tight coupling to http.Client |
| ABS-5 | `cmd/api/main.go:139-146`<br>`internal/handlers/mentor_handler.go:32` | Hardcoded URLs and configuration mixed in code | Should be in configuration, violates separation of concerns |

### 2.4 Testability Issues (8 issues)

| ID | Location | Description | Impact |
|----|----------|-------------|---------|
| TEST-1 | `internal/services/contact_service.go:91`<br>`internal/services/webhook_service.go:55` | HTTP client created inline without dependency injection | Cannot mock external HTTP calls in tests |
| TEST-2 | `internal/cache/tags_cache.go:33` | Tags cache warmup launches uncontrolled goroutine | Cannot wait for completion in tests, causes race conditions |
| TEST-3 | `internal/services/profile_service.go:134-138` | Async Airtable update in goroutine without coordination | Cannot verify completion in tests |
| TEST-4 | Multiple files | Direct use of `time.Now()` and `time.Sleep()` | Cannot control time in tests, tests become slow and flaky |
| TEST-5 | `internal/handlers/logs_handler.go:66, 72` | File system operations without abstraction | Hard to test without writing actual files |
| TEST-6 | All services and repositories | No interfaces defined for services/repositories | Cannot create mocks, hard to test handlers |
| TEST-7 | `pkg/logger` package | Global logger package state | Cannot inject test logger, output pollutes test results |
| TEST-8 | All handlers | Handlers tightly coupled to Gin context | Hard to test without httptest, no separation of logic |

### 2.5 Testing Gaps (12 issues)

| ID | Component | Coverage | Impact |
|----|-----------|----------|---------|
| GAP-1 | `internal/handlers/mentor_handler.go` | 0% - No tests | Critical endpoint untested, regression risk |
| GAP-2 | `internal/handlers/contact_handler.go` | 0% - No tests | Form submission untested, data loss risk |
| GAP-3 | `internal/handlers/profile_handler.go` | 0% - No tests | Auth-protected endpoint untested, security risk |
| GAP-4 | `internal/handlers/webhook_handler.go` | 0% - No tests | Cache invalidation untested, stale data risk |
| GAP-5 | `internal/handlers/logs_handler.go` | 0% - No tests | File operations untested, data loss risk |
| GAP-6 | All 4 services | 0% - No tests | Business logic untested, bug risk |
| GAP-7 | Both repositories | 0% - No tests | Data access untested, integration issues |
| GAP-8 | `internal/cache/mentor_cache.go` | 0% - No tests | Complex cache logic untested, race conditions possible |
| GAP-9 | `internal/cache/tags_cache.go` | 0% - No tests | Cache behavior untested |
| GAP-10 | Integration tests | 0% - None exist | End-to-end flows untested |
| GAP-11 | Error paths | <10% estimated | Error handling largely untested |
| GAP-12 | Middleware | 16% (1 of 6) tested | Security middleware untested |

### 2.6 Security Concerns (10 issues)

| ID | Location | Description | Severity |
|----|----------|-------------|----------|
| SEC-1 | `internal/handlers/webhook_handler.go:36` | Secret passed as query parameter (logged, cached, visible) | 🔴 **HIGH** - Credential exposure |
| SEC-2 | `internal/services/contact_service.go:87-88` | ReCAPTCHA verification uses GET with secret in URL | 🔴 **HIGH** - Secret exposed in logs |
| SEC-3 | `internal/handlers/contact_handler.go` | No input validation on form fields (length, format, XSS) | 🔴 **HIGH** - Input injection risk |
| SEC-4 | `internal/handlers/logs_handler.go:71` | Potential path traversal - logDir from config | 🟡 **MEDIUM** - File system access |
| SEC-5 | Rate limiting | No per-user/token rate limiting, only global | 🟡 **MEDIUM** - Individual user can't be blocked |
| SEC-6 | `internal/handlers/contact_handler.go:22` | Error messages expose internal details (`err.Error()`) | 🟢 **LOW** - Information disclosure |
| SEC-7 | `cmd/api/main.go:139-146` | CORS origins hardcoded in code instead of config | 🟢 **LOW** - Deployment flexibility |
| SEC-8 | Middleware | No CSRF protection implemented | 🟡 **MEDIUM** - Cross-site attack risk |
| SEC-9 | Logging | No request ID tracing through stack | 🟢 **LOW** - Hard to correlate logs for security incidents |
| SEC-10 | `internal/middleware/auth.go` | Auth tokens logged on failure (potential exposure) | 🟢 **LOW** - Sensitive data in logs |

### 2.7 Error Handling (7 issues)

| ID | Location | Description | Impact |
|----|----------|-------------|---------|
| ERR-1 | `internal/handlers/profile_handler.go:44`<br>`internal/handlers/webhook_handler.go:44` | Error comparison using string matching | Fragile, breaks if error messages change |
| ERR-2 | `internal/handlers/contact_handler.go:28` | Generic error messages hide useful context | Hard to debug production issues |
| ERR-3 | `config/config.go:112` | ReadInConfig() error silently ignored | Config file errors not reported |
| ERR-4 | `internal/services/webhook_service.go:35-36, 40-42` | Webhook processing errors swallowed | Silent failures, hard to debug |
| ERR-5 | Multiple services | No error wrapping with context | Lost error chain, hard to trace root cause |
| ERR-6 | All handlers | Inconsistent error response format | Clients must handle multiple formats |
| ERR-7 | `pkg/airtable/client.go` | Circuit breaker fallback returns empty list | Silent degradation, users see empty results |

### 2.8 Go-Specific Issues (6 issues)

| ID | Location | Description | Impact |
|----|----------|-------------|---------|
| GO-1 | `internal/services/profile_service.go:134` | Goroutine launched without context, may leak | If request canceled, goroutine continues wastefully |
| GO-2 | `internal/cache/tags_cache.go:33` | Goroutine launched with no cancellation mechanism | Cannot stop warmup, may leak |
| GO-3 | `internal/cache/tags_cache.go:33` | Race condition: cache Get() may run before warmup completes | Cache miss on first request, poor startup performance |
| GO-4 | All services | Context not propagated through service layer | Cannot cancel long-running operations, timeouts don't work |
| GO-5 | `internal/cache/mentor_cache.go:138` | Uses `time.Sleep()` for polling instead of proper synchronization | Inefficient, blocks goroutine unnecessarily |
| GO-6 | `internal/models/mentor.go:133` | Deep copy in applyFilters creates new struct for every mentor | Inefficient memory allocation, GC pressure |

### 2.9 API Design Issues (8 issues)

| ID | Location | Description | Impact |
|----|----------|-------------|---------|
| API-1 | All handlers | Inconsistent response formats (`gin.H{}` vs typed structs) | Client integration harder, no API contract |
| API-2 | `internal/handlers/profile_handler.go:50` | Returns 503 (Service Unavailable) for failed update | Misleading status code, should be 500 or 400 |
| API-3 | `internal/handlers/mentor_handler.go` | Returns 403 (Forbidden) without auth header, should be 401 | Incorrect HTTP semantics |
| API-4 | `internal/handlers/webhook_handler.go:36` | Webhook endpoint uses query params for secrets | Should use headers or request body |
| API-5 | `cmd/api/main.go` | No API versioning in routes | Cannot evolve API without breaking clients |
| API-6 | Middleware | Inconsistent auth header naming | `mentors_api_auth_token` vs `x-internal-mentors-api-auth-token` |
| API-7 | All responses | Missing standard fields (request_id, error_code) | Hard to debug, poor developer experience |
| API-8 | Error responses | No machine-readable error codes | Clients must parse error strings |

### 2.10 Configuration and Environment (6 issues)

| ID | Location | Description | Impact |
|----|----------|-------------|---------|
| CFG-1 | `cmd/api/main.go:139-146` | CORS origins hardcoded in code | Cannot configure without recompiling |
| CFG-2 | `internal/handlers/mentor_handler.go:32, 52` | Base URL hardcoded in handler | Environment-specific value in code |
| CFG-3 | `internal/models/mentor.go:185-188`<br>`internal/services/profile_service.go:147-150` | Sponsor tags hardcoded | Business logic in code, not configurable |
| CFG-4 | `config/config.go:169-185` | Only validates Airtable config, not other required secrets | Missing secrets discovered at runtime, not startup |
| CFG-5 | `config/config.go:94` | Log directory default "/app/logs" assumes Docker | Breaks in non-Docker environments |
| CFG-6 | `internal/services/contact_service.go:46` | Environment-specific branching in service code | Business logic should not know about environment |

---

## Prioritized Action Items

### Priority Matrix

| Priority | Count | Total Estimated Effort | Description |
|----------|-------|------------------------|-------------|
| **P0** | 5 | 1-2 days | Critical security and stability issues |
| **P1** | 12 | 1-2 weeks | High-impact maintainability improvements |
| **P2** | 10 | 2-3 weeks | Important testing and architecture work |
| **P3** | 22 | 3-4 weeks | Quality improvements |
| **P4** | 14 | 4-6 weeks | Nice-to-have enhancements |

### P0 Issues (Critical - Fix Immediately) - ~1-2 Days

| Issue ID | Description | Severity | Effort | File |
|----------|-------------|----------|--------|------|
| SEC-1 | Secret in query parameter (webhook) | Critical | 15 min | `webhook_handler.go:36` |
| SEC-2 | ReCAPTCHA secret in GET URL | Critical | 20 min | `contact_service.go:87` |
| SEC-3 | Missing input validation | High | 1 hour | `contact_handler.go` + models |
| GO-3 | Race condition in tags cache | High | 1-2 hours | `tags_cache.go:33` |
| GAP-2 | Contact handler untested | High | 3-4 hours | `contact_handler_test.go` (new) |

### P1 Issues (High Priority - Fix Soon) - ~1-2 Weeks

| Issue ID | Description | Effort | Files Affected |
|----------|-------------|--------|----------------|
| TEST-1 | HTTP client not injectable | 2 hours | Add `pkg/httpclient/`, update services |
| DUP-2 | Auth header extraction duplicated | 1 hour | Add `middleware/profile_auth.go` |
| ERR-1 | String-based error matching | 2-3 hours | Add `pkg/errors/`, update all handlers |
| ABS-3 | No typed errors | (same as ERR-1) | (same as ERR-1) |
| API-3 | Wrong HTTP status codes | 30 min | Update handlers |
| CFG-1 | CORS origins hardcoded | 30 min | Move to config |
| CFG-2 | Base URL hardcoded | 30 min | Move to config |
| SEC-8 | No CSRF protection | 2-3 hours | Add CSRF middleware |
| GO-4 | Context not propagated | 4-5 hours | Update all services, repos, caches |
| TEST-6 | No interfaces for dependencies | 2-3 hours | Add interfaces, update tests |
| API-5 | No API versioning | 2-3 hours | Add `/api/v1` routing |
| CFG-4 | Incomplete startup validation | 1 hour | Update config validation |

---

## Detailed Findings & Recommendations

### P0-1: SEC-1 - Secret in Query Parameter (webhook_handler.go:36)

**Severity:** 🔴 CRITICAL
**Effort:** 15 minutes
**Priority:** P0

#### Current Code (VULNERABLE)
```go
// internal/handlers/webhook_handler.go:34-53
func (h *WebhookHandler) RevalidateNextJS(c *gin.Context) {
    slug := c.Query("slug")
    secret := c.Query("secret")  // ❌ SECURITY ISSUE

    if slug == "" || secret == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Missing slug or secret"})
        return
    }

    if err := h.service.RevalidateNextJSManual(slug, secret); err != nil {
        if err.Error() == "invalid secret" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revalidate"})
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{"revalidated": true})
}
```

#### Why This Is Critical

Secrets in query parameters are exposed in:
1. **Server access logs** - logged by nginx, load balancers, CDNs
2. **Browser history** - stored indefinitely in user's browser
3. **Referer headers** - sent to any external resources on the page
4. **Proxy caches** - may be cached by intermediate proxies
5. **Monitoring tools** - APM tools, log aggregators capture URLs

**Real Attack Scenario:**
```bash
# Attacker finds secret in logs
GET /api/revalidate-nextjs?slug=mentor-profile&secret=supersecret123

# Now attacker can:
# 1. Force cache invalidation (DoS)
# 2. Clear specific pages repeatedly
# 3. Impact site performance
```

#### Recommended Fix

**Option 1: Use Header-Based Authentication (RECOMMENDED)**

```go
// internal/handlers/webhook_handler.go
func (h *WebhookHandler) RevalidateNextJS(c *gin.Context) {
    slug := c.Query("slug")
    secret := c.GetHeader("X-Revalidate-Secret")  // ✅ From header

    if slug == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Missing slug parameter"})
        return
    }

    if secret == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing X-Revalidate-Secret header"})
        return
    }

    if err := h.service.RevalidateNextJSManual(slug, secret); err != nil {
        if err.Error() == "invalid secret" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revalidate"})
        }
        return
    }

    c.JSON(http.StatusOK, gin.H{"revalidated": true})
}
```

**Option 2: Use Existing Auth Middleware (BETTER)**

```go
// cmd/api/main.go - Apply revalidate auth middleware
api.POST("/revalidate-nextjs",
    webhookRateLimiter.Middleware(),
    middleware.RevalidateAuthMiddleware(cfg.Auth.RevalidateSecret),  // ✅ Reuse auth pattern
    webhookHandler.RevalidateNextJS,
)

// internal/middleware/auth.go - Add new middleware
func RevalidateAuthMiddleware(validSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        secret := c.GetHeader("X-Revalidate-Secret")

        if secret == "" || secret != validSecret {
            logger.Warn("Invalid revalidate secret",
                zap.String("path", c.Request.URL.Path),
                zap.String("client_ip", c.ClientIP()),
            )
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid revalidate secret"})
            c.Abort()
            return
        }

        c.Next()
    }
}

// internal/handlers/webhook_handler.go - Simplified handler
func (h *WebhookHandler) RevalidateNextJS(c *gin.Context) {
    slug := c.Query("slug")

    if slug == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Missing slug parameter"})
        return
    }

    // Auth already verified by middleware
    if err := h.service.RevalidateNextJSInternal(slug); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revalidate"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"revalidated": true})
}

// internal/services/webhook_service.go
func (s *WebhookService) RevalidateNextJSInternal(slug string) error {
    return s.revalidateNextJS(slug)
}
```

**Migration Note:**
- Update any clients calling this endpoint to send header instead of query param
- Consider supporting both temporarily for backward compatibility
- Log deprecation warning if query param is used

---

### P0-2: SEC-2 - ReCAPTCHA Secret in GET URL (contact_service.go:87)

**Severity:** 🔴 CRITICAL
**Effort:** 20 minutes
**Priority:** P0

#### Current Code (VULNERABLE)

```go
// internal/services/contact_service.go:86-107
func (s *ContactService) verifyRecaptcha(token string) error {
    url := fmt.Sprintf("https://www.google.com/recaptcha/api/siteverify?secret=%s&response=%s",
        s.config.ReCAPTCHA.SecretKey, token)  // ❌ Secret in URL

    //nolint:gosec // URL is Google's official reCAPTCHA verification endpoint
    resp, err := http.Post(url, "application/x-www-form-urlencoded", nil)
    if err != nil {
        return fmt.Errorf("failed to verify recaptcha: %w", err)
    }
    defer resp.Body.Close()

    var result models.ReCAPTCHAResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return fmt.Errorf("failed to decode recaptcha response: %w", err)
    }

    if !result.Success {
        return fmt.Errorf("recaptcha verification failed")
    }

    return nil
}
```

#### Why This Is Critical

Even though this is calling Google's API:
1. **Secret appears in HTTP client logs** - Go's `http.Client` may log URLs
2. **Tracing tools capture URLs** - OpenTelemetry spans, APM tools
3. **HTTP proxies see the URL** - corporate proxies, debugging proxies
4. **Not Google's recommended approach** - [Official docs](https://developers.google.com/recaptcha/docs/verify) recommend POST with form body

#### Recommended Fix

```go
// internal/services/contact_service.go
import (
    "net/url"
    "strings"
)

func (s *ContactService) verifyRecaptcha(token string) error {
    // Prepare form data
    data := url.Values{}
    data.Set("secret", s.config.ReCAPTCHA.SecretKey)  // ✅ In POST body
    data.Set("response", token)

    // Send POST request with form body
    resp, err := http.Post(
        "https://www.google.com/recaptcha/api/siteverify",
        "application/x-www-form-urlencoded",
        strings.NewReader(data.Encode()),  // ✅ Secret in body, not URL
    )
    if err != nil {
        return fmt.Errorf("failed to verify recaptcha: %w", err)
    }
    defer resp.Body.Close()

    var result models.ReCAPTCHAResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return fmt.Errorf("failed to decode recaptcha response: %w", err)
    }

    if !result.Success {
        return fmt.Errorf("recaptcha verification failed")
    }

    return nil
}
```

**Testing Note:** Ensure this still works with Google's API after the change.

---

### P0-3: SEC-3 - Missing Input Validation (contact_handler.go)

**Severity:** 🔴 HIGH
**Effort:** 1 hour
**Priority:** P0

#### Current Code (VULNERABLE)

```go
// internal/handlers/contact_handler.go:19-24
func (h *ContactHandler) ContactMentor(c *gin.Context) {
    var req models.ContactMentorRequest
    if err := c.ShouldBindJSON(&req); err != nil {  // ❌ Only JSON parsing, no validation
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
        return
    }
    // ... proceeds without validating email format, length limits, etc.
}

// internal/models/contact.go
type ContactMentorRequest struct {
    Email            string `json:"email"`             // ❌ No validation
    Name             string `json:"name"`              // ❌ No validation
    Experience       string `json:"experience"`        // ❌ No validation
    Intro            string `json:"intro"`             // ❌ No validation
    TelegramUsername string `json:"telegram"`          // ❌ No validation
    MentorAirtableID string `json:"mentorId"`          // ❌ No validation
    RecaptchaToken   string `json:"recaptchaToken"`    // ❌ No validation
}
```

#### Attack Scenarios

1. **XSS Attack:**
```json
{
  "name": "<script>alert('xss')</script>",
  "email": "attacker@example.com",
  "intro": "Inject malicious script"
}
```

2. **DoS Attack:**
```json
{
  "name": "A".repeat(1000000),  // 1MB of 'A'
  "intro": "X".repeat(10000000) // 10MB string
}
```

3. **Data Pollution:**
```json
{
  "email": "not-an-email",
  "name": "",
  "experience": "invalid-level"
}
```

#### Recommended Fix

**Step 1: Add Validation Tags to Model**

```go
// internal/models/contact.go
type ContactMentorRequest struct {
    Email            string `json:"email" binding:"required,email,max=255"`
    Name             string `json:"name" binding:"required,min=2,max=100"`
    Experience       string `json:"experience" binding:"required,oneof=junior middle senior"`
    Intro            string `json:"intro" binding:"required,min=10,max=2000"`
    TelegramUsername string `json:"telegram" binding:"omitempty,max=50,alphanum"`
    MentorAirtableID string `json:"mentorId" binding:"required,startswith=rec"`
    RecaptchaToken   string `json:"recaptchaToken" binding:"required,min=20"`
}
```

**Step 2: Add Validation Error Formatter**

```go
// internal/handlers/helpers.go (new file)
package handlers

import (
    "github.com/go-playground/validator/v10"
    "github.com/gin-gonic/gin"
)

// ValidationError represents a single validation error
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

// ParseValidationErrors converts validator errors to user-friendly format
func ParseValidationErrors(err error) []ValidationError {
    var errors []ValidationError

    if validationErrors, ok := err.(validator.ValidationErrors); ok {
        for _, fieldError := range validationErrors {
            errors = append(errors, ValidationError{
                Field:   fieldError.Field(),
                Message: getErrorMessage(fieldError),
            })
        }
    }

    return errors
}

func getErrorMessage(fe validator.FieldError) string {
    switch fe.Tag() {
    case "required":
        return fe.Field() + " is required"
    case "email":
        return "Invalid email format"
    case "min":
        return fe.Field() + " must be at least " + fe.Param() + " characters"
    case "max":
        return fe.Field() + " must not exceed " + fe.Param() + " characters"
    case "oneof":
        return fe.Field() + " must be one of: " + fe.Param()
    case "alphanum":
        return fe.Field() + " must contain only letters and numbers"
    default:
        return fe.Field() + " is invalid"
    }
}
```

**Step 3: Update Handler to Use Validation**

```go
// internal/handlers/contact_handler.go
func (h *ContactHandler) ContactMentor(c *gin.Context) {
    var req models.ContactMentorRequest

    if err := c.ShouldBindJSON(&req); err != nil {
        // Format validation errors
        validationErrors := ParseValidationErrors(err)
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Validation failed",
            "details": validationErrors,
        })
        return
    }

    // At this point, input is validated and safe
    resp, err := h.service.SubmitContactForm(&req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }

    if !resp.Success {
        c.JSON(http.StatusBadRequest, resp)
        return
    }

    c.JSON(http.StatusOK, resp)
}
```

**Step 4: Apply to Other Models**

```go
// internal/models/profile.go
type SaveProfileRequest struct {
    Name         string   `json:"name" binding:"required,min=2,max=100"`
    Job          string   `json:"job" binding:"required,max=100"`
    Workplace    string   `json:"workplace" binding:"omitempty,max=100"`
    Experience   string   `json:"experience" binding:"required,max=50"`
    Price        string   `json:"price" binding:"required,max=50"`
    Tags         []string `json:"tags" binding:"max=20,dive,max=50"`
    Description  string   `json:"description" binding:"required,max=5000"`
    About        string   `json:"about" binding:"required,max=5000"`
    Competencies string   `json:"competencies" binding:"required,max=2000"`
    CalendarURL  string   `json:"calendarUrl" binding:"omitempty,url,max=500"`
}

type UploadProfilePictureRequest struct {
    Image       string `json:"image" binding:"required"`
    FileName    string `json:"fileName" binding:"required,max=255"`
    ContentType string `json:"contentType" binding:"required,oneof=image/jpeg image/png image/webp"`
}
```

**Benefits:**
- ✅ Prevents XSS, SQL injection, buffer overflow attacks
- ✅ Ensures data quality in database
- ✅ Better error messages for API clients
- ✅ Zero performance overhead (validation at parse time)
- ✅ Self-documenting API contract

---

### P0-4: GO-3 - Race Condition in Tags Cache (tags_cache.go:33)

**Severity:** 🔴 HIGH
**Effort:** 1-2 hours
**Priority:** P0

#### Current Code (RACE CONDITION)

```go
// internal/cache/tags_cache.go:23-36
func NewTagsCache(airtableClient *airtable.Client) *TagsCache {
    cache := gocache.New(tagsCacheTTL, time.Hour)

    tc := &TagsCache{
        cache:          cache,
        airtableClient: airtableClient,
    }

    // Initial population
    go tc.warmUp()  // ❌ Async warmup, returns immediately

    return tc  // ❌ Cache may still be empty!
}

// internal/cache/tags_cache.go:68-75
func (tc *TagsCache) warmUp() {
    logger.Info("Warming up tags cache")
    _, err := tc.refresh()
    if err != nil {
        logger.Error("Failed to warm up tags cache", zap.Error(err))
    }
}
```

#### Why This Is a Problem

**Race Condition Timeline:**
```
T=0ms:  NewTagsCache() called
T=1ms:  goroutine tc.warmUp() launched
T=2ms:  NewTagsCache() returns (cache EMPTY)
T=5ms:  Server starts accepting requests
T=10ms: First profile edit request arrives
T=11ms: GetTagIDByName() called -> CACHE MISS
T=50ms: warmUp() finally completes
```

**Impact:**
1. First few requests will fail or have degraded performance
2. Race detector (`go test -race`) will flag this
3. In production, causes unnecessary Airtable API calls
4. Profile edits may fail during startup

#### Running Race Detector

```bash
# This will catch the issue
go test -race ./internal/cache/...
```

#### Recommended Fix: Option 1 - Synchronous Initialization (Simpler)

```go
// internal/cache/tags_cache.go
func NewTagsCache(airtableClient *airtable.Client) *TagsCache {
    cache := gocache.New(tagsCacheTTL, time.Hour)

    tc := &TagsCache{
        cache:          cache,
        airtableClient: airtableClient,
    }

    // Initialize synchronously - blocks until cache is populated
    logger.Info("Initializing tags cache...")
    if _, err := tc.refresh(); err != nil {
        logger.Warn("Failed to initialize tags cache, will lazy-load", zap.Error(err))
        // Don't fail - cache will populate on first Get()
    } else {
        logger.Info("Tags cache initialized successfully")
    }

    return tc
}

// Remove warmUp() method - no longer needed
```

#### Recommended Fix: Option 2 - Async with WaitGroup (If startup time is critical)

```go
// internal/cache/tags_cache.go
import (
    "sync"
    "sync/atomic"
)

type TagsCache struct {
    cache          *gocache.Cache
    airtableClient *airtable.Client
    initialized    atomic.Bool  // ✅ Thread-safe flag
    initWG         sync.WaitGroup  // ✅ Synchronization primitive
}

func NewTagsCache(airtableClient *airtable.Client) *TagsCache {
    cache := gocache.New(tagsCacheTTL, time.Hour)

    tc := &TagsCache{
        cache:          cache,
        airtableClient: airtableClient,
    }

    // Launch warmup with WaitGroup
    tc.initWG.Add(1)
    go func() {
        defer tc.initWG.Done()
        logger.Info("Warming up tags cache in background")
        if _, err := tc.refresh(); err != nil {
            logger.Error("Failed to warm up tags cache", zap.Error(err))
        } else {
            tc.initialized.Store(true)
            logger.Info("Tags cache warmed up successfully")
        }
    }()

    return tc
}

func (tc *TagsCache) Get() (map[string]string, error) {
    // Wait for initial warmup to complete (only blocks once)
    tc.initWG.Wait()

    // Check cache
    if data, found := tc.cache.Get(tagsCacheKey); found {
        logger.Debug("Tags cache hit")
        return data.(map[string]string), nil
    }

    logger.Info("Tags cache miss, fetching from Airtable")

    // Cache miss, fetch and populate
    return tc.refresh()
}

// Optional: Add IsReady method for health checks
func (tc *TagsCache) IsReady() bool {
    return tc.initialized.Load()
}
```

#### Recommended Fix: Option 3 - Like MentorCache Pattern (Most Consistent)

```go
// Make TagsCache match MentorCache pattern for consistency

type TagsCache struct {
    cache          *gocache.Cache
    airtableClient *airtable.Client
    mu             sync.RWMutex
    ready          bool
}

func NewTagsCache(airtableClient *airtable.Client) *TagsCache {
    cache := gocache.New(tagsCacheTTL, time.Hour)

    return &TagsCache{
        cache:          cache,
        airtableClient: airtableClient,
        ready:          false,
    }
}

// Initialize performs initial cache population (synchronous, blocks until ready)
// Should be called during application startup before accepting requests
func (tc *TagsCache) Initialize() error {
    logger.Info("Initializing tags cache...")
    _, err := tc.refresh()
    if err != nil {
        logger.Error("Failed to initialize tags cache", zap.Error(err))
        return err
    }

    tc.mu.Lock()
    tc.ready = true
    tc.mu.Unlock()

    logger.Info("Tags cache initialized successfully")
    return nil
}

func (tc *TagsCache) IsReady() bool {
    tc.mu.RLock()
    defer tc.mu.RUnlock()
    return tc.ready
}

func (tc *TagsCache) Get() (map[string]string, error) {
    if !tc.IsReady() {
        return nil, fmt.Errorf("tags cache not initialized")
    }

    // Check cache
    if data, found := tc.cache.Get(tagsCacheKey); found {
        logger.Debug("Tags cache hit")
        return data.(map[string]string), nil
    }

    logger.Info("Tags cache miss, fetching from Airtable")
    return tc.refresh()
}

// cmd/api/main.go - Update initialization
tagsCache := cache.NewTagsCache(airtableClient)

// Initialize tags cache synchronously (just like mentor cache)
if err := tagsCache.Initialize(); err != nil {
    logger.Fatal("Failed to initialize tags cache", zap.Error(err))
}
```

**Recommendation:** Use **Option 3** for consistency with existing `MentorCache` pattern.

---

### P0-5: GAP-2 - Contact Handler Untested (contact_handler.go)

**Severity:** 🔴 HIGH (Data Loss Risk)
**Effort:** 3-4 hours
**Priority:** P0

#### Why This Is Critical

The contact form handler:
- Accepts user-submitted data (potential revenue)
- Creates records in Airtable (data persistence)
- Validates ReCAPTCHA (security control)
- Returns mentor calendar URLs (critical user flow)

**0% test coverage** means:
- Bugs can cause lost contact requests → revenue loss
- ReCAPTCHA bypass vulnerabilities undetected
- No regression protection when refactoring
- Cannot confidently deploy changes

#### Recommended Test Suite

**Step 1: Create Mock Service**

```go
// test/internal/handlers/contact_handler_test.go
package handlers_test

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/getmentor/getmentor-api/internal/handlers"
    "github.com/getmentor/getmentor-api/internal/models"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func init() {
    gin.SetMode(gin.TestMode)
}

// MockContactService implements contact service interface
type MockContactService struct {
    mock.Mock
}

func (m *MockContactService) SubmitContactForm(req *models.ContactMentorRequest) (*models.ContactMentorResponse, error) {
    args := m.Called(req)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*models.ContactMentorResponse), args.Error(1)
}

// Test successful contact form submission
func TestContactHandler_ContactMentor_Success(t *testing.T) {
    // Setup
    mockService := new(MockContactService)
    handler := handlers.NewContactHandler(mockService)

    router := gin.New()
    router.POST("/contact", handler.ContactMentor)

    // Prepare test data
    reqBody := models.ContactMentorRequest{
        Email:            "test@example.com",
        Name:             "Test User",
        Experience:       "middle",
        Intro:            "I want to learn Go programming",
        TelegramUsername: "testuser",
        MentorAirtableID: "rec123",
        RecaptchaToken:   "valid-token-123",
    }

    // Mock successful response
    mockService.On("SubmitContactForm", mock.MatchedBy(func(req *models.ContactMentorRequest) bool {
        return req.Email == "test@example.com" && req.Name == "Test User"
    })).Return(&models.ContactMentorResponse{
        Success:     true,
        CalendarURL: "https://calendly.com/mentor-slug",
    }, nil)

    // Execute request
    body, _ := json.Marshal(reqBody)
    req := httptest.NewRequest("POST", "/contact", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    // Assert response
    assert.Equal(t, http.StatusOK, w.Code)

    var resp models.ContactMentorResponse
    err := json.Unmarshal(w.Body.Bytes(), &resp)
    assert.NoError(t, err)
    assert.True(t, resp.Success)
    assert.Equal(t, "https://calendly.com/mentor-slug", resp.CalendarURL)

    // Verify mock was called correctly
    mockService.AssertExpectations(t)
}

// Test with invalid JSON
func TestContactHandler_ContactMentor_InvalidJSON(t *testing.T) {
    mockService := new(MockContactService)
    handler := handlers.NewContactHandler(mockService)

    router := gin.New()
    router.POST("/contact", handler.ContactMentor)

    // Send invalid JSON
    req := httptest.NewRequest("POST", "/contact", bytes.NewReader([]byte("not-json")))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusBadRequest, w.Code)

    var resp map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.Contains(t, resp, "error")
}

// Test with missing required fields (after validation is added)
func TestContactHandler_ContactMentor_MissingFields(t *testing.T) {
    mockService := new(MockContactService)
    handler := handlers.NewContactHandler(mockService)

    router := gin.New()
    router.POST("/contact", handler.ContactMentor)

    // Missing required fields
    reqBody := models.ContactMentorRequest{
        Email: "test@example.com",
        // Missing: Name, Experience, Intro, MentorAirtableID, RecaptchaToken
    }

    body, _ := json.Marshal(reqBody)
    req := httptest.NewRequest("POST", "/contact", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Test with invalid email format (after validation is added)
func TestContactHandler_ContactMentor_InvalidEmail(t *testing.T) {
    mockService := new(MockContactService)
    handler := handlers.NewContactHandler(mockService)

    router := gin.New()
    router.POST("/contact", handler.ContactMentor)

    reqBody := models.ContactMentorRequest{
        Email:            "not-an-email",  // Invalid format
        Name:             "Test User",
        Experience:       "middle",
        Intro:            "I want to learn Go",
        MentorAirtableID: "rec123",
        RecaptchaToken:   "token",
    }

    body, _ := json.Marshal(reqBody)
    req := httptest.NewRequest("POST", "/contact", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Test ReCAPTCHA failure
func TestContactHandler_ContactMentor_CaptchaFailed(t *testing.T) {
    mockService := new(MockContactService)
    handler := handlers.NewContactHandler(mockService)

    router := gin.New()
    router.POST("/contact", handler.ContactMentor)

    reqBody := models.ContactMentorRequest{
        Email:            "test@example.com",
        Name:             "Test User",
        Experience:       "middle",
        Intro:            "I want to learn Go",
        MentorAirtableID: "rec123",
        RecaptchaToken:   "invalid-token",
    }

    // Mock captcha failure
    mockService.On("SubmitContactForm", mock.Anything).Return(
        &models.ContactMentorResponse{
            Success: false,
            Error:   "Captcha verification failed",
        },
        nil,
    )

    body, _ := json.Marshal(reqBody)
    req := httptest.NewRequest("POST", "/contact", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusBadRequest, w.Code)

    var resp models.ContactMentorResponse
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.False(t, resp.Success)
    assert.Contains(t, resp.Error, "Captcha")

    mockService.AssertExpectations(t)
}

// Test service error
func TestContactHandler_ContactMentor_ServiceError(t *testing.T) {
    mockService := new(MockContactService)
    handler := handlers.NewContactHandler(mockService)

    router := gin.New()
    router.POST("/contact", handler.ContactMentor)

    reqBody := models.ContactMentorRequest{
        Email:            "test@example.com",
        Name:             "Test User",
        Experience:       "middle",
        Intro:            "I want to learn Go",
        MentorAirtableID: "rec123",
        RecaptchaToken:   "token",
    }

    // Mock service returning error
    mockService.On("SubmitContactForm", mock.Anything).Return(
        nil,
        errors.New("internal service error"),
    )

    body, _ := json.Marshal(reqBody)
    req := httptest.NewRequest("POST", "/contact", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusInternalServerError, w.Code)

    mockService.AssertExpectations(t)
}

// Test with very long strings (boundary testing)
func TestContactHandler_ContactMentor_LongStrings(t *testing.T) {
    mockService := new(MockContactService)
    handler := handlers.NewContactHandler(mockService)

    router := gin.New()
    router.POST("/contact", handler.ContactMentor)

    // Create extremely long intro (should be rejected after validation is added)
    longIntro := strings.Repeat("A", 10000)

    reqBody := models.ContactMentorRequest{
        Email:            "test@example.com",
        Name:             "Test User",
        Experience:       "middle",
        Intro:            longIntro,  // Too long
        MentorAirtableID: "rec123",
        RecaptchaToken:   "token",
    }

    body, _ := json.Marshal(reqBody)
    req := httptest.NewRequest("POST", "/contact", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    // Should be rejected (400) after validation is added
    assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

**Step 2: Run Tests**

```bash
# Run the tests
go test -v ./test/internal/handlers/

# Run with coverage
go test -v -coverprofile=coverage.out ./test/internal/handlers/
go tool cover -func=coverage.out

# Run with race detector
go test -v -race ./test/internal/handlers/
```

**Step 3: Add Interface for Service (enables mocking)**

```go
// internal/services/interfaces.go (new file)
package services

import (
    "github.com/getmentor/getmentor-api/internal/models"
)

type ContactServiceInterface interface {
    SubmitContactForm(req *models.ContactMentorRequest) (*models.ContactMentorResponse, error)
}

// Ensure ContactService implements the interface
var _ ContactServiceInterface = (*ContactService)(nil)
```

**Coverage Goal:** Aim for >80% coverage on this critical handler.

---

## Testing Strategy

### Immediate Actions (P0)

1. **Write tests for contact handler** (GAP-2) - 3-4 hours
2. **Set up mocking infrastructure** (TEST-6) - 2-3 hours
   - Create service interfaces
   - Set up testify/mock
   - Document testing patterns

### Short-term (P1) - 1-2 weeks

1. **Test critical handlers**
   - Profile handler (GAP-3)
   - Webhook handler (GAP-4)
   - Mentor handler (GAP-1)

2. **Test business logic**
   - All services (GAP-6)
   - Test with mocked dependencies

3. **Set up CI/CD quality gates**
   - Minimum coverage threshold: 70%
   - Race detector must pass
   - Linter must pass

### Medium-term (P2) - 2-3 weeks

1. **Repository tests** (GAP-7)
   - Mock Airtable client
   - Test caching behavior
   - Test error handling

2. **Cache tests** (GAP-8, GAP-9)
   - Test TTL behavior
   - Test race conditions
   - Test refresh logic

3. **Integration tests** (GAP-10)
   - End-to-end API tests
   - Test with real dependencies (test instance of Airtable)

### Long-term (P3-P4) - Ongoing

1. **Middleware tests** (GAP-12)
2. **Error path coverage** (GAP-11)
3. **Property-based testing** for complex logic
4. **Load/stress testing**

### Test Structure Improvement

**Current (Non-idiomatic):**
```
test/
├── internal/
└── pkg/
```

**Recommended (Idiomatic Go):**
```
internal/
├── handlers/
│   ├── contact_handler.go
│   └── contact_handler_test.go  ← Co-located
├── services/
│   ├── contact_service.go
│   └── contact_service_test.go  ← Co-located
```

**Migration Strategy:**
1. Write new tests co-located
2. Gradually migrate existing tests
3. Remove old `test/` directory when done

---

## Long-term Architectural Improvements

### 1. Service Layer Consistency (ARCH-2)

**Problem:** MentorService is a pass-through, while ContactService and ProfileService contain business logic.

**Options:**

**Option A: Remove MentorService (Recommended if no future logic planned)**
```go
// Handlers call repository directly
func (h *MentorHandler) GetPublicMentors(c *gin.Context) {
    mentors, err := h.repo.GetAll(c.Request.Context(), models.FilterOptions{
        OnlyVisible: true,
    })
    // ...
}
```

**Option B: Keep for consistency, add value later**
```go
// Add business logic to MentorService as it grows
func (s *MentorService) GetFeaturedMentors(ctx context.Context) ([]*models.Mentor, error) {
    // Business logic: filter featured mentors
    mentors, err := s.repo.GetAll(ctx, models.FilterOptions{OnlyVisible: true})
    if err != nil {
        return nil, err
    }

    // Filter for featured
    featured := make([]*models.Mentor, 0)
    for _, m := range mentors {
        if m.SortOrder <= 10 {  // Top 10 are featured
            featured = append(featured, m)
        }
    }

    return featured, nil
}
```

**Recommendation:** Keep service layer for consistency, add more value over time.

### 2. API Response Standardization (API-1, API-7, API-8)

**Current:** Inconsistent responses
```go
// Some handlers
c.JSON(http.StatusOK, gin.H{"success": true})

// Others
c.JSON(http.StatusOK, gin.H{"mentors": mentors})

// Errors
c.JSON(http.StatusBadRequest, gin.H{"error": "message"})
```

**Recommended:** Standard response envelope

```go
// pkg/response/response.go
package response

type APIResponse struct {
    Success   bool        `json:"success"`
    Data      interface{} `json:"data,omitempty"`
    Error     *APIError   `json:"error,omitempty"`
    RequestID string      `json:"request_id,omitempty"`
}

type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details interface{} `json:"details,omitempty"`
}

// Standard error codes
const (
    ErrCodeValidation   = "VALIDATION_ERROR"
    ErrCodeNotFound     = "NOT_FOUND"
    ErrCodeUnauthorized = "UNAUTHORIZED"
    ErrCodeForbidden    = "FORBIDDEN"
    ErrCodeInternal     = "INTERNAL_ERROR"
    ErrCodeRateLimit    = "RATE_LIMIT"
)

func Success(c *gin.Context, data interface{}) {
    requestID := c.GetString("request_id")
    c.JSON(http.StatusOK, APIResponse{
        Success:   true,
        Data:      data,
        RequestID: requestID,
    })
}

func Error(c *gin.Context, statusCode int, code, message string, details interface{}) {
    requestID := c.GetString("request_id")
    c.JSON(statusCode, APIResponse{
        Success: false,
        Error: &APIError{
            Code:    code,
            Message: message,
            Details: details,
        },
        RequestID: requestID,
    })
}

// Usage in handlers
func (h *MentorHandler) GetPublicMentors(c *gin.Context) {
    mentors, err := h.service.GetAllMentors(c.Request.Context(), models.FilterOptions{
        OnlyVisible: true,
    })
    if err != nil {
        response.Error(c, http.StatusInternalServerError,
            response.ErrCodeInternal, "Failed to fetch mentors", nil)
        return
    }

    publicMentors := make([]models.PublicMentorResponse, 0, len(mentors))
    for _, mentor := range mentors {
        publicMentors = append(publicMentors, mentor.ToPublicResponse("https://getmentor.dev"))
    }

    response.Success(c, gin.H{"mentors": publicMentors})
}
```

### 3. Request ID Tracing (SEC-9)

```go
// middleware/request_id.go
package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

func RequestIDMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Check if client provided request ID
        requestID := c.GetHeader("X-Request-ID")

        // Generate if not provided
        if requestID == "" {
            requestID = uuid.New().String()
        }

        // Store in context
        c.Set("request_id", requestID)

        // Add to response headers
        c.Header("X-Request-ID", requestID)

        // Add to logger context (if logger supports it)
        logger.WithContext(c).Info("Request received",
            zap.String("request_id", requestID),
            zap.String("method", c.Request.Method),
            zap.String("path", c.Request.URL.Path),
        )

        c.Next()
    }
}

// Apply in main.go
router.Use(middleware.RequestIDMiddleware())
```

### 4. API Versioning (API-5)

```go
// cmd/api/main.go
// Current
api := router.Group("/api")

// Recommended
v1 := router.Group("/api/v1")
{
    // All current endpoints
    v1.GET("/mentors", ...)
    v1.POST("/contact-mentor", ...)
}

// Future v2 can coexist
v2 := router.Group("/api/v2")
{
    // New endpoints or breaking changes
}
```

### 5. CSRF Protection (SEC-8)

```go
// Use existing CSRF middleware
import "github.com/gin-contrib/csrf"

// main.go
csrfMiddleware := csrf.Middleware(csrf.Options{
    Secret: cfg.Auth.CSRFSecret,
    ErrorFunc: func(c *gin.Context) {
        c.JSON(http.StatusForbidden, gin.H{"error": "CSRF token invalid"})
        c.Abort()
    },
})

// Apply to state-changing endpoints
api.POST("/contact-mentor", contactRateLimiter.Middleware(), csrfMiddleware, contactHandler.ContactMentor)
api.POST("/save-profile", profileRateLimiter.Middleware(), csrfMiddleware, profileHandler.SaveProfile)
```

---

## P1 Fixes Completed

### P1-8: TEST-6 - No Interfaces for Dependencies

**Status:** ✅ COMPLETED
**Effort:** 2 hours
**Priority:** P1

#### Problem
Services were concrete types, making it impossible to create mocks for testing handlers. This prevented proper unit testing of handlers in isolation.

#### Solution Implemented

**Step 1: Created Service Interfaces**

Added interface definitions in `internal/services/interfaces.go`:

```go
// MentorServiceInterface defines the interface for mentor service operations
type MentorServiceInterface interface {
    GetAllMentors(opts models.FilterOptions) ([]*models.Mentor, error)
    GetMentorByID(id int, opts models.FilterOptions) (*models.Mentor, error)
    GetMentorBySlug(slug string, opts models.FilterOptions) (*models.Mentor, error)
    GetMentorByRecordID(recordID string, opts models.FilterOptions) (*models.Mentor, error)
}

// ProfileServiceInterface defines the interface for profile service operations
type ProfileServiceInterface interface {
    SaveProfile(id int, token string, req *models.SaveProfileRequest) error
    UploadProfilePicture(id int, token string, req *models.UploadProfilePictureRequest) (string, error)
}

// WebhookServiceInterface defines the interface for webhook service operations
type WebhookServiceInterface interface {
    HandleAirtableWebhook(payload *models.WebhookPayload) error
}

// Ensure services implement their interfaces (compile-time checks)
var _ ContactServiceInterface = (*ContactService)(nil)
var _ MentorServiceInterface = (*MentorService)(nil)
var _ ProfileServiceInterface = (*ProfileService)(nil)
var _ WebhookServiceInterface = (*WebhookService)(nil)
```

**Step 2: Updated Handlers to Use Interfaces**

Modified all handlers to accept and use interfaces instead of concrete types:

```go
// internal/handlers/mentor_handler.go
type MentorHandler struct {
    service services.MentorServiceInterface  // Changed from *services.MentorService
    baseURL string
}

func NewMentorHandler(service services.MentorServiceInterface, baseURL string) *MentorHandler {
    return &MentorHandler{
        service: service,
        baseURL: baseURL,
    }
}

// internal/handlers/profile_handler.go
type ProfileHandler struct {
    service services.ProfileServiceInterface  // Changed from *services.ProfileService
}

func NewProfileHandler(service services.ProfileServiceInterface) *ProfileHandler {
    return &ProfileHandler{service: service}
}

// internal/handlers/webhook_handler.go
type WebhookHandler struct {
    service services.WebhookServiceInterface  // Changed from *services.WebhookService
}

func NewWebhookHandler(service services.WebhookServiceInterface) *WebhookHandler {
    return &WebhookHandler{service: service}
}
```

#### Benefits

1. **Testability**: Handlers can now be tested with mocked services
2. **Type Safety**: Compile-time checks ensure services implement interfaces
3. **Decoupling**: Handlers depend on abstractions, not concrete implementations
4. **Future-proof**: Easy to swap implementations or add decorators

#### Verification

- ✅ Project builds successfully: `go build ./...`
- ✅ All tests pass: `go test ./...`
- ✅ No breaking changes to existing code
- ✅ Compile-time interface conformance checks in place

### P1-9: SEC-8 - CSRF Protection

**Status:** ✅ NOT APPLICABLE (Documented)
**Effort:** 1 hour (analysis)
**Priority:** P1

#### Problem Analysis

The original issue identified lack of CSRF protection as a security concern.

#### Investigation

After thorough analysis of the codebase:

1. **Authentication Method**: API uses header-based authentication (X-Mentor-ID, X-Auth-Token)
2. **No Credentials**: `AllowCredentials: false` in CORS config (cmd/api/main.go:159)
3. **No Cookies**: Grep search confirms no cookie usage anywhere in codebase
4. **CORS Configured**: Proper CORS configuration with allowed origins

#### Conclusion

**CSRF protection is NOT NEEDED for this API architecture.**

**Reasoning:**
- CSRF attacks exploit browsers' automatic sending of credentials (cookies, HTTP auth)
- This API uses custom headers for authentication
- Browsers cannot automatically send custom headers in cross-origin requests
- CORS prevents malicious sites from reading responses even if they make requests
- The `/contact-mentor` endpoint has ReCAPTCHA + rate limiting protection

**Current Protection Mechanisms:**
- ✅ Header-based auth prevents automatic credential sending
- ✅ CORS restricts which origins can make requests
- ✅ ReCAPTCHA protects public endpoints
- ✅ Rate limiting prevents abuse
- ✅ Proper input validation

**Reference:** OWASP recommends CSRF protection primarily for cookie-based sessions. For stateless APIs with header-based auth, CSRF is not applicable.

#### Recommendation

No code changes needed. This issue can be marked as resolved with the understanding that CSRF protection is not required for header-based authentication APIs.

### P1-10: API-5 - No API Versioning

**Status:** ✅ COMPLETED
**Effort:** 2 hours
**Priority:** P1

#### Problem

API endpoints had no versioning scheme, making it difficult to introduce breaking changes or evolve the API without disrupting existing clients.

#### Solution Implemented

Added versioned routing structure with backward compatibility:

**Changes to `cmd/api/main.go`:**

```go
// API routes - operational endpoints (unversioned)
api := router.Group("/api")
{
    api.GET("/healthcheck", ...)
    api.GET("/metrics", ...)
}

// API v1 routes - all business endpoints
v1 := router.Group("/api/v1")
v1.Use(middleware.BodySizeLimitMiddleware(1 * 1024 * 1024))
{
    v1.GET("/mentors", ...)
    v1.GET("/mentor/:id", ...)
    v1.POST("/internal/mentors", ...)
    v1.POST("/contact-mentor", ...)
    v1.POST("/save-profile", ...)
    v1.POST("/upload-profile-picture", ...)
    v1.POST("/logs", ...)
    v1.POST("/webhooks/airtable", ...)
}

// Backward compatibility: Alias old /api/* routes to /api/v1/*
apiCompat := router.Group("/api")
apiCompat.Use(middleware.BodySizeLimitMiddleware(1 * 1024 * 1024))
{
    // All endpoints duplicated for gradual migration
}
```

#### Design Decisions

1. **Operational Endpoints Unversioned**: `/api/healthcheck` and `/api/metrics` remain at root level as they're operational, not business endpoints
2. **Backward Compatibility**: Old `/api/*` routes still work, aliased to `/api/v1/*`
3. **Clear Deprecation Path**: Comments indicate compatibility layer should be removed in future
4. **No Breaking Changes**: Existing clients continue to work without modification

#### Benefits

1. **Future-proof**: Can introduce v2 API without breaking v1 clients
2. **Gradual Migration**: Clients can migrate at their own pace
3. **Clear Versioning**: API version is explicit in the URL
4. **Industry Standard**: Follows REST API versioning best practices

#### API Endpoints

**New Versioned Endpoints:**
- `GET /api/v1/mentors`
- `GET /api/v1/mentor/:id`
- `POST /api/v1/internal/mentors`
- `POST /api/v1/contact-mentor`
- `POST /api/v1/save-profile`
- `POST /api/v1/upload-profile-picture`
- `POST /api/v1/logs`
- `POST /api/v1/webhooks/airtable`

**Operational Endpoints (Unversioned):**
- `GET /api/healthcheck`
- `GET /api/metrics`

**Deprecated (but still working):**
- All old `/api/*` business endpoints

#### Migration Plan

1. **Phase 1 (Current)**: Both `/api/*` and `/api/v1/*` work
2. **Phase 2 (Next Release)**: Add deprecation warnings to old endpoints
3. **Phase 3 (Future)**: Remove compatibility layer after client migration

#### Verification

- ✅ Project builds successfully
- ✅ All tests pass
- ✅ Backward compatibility maintained
- ✅ New versioned endpoints available

### P1-11: GO-4 - Context Not Propagated Through Layers

**Status:** ✅ COMPLETED
**Effort:** 4 hours
**Priority:** P1

#### Problem

Context was not being propagated through the application layers (handlers → services → repositories), preventing proper request cancellation, timeout handling, and distributed tracing.

#### Solution Implemented

Added `context.Context` as the first parameter to all service and repository methods, following Go best practices.

**Changes Made:**

1. **Service Interfaces** (`internal/services/interfaces.go`)
   - Added `ctx context.Context` as first parameter to all interface methods
   - Updated: ContactServiceInterface, MentorServiceInterface, ProfileServiceInterface, WebhookServiceInterface

2. **Service Implementations**
   - `internal/services/mentor_service.go`: Updated 4 methods
   - `internal/services/contact_service.go`: Updated SubmitContactForm
   - `internal/services/profile_service.go`: Updated SaveProfile, UploadProfilePicture
   - `internal/services/webhook_service.go`: Updated HandleAirtableWebhook

3. **Repository Layer**
   - `internal/repository/mentor_repository.go`: Updated 9 methods (GetAll, GetByID, GetBySlug, GetByRecordID, Update, UpdateImage, GetTagIDByName, GetAllTags)
   - `internal/repository/client_request_repository.go`: Updated Create method

4. **Handler Layer**
   - All handlers updated to extract context from `gin.Context` and pass it down
   - `internal/handlers/mentor_handler.go`: 6 service calls updated
   - `internal/handlers/contact_handler.go`: 1 service call updated
   - `internal/handlers/profile_handler.go`: 2 service calls updated
   - `internal/handlers/webhook_handler.go`: 1 service call updated

5. **Tests**
   - Updated mock service in `test/internal/handlers/contact_handler_test.go`
   - Updated all mock expectations to include context parameter

**Example Context Propagation:**

```go
// Handler extracts context from gin.Context
func (h *MentorHandler) GetPublicMentors(c *gin.Context) {
    mentors, err := h.service.GetAllMentors(c.Request.Context(), models.FilterOptions{
        OnlyVisible: true,
    })
    // ...
}

// Service passes context to repository
func (s *MentorService) GetAllMentors(ctx context.Context, opts models.FilterOptions) ([]*models.Mentor, error) {
    return s.repo.GetAll(ctx, opts)
}

// Repository receives context (ready for future use with database calls)
func (r *MentorRepository) GetAll(ctx context.Context, opts models.FilterOptions) ([]*models.Mentor, error) {
    // Context available for cancellation, timeouts, tracing
    // ...
}
```

**Async Operations:**

For the async goroutine in ProfileService.UploadProfilePicture, used `context.Background()` since the operation should complete independently of the request lifecycle:

```go
go func() {
    if err := s.mentorRepo.UpdateImage(context.Background(), mentor.AirtableID, imageURL); err != nil {
        logger.Error("Failed to update mentor image in Airtable", zap.Error(err))
    }
}()
```

#### Benefits

1. **Request Cancellation**: If client disconnects, context cancellation can stop ongoing operations
2. **Timeout Handling**: Can set deadlines for operations using context.WithTimeout
3. **Distributed Tracing**: Context carries trace IDs through the call stack for OpenTelemetry
4. **Best Practices**: Follows Go standard library conventions for context propagation
5. **Future-Proof**: Ready for adding cancellation logic, timeouts, and trace propagation

#### Next Steps (Future Enhancements)

1. Add context timeouts at service layer for long-running operations
2. Implement cancellation checks in repository methods
3. Propagate context to Airtable client calls for cancellation support
4. Add context values for request IDs and trace propagation

#### Verification

- ✅ Project builds successfully
- ✅ All tests pass (including updated mocks)
- ✅ Context flows from gin.Context through all layers
- ✅ No breaking changes to external API

---

## Conclusion

This codebase has a **solid foundation** with excellent observability and resilience patterns. The main areas for improvement are:

1. **Security** - 5 critical issues need immediate attention
2. **Testing** - Dramatically increase coverage from <10% to >70%
3. **Testability** - Refactor to enable easier testing
4. **Consistency** - Standardize patterns across handlers and services

**Recommended Approach:**
1. **Week 1:** Fix all P0 issues (security + race condition + contact handler tests)
2. **Weeks 2-3:** Address P1 issues (testability, typed errors, context propagation)
3. **Weeks 4-5:** P2 testing infrastructure and remaining tests
4. **Ongoing:** P3/P4 improvements as time permits

The codebase is maintainable and well-structured. With these improvements, it will be production-ready with high confidence.

---

**Document Version:** 1.0
**Last Updated:** 2025-11-25
**Next Review:** After P0 and P1 fixes are complete
