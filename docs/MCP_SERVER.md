# MCP (Model Context Protocol) Server

This document describes the MCP server implementation for the GetMentor API, which allows AI tools to search and retrieve mentor information.

## Overview

The MCP server implements the Model Context Protocol using JSON-RPC 2.0. It provides two main tools:
1. **search_mentors** - Search for mentors with flexible filtering
2. **get_mentor_details** - Get detailed information about a specific mentor

## Endpoint

**Internal Route:** `POST /api/internal/mcp`
**Public Route:** Will be configured via Traefik (user-defined)

## Authentication

All MCP requests must include the `X-MCP-API-Token` header with a valid token.

**Environment Variable:** `MCP_API_TOKEN`

Example:
```bash
curl -X POST https://your-domain.com/api/internal/mcp \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: your-secret-token" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize",...}'
```

## Rate Limiting

- **Rate:** 20 requests per second
- **Burst:** 40 requests
- Exceeding the limit returns HTTP 429 with a JSON-RPC error

## Protocol Flow

### 1. Initialize

Client initiates connection and receives server capabilities.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "my-ai-tool",
      "version": "1.0.0"
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {}
    },
    "serverInfo": {
      "name": "getmentor-mcp-server",
      "version": "1.0.0"
    }
  }
}
```

### 2. List Tools

Get available tools and their schemas.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "search_mentors",
        "description": "Search for mentors based on keywords and filters...",
        "inputSchema": {
          "type": "object",
          "properties": {
            "keywords": {
              "type": "array",
              "description": "Keywords to search...",
              "items": {"type": "string"}
            },
            "name": {"type": "string", "description": "Filter by exact mentor name"},
            "workplace": {"type": "string", "description": "Filter by exact workplace"},
            "experience": {"type": "string", "description": "Filter by experience level"},
            "price": {"type": "string", "description": "Filter by price"},
            "tags": {
              "type": "array",
              "description": "Filter by tags...",
              "items": {"type": "string"}
            },
            "limit": {
              "type": "integer",
              "description": "Maximum number of results...",
              "default": 50
            }
          }
        }
      },
      {
        "name": "get_mentor_details",
        "description": "Get detailed information about a specific mentor...",
        "inputSchema": {
          "type": "object",
          "properties": {
            "id": {"type": "integer", "description": "Mentor ID"},
            "slug": {"type": "string", "description": "Mentor slug"}
          }
        }
      }
    ]
  }
}
```

### 3. Call Tool

Execute a specific tool.

#### search_mentors

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "search_mentors",
    "arguments": {
      "keywords": ["react", "javascript"],
      "tags": ["frontend"],
      "limit": 10
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "[\n  {\n    \"id\": 1,\n    \"name\": \"John Doe\",\n    \"job_title\": \"Senior Frontend Developer\",\n    \"workplace\": \"Tech Corp\",\n    \"experience\": \"5+ years\",\n    \"price\": \"5000₽\",\n    \"tags\": [\"frontend\", \"react\"],\n    \"competencies\": \"React, JavaScript, TypeScript\",\n    \"slug\": \"john-doe\"\n  }\n]"
      }
    ]
  }
}
```

#### get_mentor_details

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "get_mentor_details",
    "arguments": {
      "slug": "john-doe"
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\n  \"id\": 1,\n  \"name\": \"John Doe\",\n  \"job_title\": \"Senior Frontend Developer\",\n  \"workplace\": \"Tech Corp\",\n  \"experience\": \"5+ years\",\n  \"price\": \"5000₽\",\n  \"tags\": [\"frontend\", \"react\"],\n  \"competencies\": \"React, JavaScript, TypeScript\",\n  \"description\": \"Expert in building modern web applications\",\n  \"about\": \"10 years of experience in software development...\",\n  \"photo_url\": \"https://example.com/photo.jpg\",\n  \"slug\": \"john-doe\",\n  \"link\": \"https://гетментор.рф/mentor/john-doe\"\n}"
      }
    ]
  }
}
```

## Search Functionality

### Keyword Search
- **Fields searched:** competencies, description, about
- **Matching:** Case-insensitive substring match
- **Logic:** Any keyword matches (OR logic)

### Deterministic Filters
These filters require exact matches:
- `name` - Exact mentor name
- `workplace` - Exact workplace name
- `experience` - Exact experience level
- `price` - Exact price

### Tags Filter
- Match at least one tag (OR logic)

### Example Combinations

**Search for React mentors at a specific company:**
```json
{
  "keywords": ["react"],
  "workplace": "Google"
}
```

**Search for mentors with specific tags and experience:**
```json
{
  "tags": ["backend", "golang"],
  "experience": "5+ years",
  "limit": 20
}
```

## Error Handling

All errors follow JSON-RPC 2.0 error format:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid Request"
  }
}
```

### Error Codes

| Code | Name | Description | HTTP Status |
|------|------|-------------|-------------|
| -32700 | Parse Error | Invalid JSON | 400 |
| -32600 | Invalid Request | Invalid JSON-RPC | 400 |
| -32601 | Method Not Found | Unknown method | 400 |
| -32602 | Invalid Params | Invalid parameters | 400 |
| -32603 | Internal Error | Server error | 500 |
| -32000 | Authentication Error | Missing/invalid token | 401 |
| -32001 | Rate Limit Error | Too many requests | 429 |
| -32002 | Not Found Error | Mentor not found | 404 |

## Observability

### Metrics

The MCP server exposes Prometheus metrics at `/api/metrics`:

**Tool Invocation Counter:**
- Metric: `gm_api_mcp_tool_invocations_total`
- Labels: `tool_name`, `status`

**Tool Duration Histogram:**
- Metric: `gm_api_mcp_tool_duration_seconds`
- Labels: `tool_name`

**Error Counter:**
- Metric: `gm_api_mcp_errors_total`
- Labels: `error_type`

### Logging

All MCP requests and errors are logged with:
- Request method and ID
- Client IP address
- Tool name and arguments (for tool calls)
- Error details (for failures)

Log level: INFO for successful requests, WARN/ERROR for failures

## Configuration

### Environment Variables

Add to your `.env` file:

```bash
# MCP API Token (required)
MCP_API_TOKEN=your-secure-token-here
```

### Traefik Configuration

Example Traefik configuration to expose the MCP endpoint publicly:

```yaml
http:
  routers:
    mcp-router:
      rule: "Host(`api.yourdomain.com`) && Path(`/mcp`)"
      service: mcp-service
      middlewares:
        - mcp-headers

  services:
    mcp-service:
      loadBalancer:
        servers:
          - url: "http://getmentor-api:8081/api/internal/mcp"

  middlewares:
    mcp-headers:
      headers:
        customRequestHeaders:
          X-Forwarded-Proto: "https"
```

This routes `https://api.yourdomain.com/mcp` → `http://getmentor-api:8081/api/internal/mcp`

## Testing

### Manual Testing with curl

```bash
# 1. Initialize
curl -X POST http://localhost:8081/api/internal/mcp \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: your-token" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "test", "version": "1.0"}
    }
  }'

# 2. List tools
curl -X POST http://localhost:8081/api/internal/mcp \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: your-token" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }'

# 3. Search mentors
curl -X POST http://localhost:8081/api/internal/mcp \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: your-token" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "search_mentors",
      "arguments": {
        "keywords": ["react"],
        "limit": 5
      }
    }
  }'

# 4. Get mentor details
curl -X POST http://localhost:8081/api/internal/mcp \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: your-token" \
  -d '{
    "jsonrpc": "2.0",
    "id": 4,
    "method": "tools/call",
    "params": {
      "name": "get_mentor_details",
      "arguments": {
        "id": 1
      }
    }
  }'
```

## Security Considerations

1. **Token Security**: Keep `MCP_API_TOKEN` secret and rotate regularly
2. **Rate Limiting**: Built-in protection against abuse (20 req/sec, burst 40)
3. **HTTPS**: Always use HTTPS in production via Traefik
4. **IP Whitelisting**: Consider adding IP restrictions in Traefik for additional security
5. **Monitoring**: Monitor metrics for unusual patterns or abuse

## Architecture

```
┌─────────────┐
│   AI Tool   │
└──────┬──────┘
       │ HTTPS (Traefik)
       ↓
┌─────────────────────────┐
│   Traefik (Proxy)       │
│   /mcp → /internal/mcp  │
└──────┬──────────────────┘
       │ HTTP (Internal)
       ↓
┌─────────────────────────┐
│   MCP Handler           │
│   - Auth Middleware     │
│   - Rate Limit          │
│   - Observability       │
└──────┬──────────────────┘
       │
       ↓
┌─────────────────────────┐
│   MCP Server            │
│   - Protocol Handler    │
│   - Tool Router         │
└──────┬──────────────────┘
       │
       ↓
┌─────────────────────────┐
│   Mentor Service        │
│   - Search Logic        │
│   - Data Retrieval      │
└─────────────────────────┘
```

## Implementation Details

### Files Created

- `internal/mcp/models.go` - JSON-RPC and MCP protocol types
- `internal/mcp/server.go` - Core MCP server implementation
- `internal/handlers/mcp_handler.go` - HTTP handler
- `internal/middleware/mcp_auth.go` - Authentication middleware
- `pkg/metrics/metrics.go` - MCP-specific metrics (updated)
- `config/config.go` - MCP configuration (updated)

### Key Features

- **Custom Implementation**: No third-party MCP libraries, built from scratch
- **JSON-RPC 2.0**: Full compliance with JSON-RPC 2.0 specification
- **Comprehensive Observability**: Metrics, logging, and tracing
- **Production Ready**: Rate limiting, auth, error handling
- **Flexible Search**: Keyword + filter-based search with case-insensitive matching
