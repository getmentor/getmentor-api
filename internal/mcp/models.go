package mcp

// JSON-RPC 2.0 protocol structures for Model Context Protocol (MCP)

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// MCP-specific error codes
const (
	AuthenticationError = -32000
	RateLimitError      = -32001
	NotFoundError       = -32002
)

// InitializeParams represents parameters for initialize request
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

// ClientInfo represents client information
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult represents the result of initialize request
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

// Capabilities represents server capabilities
type Capabilities struct {
	Tools map[string]interface{} `json:"tools,omitempty"`
}

// ServerInfo represents server information
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsListResult represents the result of tools/list request
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema represents JSON Schema for tool input
type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]Property    `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Additional map[string]interface{} `json:"additionalProperties,omitempty"`
}

// Property represents a JSON Schema property
type Property struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Items       *Items      `json:"items,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// Items represents JSON Schema array items
type Items struct {
	Type string `json:"type"`
}

// ToolCallParams represents parameters for tools/call request
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolCallResult represents the result of tools/call request
type ToolCallResult struct {
	Content []Content `json:"content"`
}

// Content represents tool result content
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// SearchMentorsArgs represents arguments for search_mentors tool
type SearchMentorsArgs struct {
	Keywords   []string `json:"keywords,omitempty"`
	Name       string   `json:"name,omitempty"`
	Workplace  string   `json:"workplace,omitempty"`
	Experience string   `json:"experience,omitempty"`
	Price      string   `json:"price,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

// GetMentorDetailsArgs represents arguments for get_mentor_details tool
type GetMentorDetailsArgs struct {
	ID   int    `json:"id,omitempty"`
	Slug string `json:"slug,omitempty"`
}

// MentorSearchResult represents a mentor in search results
type MentorSearchResult struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	JobTitle     string   `json:"job_title"`
	Workplace    string   `json:"workplace"`
	Experience   string   `json:"experience"`
	Price        string   `json:"price"`
	Tags         []string `json:"tags"`
	Competencies string   `json:"competencies"`
	Slug         string   `json:"slug"`
}

// MentorDetailsResult represents detailed mentor information
type MentorDetailsResult struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	JobTitle     string   `json:"job_title"`
	Workplace    string   `json:"workplace"`
	Experience   string   `json:"experience"`
	Price        string   `json:"price"`
	Tags         []string `json:"tags"`
	Competencies string   `json:"competencies"`
	Description  string   `json:"description"`
	About        string   `json:"about"`
	PhotoURL     string   `json:"photo_url"`
	Slug         string   `json:"slug"`
	Link         string   `json:"link"`
}
