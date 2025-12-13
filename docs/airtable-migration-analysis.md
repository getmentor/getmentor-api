# Airtable to Database Migration Analysis

## Executive Summary

This document analyzes the current Airtable usage in the GetMentor API and provides a migration plan to a managed database solution in Yandex Cloud.

**Recommendation:** Migrate to **Yandex Managed Service for PostgreSQL**

---

## 1. Current State Analysis

### 1.1 Data Models

The application uses 3 Airtable tables:

#### Mentors Table (6,000 records)
| Field | Type | Description |
|-------|------|-------------|
| Id | Integer | Unique mentor ID |
| Alias | String | URL slug identifier |
| Name | String | Mentor name |
| JobTitle | String | Job title |
| Workplace | String | Company/workplace |
| Details | String | Detailed description (long text) |
| About | String | About section (long text) |
| Competencies | String | Technical competencies (long text) |
| Experience | String | Experience level (e.g., "5-10") |
| Price | String | Hourly rate/pricing |
| Done Sessions Count | Integer | Number of completed sessions |
| Tags | String | Comma-separated tags |
| SortOrder | Integer | Display ordering |
| OnSite | Integer | Visibility flag (1=visible) |
| Status | String | Status ("active" or other) |
| AuthToken | String | Authentication token (sensitive) |
| Calendly Url | String | Calendar scheduling URL |
| Is New | Integer | New mentor flag |
| Image_Attachment | Array | Profile image attachment |

**Source:** `internal/models/mentor.go:77-101`

#### Client Requests Table (100,000 records)
| Field | Type | Description |
|-------|------|-------------|
| Email | String | Client email |
| Name | String | Client name |
| Description | String | Contact request message |
| Telegram | String | Telegram username |
| Level | String | Experience level (optional) |
| Mentor | Link | Reference to Mentors table |
| Status | String | Request status: "active", "in_progress", "done" |

**Source:** `pkg/airtable/client.go:305-312`

**Note:** The Client Requests table is accessed by a **Telegram chatbot** which:
- Fetches requests for a given mentor
- Updates the status field through the request lifecycle (active → in_progress → done)

#### Tags Table (~50 records estimated)
| Field | Type | Description |
|-------|------|-------------|
| Name | String | Tag name |
| (record ID) | String | Airtable record ID |

**Source:** `pkg/airtable/client.go:430-436`

### 1.2 Data Relationships

```
┌─────────────────┐       ┌──────────────────┐
│ Client Requests │──────▶│     Mentors      │
│                 │  N:1  │                  │
└─────────────────┘       └────────┬─────────┘
                                   │
                                   │ N:M (via comma-separated string)
                                   ▼
                          ┌──────────────────┐
                          │      Tags        │
                          └──────────────────┘
```

### 1.3 Query Patterns

#### API Server (this codebase)
| Operation | Frequency | Pattern |
|-----------|-----------|---------|
| Get all mentors | Every 10 min (cache refresh) | Fetch all → cache |
| Get mentor by ID/slug | High (served from cache) | Cache lookup O(1) |
| Create client request | Low (~10/day) | Single INSERT |
| Update mentor profile | Low (~5/day) | Single UPDATE |
| Get all tags | Every 24h (cache refresh) | Fetch all → cache |

#### Telegram Chatbot (external system)
| Operation | Frequency | Pattern |
|-----------|-----------|---------|
| Get requests by mentor | Moderate | Filter by mentor_id |
| Update request status | Moderate | Single UPDATE (status field) |

**Important:** The Telegram bot directly accesses Airtable. After migration, it will need PostgreSQL access or an API endpoint.

### 1.4 Current Architecture

```
HTTP Request → Handler → Service → Repository → Cache → Airtable Client → Airtable API
                                       ↑
                                       └── In-memory cache (go-cache)
                                           • Mentor TTL: 10 minutes
                                           • Tags TTL: 24 hours
```

**Key Characteristics:**
- Read-heavy workload (99%+ reads from cache)
- Simple queries (no JOINs, no aggregations in Airtable)
- All filtering/sorting done in-memory after fetching
- Good abstraction layer already exists (repository pattern)

### 1.5 Data Volume Projections

| Year | Mentors | Client Requests | Total Records |
|------|---------|-----------------|---------------|
| 2025 | 6,000 | 100,000 | 106,000 |
| 2026 | 9,000 | 150,000 | 159,000 |
| 2027 | 13,500 | 225,000 | 238,500 |
| 2028 | 20,250 | 337,500 | 357,750 |
| 2030 | 45,500 | 759,375 | ~805,000 |

Even at 5-year projection, data volume remains modest (<1M records).

---

## 2. Database Options Analysis

### 2.1 Available Options in Yandex Cloud

| Service | Type | Best For |
|---------|------|----------|
| **Managed PostgreSQL** | Relational RDBMS | General-purpose, OLTP workloads |
| **Managed MySQL** | Relational RDBMS | General-purpose, OLTP workloads |
| **YDB** | Distributed SQL | High-scale, global distribution |
| Managed Redis | Key-Value Store | Caching only |
| Managed Greenplum | MPP Analytics | OLAP workloads |
| Managed OpenSearch | Search Engine | Full-text search |

### 2.2 Eliminated Options

| Option | Reason for Elimination |
|--------|------------------------|
| **Redis** | Key-value store, not suitable as primary database |
| **Greenplum** | Designed for OLAP/analytics, overkill for simple OLTP |
| **OpenSearch** | Search engine, not a transactional database |
| **NoSQL/Document DB** | Data is clearly relational with FK constraints |

### 2.3 Top Two Candidates

Based on the workload characteristics:

1. **Yandex Managed PostgreSQL** - Industry-standard relational database
2. **Yandex YDB** - Yandex's distributed SQL database

---

## 3. Detailed Comparison: PostgreSQL vs YDB

### 3.1 Feature Comparison

| Criteria | PostgreSQL | YDB |
|----------|------------|-----|
| **Data Model Fit** | ⭐⭐⭐⭐⭐ Perfect for relational data | ⭐⭐⭐⭐ Good, but designed for larger scale |
| **Query Language** | SQL (standard) | YQL (SQL-like, but different) |
| **Go Driver Maturity** | ⭐⭐⭐⭐⭐ pgx, sqlx, GORM - mature | ⭐⭐⭐ ydb-go-sdk - less ecosystem |
| **Learning Curve** | ⭐⭐⭐⭐⭐ Team likely familiar | ⭐⭐⭐ Requires learning new concepts |
| **Operational Complexity** | ⭐⭐⭐⭐⭐ Simple, well-documented | ⭐⭐⭐ More complex topology |
| **Cost Efficiency** | ⭐⭐⭐⭐⭐ Pay for what you use | ⭐⭐⭐⭐ Serverless option available |
| **Performance at 10 RPS** | ⭐⭐⭐⭐⭐ More than sufficient | ⭐⭐⭐⭐⭐ Overkill |
| **Scaling Ceiling** | ⭐⭐⭐⭐ Up to millions of records | ⭐⭐⭐⭐⭐ Petabyte scale |
| **ACID Transactions** | ⭐⭐⭐⭐⭐ Full support | ⭐⭐⭐⭐⭐ Full support |
| **Community Support** | ⭐⭐⭐⭐⭐ Massive community | ⭐⭐⭐ Smaller, growing |

### 3.2 Cost Comparison (Estimated)

**Workload assumptions:**
- 10 RPS average
- 106k records (~50MB data)
- 1 replica for HA

| Resource | PostgreSQL (s2.micro) | YDB (Serverless) |
|----------|----------------------|------------------|
| Compute | ~$25/month | Pay per request |
| Storage (50GB) | ~$5/month | ~$3/month |
| Backup | Included | Included |
| **Estimated Monthly** | **~$30-50/month** | **~$20-40/month** |

*Note: YDB serverless may be cheaper for low traffic but has less predictable costs.*

### 3.3 Migration Effort Comparison

| Aspect | PostgreSQL | YDB |
|--------|------------|-----|
| Schema Migration | Standard DDL | YQL DDL (different syntax) |
| Data Migration | pg_dump, COPY, tools like pgloader | Custom scripts required |
| Code Changes | Minimal (change driver) | Moderate (new SDK, different API) |
| Testing | Standard patterns | Learn new patterns |
| **Estimated Effort** | **2-3 weeks** | **4-6 weeks** |

### 3.4 Pros and Cons Summary

#### PostgreSQL

**Pros:**
- Industry standard, proven reliability
- Excellent Go ecosystem (pgx, sqlx, GORM)
- Team familiarity (likely)
- Simple operational model
- Rich feature set (JSONB, full-text search, etc.)
- Easy to find developers

**Cons:**
- Vertical scaling has limits (not an issue at this scale)
- Single-region by default (multi-region requires setup)

#### YDB

**Pros:**
- Serverless option for variable workloads
- Designed for massive scale
- Global distribution capabilities
- Auto-sharding

**Cons:**
- Steeper learning curve
- Smaller ecosystem and community
- YQL syntax differences from standard SQL
- Fewer Go ORM options
- More complex for simple use cases

---

## 4. Recommendation

### Winner: **Yandex Managed PostgreSQL**

**Rationale:**

1. **Right-sized solution**: PostgreSQL handles millions of records and thousands of RPS easily. YDB's distributed architecture is designed for petabyte-scale workloads that this application doesn't need.

2. **Lower migration risk**: PostgreSQL is standard SQL, making migration straightforward. The existing repository pattern means minimal code changes.

3. **Better Go ecosystem**: Libraries like `pgx` and `sqlx` are battle-tested with excellent performance. GORM is available if ORM is desired.

4. **Team productivity**: Standard PostgreSQL means faster onboarding, easier debugging, and more community resources.

5. **Cost-effective**: For predictable, low-volume workloads, dedicated PostgreSQL instances are cost-efficient and predictable.

6. **Future optionality**: If scale demands it, PostgreSQL can be replaced with YDB later. The repository pattern makes this feasible.

---

## 5. Migration Plan

### 5.1 Phase Overview

```
Phase 1: Schema Design & Setup (3-4 days)
    ↓
Phase 2: Data Access Layer (5-7 days)
    ↓
Phase 3: Data Migration (2-3 days)
    ↓
Phase 4: Testing & Validation (3-4 days)
    ↓
Phase 5: Cutover & Monitoring (1-2 days)
```

**Total estimated duration: 2-3 weeks**

### 5.2 Phase 1: Schema Design & Database Setup

#### 5.2.1 PostgreSQL Schema

```sql
-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Tags table (lookup table)
CREATE TABLE tags (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tags_name ON tags(name);

-- Mentors table
CREATE TABLE mentors (
    id SERIAL PRIMARY KEY,
    airtable_id VARCHAR(50) UNIQUE,  -- For migration reference, can remove later
    mentor_id INTEGER UNIQUE NOT NULL, -- Original "Id" field
    slug VARCHAR(100) UNIQUE NOT NULL, -- "Alias" field
    name VARCHAR(255) NOT NULL,
    job_title VARCHAR(255),
    workplace VARCHAR(255),
    details TEXT,
    about TEXT,
    competencies TEXT,
    experience VARCHAR(50),
    price VARCHAR(100),
    sessions_count INTEGER DEFAULT 0,
    sort_order INTEGER DEFAULT 0,
    is_visible BOOLEAN DEFAULT false,
    status VARCHAR(50) DEFAULT 'pending',
    auth_token VARCHAR(255),
    calendar_url VARCHAR(500),
    is_new BOOLEAN DEFAULT false,
    image_url VARCHAR(500),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_mentors_slug ON mentors(slug);
CREATE INDEX idx_mentors_mentor_id ON mentors(mentor_id);
CREATE INDEX idx_mentors_is_visible ON mentors(is_visible);
CREATE INDEX idx_mentors_status ON mentors(status);
CREATE INDEX idx_mentors_sort_order ON mentors(sort_order);

-- Mentor-Tags junction table (many-to-many)
CREATE TABLE mentor_tags (
    mentor_id INTEGER REFERENCES mentors(id) ON DELETE CASCADE,
    tag_id INTEGER REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (mentor_id, tag_id)
);

CREATE INDEX idx_mentor_tags_mentor_id ON mentor_tags(mentor_id);
CREATE INDEX idx_mentor_tags_tag_id ON mentor_tags(tag_id);

-- Client requests table
CREATE TABLE client_requests (
    id SERIAL PRIMARY KEY,
    airtable_id VARCHAR(50) UNIQUE,  -- For migration reference
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    telegram VARCHAR(100),
    level VARCHAR(50),
    mentor_id INTEGER REFERENCES mentors(id),
    status VARCHAR(20) DEFAULT 'active',  -- active, in_progress, done
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_client_requests_mentor_id ON client_requests(mentor_id);
CREATE INDEX idx_client_requests_email ON client_requests(email);
CREATE INDEX idx_client_requests_created_at ON client_requests(created_at);
CREATE INDEX idx_client_requests_status ON client_requests(status);
-- Composite index for Telegram bot: get active requests for a mentor
CREATE INDEX idx_client_requests_mentor_status ON client_requests(mentor_id, status);

-- Updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_mentors_updated_at
    BEFORE UPDATE ON mentors
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_client_requests_updated_at
    BEFORE UPDATE ON client_requests
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

#### 5.2.2 Yandex Cloud Setup Tasks

1. Create Managed PostgreSQL cluster
   - Instance type: s2.micro (2 vCPU, 8GB RAM) - sufficient for this workload
   - Storage: 20GB SSD (expandable)
   - PostgreSQL version: 16
   - Enable connection pooling (PgBouncer)
   - Configure automatic backups

2. Network configuration
   - Place in same VPC as application
   - Configure security groups for application access
   - Set up SSL certificates

3. Create database and user
   - Database: `getmentor`
   - Application user with limited privileges

### 5.3 Phase 2: Data Access Layer Refactoring

#### 5.3.1 New Package Structure

```
internal/
├── database/
│   ├── postgres/
│   │   ├── client.go          # Connection pool, health checks
│   │   ├── migrations/        # SQL migration files
│   │   └── queries/           # SQL query files (sqlc)
│   └── migrations.go          # Migration runner
├── repository/
│   ├── mentor_repository.go   # Update to use PostgreSQL
│   ├── client_request_repository.go
│   └── tag_repository.go      # New
```

#### 5.3.2 Key Code Changes

**New PostgreSQL client** (`internal/database/postgres/client.go`):
```go
package postgres

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
    pool *pgxpool.Pool
}

func NewClient(ctx context.Context, connString string) (*Client, error) {
    config, err := pgxpool.ParseConfig(connString)
    if err != nil {
        return nil, err
    }

    config.MaxConns = 10  // Sufficient for 10 RPS
    config.MinConns = 2

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, err
    }

    return &Client{pool: pool}, nil
}
```

**Updated MentorRepository interface** - no changes needed, implementation changes only:
```go
// internal/repository/mentor_repository.go
// Change from Airtable client to PostgreSQL client
type MentorRepository struct {
    db    *postgres.Client
    cache *cache.MentorCache  // Keep caching layer
}

func (r *MentorRepository) GetAll(ctx context.Context) ([]*models.Mentor, error) {
    // Check cache first (existing logic)
    if cached := r.cache.Get(); cached != nil {
        return cached, nil
    }

    // Query PostgreSQL instead of Airtable
    mentors, err := r.db.GetAllMentors(ctx)
    if err != nil {
        return nil, err
    }

    r.cache.Set(mentors)
    return mentors, nil
}
```

#### 5.3.3 Migration Strategy for Code

1. **Dual-write phase** (optional, for zero-downtime):
   - Write to both Airtable and PostgreSQL
   - Read from Airtable
   - Validate data consistency

2. **Switch reads**:
   - Read from PostgreSQL
   - Write to both (for rollback safety)

3. **Complete cutover**:
   - Remove Airtable writes
   - Remove Airtable code

### 5.4 Phase 3: Data Migration

#### 5.4.1 Migration Script Approach

```go
// cmd/migrate/main.go
package main

func main() {
    // 1. Connect to both Airtable and PostgreSQL
    airtable := airtable.NewClient(...)
    postgres := postgres.NewClient(...)

    // 2. Migrate Tags first (dependency)
    tags := airtable.GetAllTags()
    for name, airtableID := range tags {
        postgres.InsertTag(name, airtableID)
    }

    // 3. Migrate Mentors
    mentors := airtable.GetAllMentors()
    for _, m := range mentors {
        pgMentor := convertMentor(m)
        mentorID := postgres.InsertMentor(pgMentor)

        // 4. Create mentor-tag associations
        for _, tagName := range m.Tags {
            tagID := postgres.GetTagIDByName(tagName)
            postgres.InsertMentorTag(mentorID, tagID)
        }
    }

    // 5. Migrate Client Requests
    // Note: This requires pagination due to volume (100k records)
    offset := 0
    batchSize := 1000
    for {
        requests := airtable.GetClientRequests(offset, batchSize)
        if len(requests) == 0 {
            break
        }

        for _, r := range requests {
            mentorID := postgres.GetMentorIDByAirtableID(r.MentorAirtableID)
            postgres.InsertClientRequest(r, mentorID)
        }

        offset += batchSize
    }

    // 6. Validate counts
    validateMigration(airtable, postgres)
}
```

#### 5.4.2 Data Validation Checks

- Record counts match
- All mentor IDs preserved
- All slugs preserved
- All relationships intact
- Sample data spot checks

### 5.5 Phase 4: Testing & Validation

#### 5.5.1 Test Categories

1. **Unit Tests**: Repository layer with mock database
2. **Integration Tests**: Against test PostgreSQL instance
3. **Load Tests**: Verify performance at 10+ RPS
4. **API Compatibility Tests**: Ensure API responses unchanged

#### 5.5.2 Key Test Scenarios

| Scenario | Expected Result |
|----------|-----------------|
| Get all mentors | Same response as Airtable |
| Get mentor by slug | Same mentor data |
| Create client request | Record created, returns calendar URL |
| Update mentor profile | Fields updated correctly |
| Cache refresh | Cache populated from PostgreSQL |
| Get requests by mentor (bot) | Returns filtered requests with status |
| Update request status (bot) | Status updated, updated_at changed |

### 5.6 Phase 5: Cutover & Monitoring

#### 5.6.1 Cutover Steps

1. **Freeze Airtable writes** (maintenance window)
2. **Run final data sync**
3. **Deploy PostgreSQL-enabled application**
4. **Verify application health**
5. **Monitor for errors**
6. **Remove Airtable code** (follow-up PR)

#### 5.6.2 Rollback Plan

If issues occur:
1. Revert deployment to Airtable version
2. Airtable data is still authoritative
3. Investigate issues
4. Re-attempt migration

#### 5.6.3 Monitoring Additions

Add Prometheus metrics for PostgreSQL:
- Connection pool usage
- Query latency
- Error rates

---

## 6. Files to Modify

### 6.1 Files to Remove (after migration)

| File | Purpose |
|------|---------|
| `pkg/airtable/client.go` | Airtable API client |
| `pkg/airtable/client_test.go` | Airtable tests |
| `internal/cache/tags_cache.go` | Can simplify if tags rarely change |

### 6.2 Files to Create

| File | Purpose |
|------|---------|
| `internal/database/postgres/client.go` | PostgreSQL connection pool |
| `internal/database/postgres/mentor_queries.go` | Mentor SQL queries |
| `internal/database/postgres/request_queries.go` | Client request queries (incl. status updates) |
| `internal/database/migrations/*.sql` | Database migrations |
| `cmd/migrate/main.go` | Data migration script |
| `internal/handlers/request_handler.go` | API endpoints for Telegram bot |
| `internal/services/request_service.go` | Business logic for request operations |

### 6.3 Files to Modify

| File | Changes |
|------|---------|
| `config/config.go` | Add PostgreSQL config, remove Airtable |
| `cmd/api/main.go` | Initialize PostgreSQL client |
| `internal/repository/mentor_repository.go` | Use PostgreSQL instead of Airtable |
| `internal/repository/client_request_repository.go` | Use PostgreSQL |
| `internal/cache/mentor_cache.go` | Update data source calls |
| `go.mod` | Add pgx, remove airtable SDK |

---

## 7. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Data loss during migration | Low | High | Full backup, validation scripts |
| Performance regression | Low | Medium | Load testing before cutover |
| API compatibility issues | Low | High | Comprehensive API tests |
| Rollback needed | Low | Medium | Keep Airtable active during transition |
| Learning curve | Low | Low | PostgreSQL is well-known |

---

## 8. Success Criteria

- [ ] All 6,000 mentors migrated with data integrity
- [ ] All 100,000 client requests migrated (including status field)
- [ ] API response format unchanged
- [ ] Response latency ≤ current (with caching)
- [ ] Zero data loss
- [ ] Successful load test at 10+ RPS
- [ ] Monitoring and alerting in place
- [ ] Telegram bot API endpoints functional
- [ ] Bot can query requests by mentor and update status

---

## 9. Stakeholder Clarifications (Resolved)

The following questions have been answered:

| Question | Answer | Impact |
|----------|--------|--------|
| **Client Requests access** | Telegram chatbot reads requests by mentor and updates status (active → in_progress → done) | Added status field, indexes, and API considerations |
| **Mentor updates** | Self-edit via profile page + manual admin edits via Airtable UI | **Future requirement:** Admin interface needed post-migration |
| **Image storage** | Already in Yandex Object Storage | No migration needed for images |
| **Downtime tolerance** | Some downtime is acceptable | Simplifies cutover (no dual-write needed) |
| **Tags management** | Tags are mostly static, rarely change | Can simplify tags handling, no complex sync needed |

## 10. Additional Scope Considerations

### 10.1 Telegram Bot Migration (Required)

The Telegram chatbot currently accesses Airtable directly. Post-migration options:

**Option A: Direct PostgreSQL access**
- Bot connects to PostgreSQL directly
- Simpler implementation
- Requires network access to database

**Option B: API endpoints in this service**
- Add REST endpoints for bot operations:
  - `GET /api/v1/requests?mentor_id={id}&status={status}`
  - `PATCH /api/v1/requests/{id}/status`
- Better separation of concerns
- Existing auth patterns can be reused

**Recommendation:** Option B (API endpoints) - cleaner architecture, single point of database access.

### 10.2 Admin Interface (Future - Out of Scope)

After migration, you'll lose Airtable's convenient UI for manual data edits. Future options:

1. **Minimal CLI tool** - Quick to build, script-based admin operations
2. **Simple web admin panel** - React/Vue dashboard with basic CRUD
3. **Off-the-shelf solution** - Tools like [pgAdmin](https://www.pgadmin.org/), [Retool](https://retool.com/), or [Directus](https://directus.io/)
4. **Database IDE** - Use DataGrip, DBeaver, or similar for direct SQL access

**Note:** This is explicitly out of scope for this migration but should be planned as a follow-up.

---

## Appendix A: Yandex Cloud PostgreSQL Pricing Reference

| Resource | Specification | Est. Monthly Cost |
|----------|---------------|-------------------|
| Compute | s2.micro (2 vCPU, 8GB) | ~$25 |
| Storage | 20GB SSD | ~$3 |
| Backup | 7-day retention | Included |
| Network | Same VPC | Free |
| **Total** | | **~$28-35/month** |

## Appendix B: Alternative - YDB If Requirements Change

If future requirements include:
- Multi-region deployment
- 1000+ RPS
- Millions of records

Consider revisiting YDB. The repository pattern in the codebase makes switching databases feasible.
