# GetMentor MCP Server

Model Context Protocol (MCP) server for searching and retrieving mentor information from GetMentor API.

## Overview

The MCP server provides AI tools with access to mentor search functionality following the JSON-RPC 2.0 protocol. It enables AI assistants to:

- List all active mentors with filtering capabilities
- Search mentors by keywords
- Retrieve detailed mentor information

## Endpoint

**URL:** `POST /api/internal/mcp`

**Authentication:** Bearer token in `mentors_api_auth_token` header

## Environment Configuration

Add the following to your `.env` file:

```bash
MCP_AUTH_TOKEN=your_secure_mcp_token_here
```

## MCP Protocol

The server implements MCP over HTTP using JSON-RPC 2.0.

### Initialization

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "params": {},
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {}
    },
    "serverInfo": {
      "name": "getmentor-mcp-server",
      "version": "1.0.0"
    }
  },
  "id": 1
}
```

### List Available Tools

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "params": {},
  "id": 2
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "tools": [
      {
        "name": "list_mentors",
        "description": "List all active mentors with optional filtering...",
        "inputSchema": { ... }
      },
      {
        "name": "get_mentor",
        "description": "Get detailed information about a specific mentor...",
        "inputSchema": { ... }
      },
      {
        "name": "search_mentors",
        "description": "Search for mentors by keywords...",
        "inputSchema": { ... }
      }
    ]
  },
  "id": 2
}
```

## Available Tools

### 1. list_mentors

Lists all active mentors with optional filtering.

**Parameters:**
- `tags` (array, optional): Filter by mentor tags (e.g., `["Python", "Machine Learning"]`)
- `experience` (string, optional): Filter by experience level (e.g., "Senior", "Middle")
- `minPrice` (string, optional): Minimum price filter (inclusive)
- `maxPrice` (string, optional): Maximum price filter (inclusive)
- `workplace` (string, optional): Filter by workplace/company name
- `limit` (integer, optional): Maximum results (default: 50, max: 200)

**Example Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "list_mentors",
    "arguments": {
      "tags": ["Python", "Backend"],
      "experience": "Senior",
      "limit": 10
    }
  },
  "id": 3
}
```

**Example Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Found 5 mentors matching the criteria."
      }
    ],
    "isError": false,
    "_meta": {
      "mentors": [
        {
          "id": 1,
          "slug": "ivan-petrov",
          "name": "Ivan Petrov",
          "jobTitle": "Senior Backend Engineer",
          "workplace": "Yandex",
          "experience": "Senior (10+ years)",
          "tags": ["Python", "Backend", "Go"],
          "competencies": "Python, Go, PostgreSQL, Redis",
          "price": "5000₽/час",
          "photoUrl": "https://...",
          "doneSessions": 42
        }
      ],
      "count": 5
    }
  },
  "id": 3
}
```

### 2. get_mentor

Retrieves detailed information about a specific mentor.

**Parameters:**
- `id` (integer, optional): Mentor ID
- `slug` (string, optional): Mentor slug (URL-friendly identifier)

*Note: One of `id` or `slug` must be provided.*

**Example Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_mentor",
    "arguments": {
      "slug": "ivan-petrov"
    }
  },
  "id": 4
}
```

**Example Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Found mentor: Ivan Petrov (Senior Backend Engineer at Yandex)"
      }
    ],
    "isError": false,
    "_meta": {
      "mentor": {
        "id": 1,
        "slug": "ivan-petrov",
        "name": "Ivan Petrov",
        "jobTitle": "Senior Backend Engineer",
        "workplace": "Yandex",
        "experience": "Senior (10+ years)",
        "tags": ["Python", "Backend", "Go"],
        "competencies": "Python, Go, PostgreSQL, Redis, Docker, Kubernetes",
        "price": "5000₽/час",
        "photoUrl": "https://...",
        "doneSessions": 42,
        "description": "Experienced backend engineer specializing in high-load systems...",
        "about": "I have 12 years of experience building scalable backend systems..."
      }
    }
  },
  "id": 4
}
```

### 3. search_mentors

Searches mentors by keywords in competencies, description, and about fields.

**Parameters:**
- `query` (string, **required**): Search keywords (space-separated)
- `tags` (array, optional): Filter by mentor tags
- `experience` (string, optional): Filter by experience level
- `minPrice` (string, optional): Minimum price filter
- `maxPrice` (string, optional): Maximum price filter
- `workplace` (string, optional): Filter by workplace
- `limit` (integer, optional): Maximum results (default: 20, max: 100)

**Example Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "search_mentors",
    "arguments": {
      "query": "machine learning tensorflow",
      "tags": ["Python"],
      "limit": 5
    }
  },
  "id": 5
}
```

**Example Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Found 3 mentors matching search query 'machine learning tensorflow'."
      }
    ],
    "isError": false,
    "_meta": {
      "mentors": [
        {
          "id": 15,
          "slug": "anna-ivanova",
          "name": "Anna Ivanova",
          "jobTitle": "ML Engineer",
          "workplace": "Google",
          "experience": "Senior (8 years)",
          "tags": ["Python", "Machine Learning", "TensorFlow"],
          "competencies": "Python, TensorFlow, PyTorch, Scikit-learn",
          "price": "6000₽/час",
          "photoUrl": "https://...",
          "doneSessions": 28,
          "description": "Machine learning engineer with expertise in deep learning...",
          "about": "I specialize in building ML models using TensorFlow and PyTorch..."
        }
      ],
      "count": 3,
      "query": "machine learning tensorflow"
    }
  },
  "id": 5
}
```

## Error Handling

The server returns JSON-RPC 2.0 error responses:

**Error Response:**
```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32602,
    "message": "Invalid parameters",
    "data": "query parameter is required"
  },
  "id": 6
}
```

**Error Codes:**
- `-32700`: Parse error (invalid JSON)
- `-32600`: Invalid request (missing required fields)
- `-32601`: Method not found (unknown method)
- `-32602`: Invalid params (parameter validation failed)
- `-32603`: Internal error (server error)

## Rate Limiting

- **Rate limit:** 20 requests/second
- **Burst:** 40 requests
- **Scope:** Per IP address

Exceeding the rate limit returns HTTP 429 (Too Many Requests).

## Testing with cURL

### Initialize
```bash
curl -X POST http://localhost:8081/api/internal/mcp \
  -H "Content-Type: application/json" \
  -H "mentors_api_auth_token: your_mcp_token" \
  -d '{
    "jsonrpc": "2.0",
    "method": "initialize",
    "params": {},
    "id": 1
  }'
```

### List Tools
```bash
curl -X POST http://localhost:8081/api/internal/mcp \
  -H "Content-Type: application/json" \
  -H "mentors_api_auth_token: your_mcp_token" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/list",
    "params": {},
    "id": 2
  }'
```

### Search Mentors
```bash
curl -X POST http://localhost:8081/api/internal/mcp \
  -H "Content-Type: application/json" \
  -H "mentors_api_auth_token: your_mcp_token" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "search_mentors",
      "arguments": {
        "query": "python backend",
        "limit": 5
      }
    },
    "id": 3
  }'
```

## Observability

### Metrics

The MCP server exposes Prometheus metrics at `/api/metrics`:

**MCP-specific metrics:**
- `gm_api_mcp_request_total{method, status}` - Total MCP requests
- `gm_api_mcp_request_duration_seconds{method}` - Request duration
- `gm_api_mcp_tool_invocations_total{tool, status}` - Tool invocation count
- `gm_api_mcp_search_keywords_total{keyword_count_range}` - Search keyword distribution
- `gm_api_mcp_results_returned{tool}` - Number of results returned

**Example:**
```bash
curl http://localhost:8081/api/metrics | grep mcp
```

### Logging

All MCP requests are logged with structured logging:
- Request method and parameters
- Request duration
- Result counts
- Errors with stack traces

Logs are written to `/app/logs/` and indexed in Grafana Loki.

## Traefik Configuration

Example Traefik route configuration:

```yaml
http:
  routers:
    mcp-server:
      rule: "Host(`mcp.getmentor.com`) && Path(`/mcp`)"
      service: getmentor-api
      middlewares:
        - https-redirect
        - rate-limit
      tls:
        certResolver: letsencrypt

  services:
    getmentor-api:
      loadBalancer:
        servers:
          - url: "http://backend:8081/api/internal/mcp"

  middlewares:
    rate-limit:
      rateLimit:
        average: 100
        burst: 200
```

## Security Considerations

1. **Authentication**: Always use strong, random tokens for `MCP_AUTH_TOKEN`
2. **Rate Limiting**: Adjust rate limits based on expected AI tool usage patterns
3. **Network Isolation**: Use Traefik/proxy for public exposure, keep API internal
4. **Monitoring**: Set up alerts for unusual request patterns
5. **Token Rotation**: Regularly rotate MCP auth tokens

## AI Tool Integration

### Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "getmentor": {
      "url": "https://mcp.getmentor.com/mcp",
      "transport": "http",
      "headers": {
        "mentors_api_auth_token": "your_mcp_token"
      }
    }
  }
}
```

### Custom Integration

```python
import requests

MCP_URL = "https://mcp.getmentor.com/mcp"
MCP_TOKEN = "your_mcp_token"

def search_mentors(query, tags=None, limit=10):
    payload = {
        "jsonrpc": "2.0",
        "method": "tools/call",
        "params": {
            "name": "search_mentors",
            "arguments": {
                "query": query,
                "tags": tags or [],
                "limit": limit
            }
        },
        "id": 1
    }

    headers = {
        "Content-Type": "application/json",
        "mentors_api_auth_token": MCP_TOKEN
    }

    response = requests.post(MCP_URL, json=payload, headers=headers)
    return response.json()

# Example usage
result = search_mentors("python backend", tags=["Python"], limit=5)
mentors = result["result"]["_meta"]["mentors"]
```

## Architecture

```
AI Tool/Client
    ↓ (HTTPS with auth token)
Traefik Proxy
    ↓
GetMentor API (/api/internal/mcp)
    ↓
MCP Handler (JSON-RPC 2.0)
    ↓
MCP Service (Search & Filter)
    ↓
Mentor Repository
    ↓
Cache Layer (10min TTL)
    ↓
Airtable API
```

## Support

For issues or questions:
- Check logs: `docker logs getmentor-api`
- View metrics: `http://localhost:8081/api/metrics`
- GitHub Issues: [getmentor/getmentor-api/issues](https://github.com/getmentor/getmentor-api/issues)
