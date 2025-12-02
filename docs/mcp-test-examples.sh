#!/bin/bash

# MCP Server Test Examples
# This script provides example curl commands to test the MCP server

# Configuration
MCP_URL="http://localhost:8081/api/internal/mcp"
MCP_TOKEN="your-mcp-token-here"  # Set this to your actual MCP_API_TOKEN

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== MCP Server Test Examples ===${NC}\n"

# Test 1: Initialize
echo -e "${GREEN}Test 1: Initialize${NC}"
curl -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: $MCP_TOKEN" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {
        "name": "test-client",
        "version": "1.0.0"
      }
    }
  }' | jq .

echo -e "\n"

# Test 2: List Tools
echo -e "${GREEN}Test 2: List Tools${NC}"
curl -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: $MCP_TOKEN" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }' | jq .

echo -e "\n"

# Test 3: Search Mentors (simple keyword search)
echo -e "${GREEN}Test 3: Search Mentors - Keyword Search${NC}"
curl -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: $MCP_TOKEN" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "search_mentors",
      "arguments": {
        "keywords": ["react", "javascript"],
        "limit": 5
      }
    }
  }' | jq .

echo -e "\n"

# Test 4: Search Mentors with filters
echo -e "${GREEN}Test 4: Search Mentors - With Filters${NC}"
curl -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: $MCP_TOKEN" \
  -d '{
    "jsonrpc": "2.0",
    "id": 4,
    "method": "tools/call",
    "params": {
      "name": "search_mentors",
      "arguments": {
        "keywords": ["backend"],
        "tags": ["golang", "python"],
        "limit": 10
      }
    }
  }' | jq .

echo -e "\n"

# Test 5: Search All Active Mentors
echo -e "${GREEN}Test 5: Get All Active Mentors${NC}"
curl -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: $MCP_TOKEN" \
  -d '{
    "jsonrpc": "2.0",
    "id": 5,
    "method": "tools/call",
    "params": {
      "name": "search_mentors",
      "arguments": {
        "limit": 50
      }
    }
  }' | jq .

echo -e "\n"

# Test 6: Get Mentor Details by ID
echo -e "${GREEN}Test 6: Get Mentor Details by ID${NC}"
curl -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: $MCP_TOKEN" \
  -d '{
    "jsonrpc": "2.0",
    "id": 6,
    "method": "tools/call",
    "params": {
      "name": "get_mentor_details",
      "arguments": {
        "id": 1
      }
    }
  }' | jq .

echo -e "\n"

# Test 7: Get Mentor Details by Slug
echo -e "${GREEN}Test 7: Get Mentor Details by Slug${NC}"
curl -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: $MCP_TOKEN" \
  -d '{
    "jsonrpc": "2.0",
    "id": 7,
    "method": "tools/call",
    "params": {
      "name": "get_mentor_details",
      "arguments": {
        "slug": "john-doe"
      }
    }
  }' | jq .

echo -e "\n"

# Test 8: Error - Missing Auth Token
echo -e "${GREEN}Test 8: Error - Missing Auth Token${NC}"
curl -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 8,
    "method": "tools/list"
  }' | jq .

echo -e "\n"

# Test 9: Error - Invalid Method
echo -e "${GREEN}Test 9: Error - Invalid Method${NC}"
curl -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: $MCP_TOKEN" \
  -d '{
    "jsonrpc": "2.0",
    "id": 9,
    "method": "invalid/method"
  }' | jq .

echo -e "\n"

# Test 10: Error - Invalid Tool Name
echo -e "${GREEN}Test 10: Error - Invalid Tool Name${NC}"
curl -X POST "$MCP_URL" \
  -H "Content-Type: application/json" \
  -H "X-MCP-API-Token: $MCP_TOKEN" \
  -d '{
    "jsonrpc": "2.0",
    "id": 10,
    "method": "tools/call",
    "params": {
      "name": "invalid_tool",
      "arguments": {}
    }
  }' | jq .

echo -e "\n${BLUE}=== Tests Complete ===${NC}\n"
