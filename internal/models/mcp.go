package models

// JSON-RPC 2.0 Request
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"` // Must be "2.0"
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
	ID      interface{}            `json:"id"` // Can be string, number, or null
}

// JSON-RPC 2.0 Response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"` // Must be "2.0"
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// JSON-RPC 2.0 Error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// MCP Tool definitions following MCP protocol
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCP list_mentors parameters
type ListMentorsParams struct {
	Tags       []string `json:"tags,omitempty"`       // Filter by tags
	Experience string   `json:"experience,omitempty"` // Filter by experience level
	MinPrice   string   `json:"minPrice,omitempty"`   // Minimum price (inclusive)
	MaxPrice   string   `json:"maxPrice,omitempty"`   // Maximum price (inclusive)
	Workplace  string   `json:"workplace,omitempty"`  // Filter by workplace
	Limit      int      `json:"limit,omitempty"`      // Limit results (default: 50, max: 200)
}

// MCP get_mentor parameters
type GetMentorParams struct {
	ID   *int    `json:"id,omitempty"`   // Mentor ID
	Slug *string `json:"slug,omitempty"` // Mentor slug
}

// MCP search_mentors parameters
type SearchMentorsParams struct {
	Query      string   `json:"query"`                // Search keywords (space-separated)
	Tags       []string `json:"tags,omitempty"`       // Filter by tags
	Experience string   `json:"experience,omitempty"` // Filter by experience level
	MinPrice   string   `json:"minPrice,omitempty"`   // Minimum price (inclusive)
	MaxPrice   string   `json:"maxPrice,omitempty"`   // Maximum price (inclusive)
	Workplace  string   `json:"workplace,omitempty"`  // Filter by workplace
	Limit      int      `json:"limit,omitempty"`      // Limit results (default: 20, max: 100)
}

// MCP Mentor response (basic info for list_mentors)
type MCPMentorBasic struct {
	ID           int      `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	JobTitle     string   `json:"jobTitle"`
	Workplace    string   `json:"workplace"`
	Experience   string   `json:"experience"`
	Tags         []string `json:"tags"`
	Competencies string   `json:"competencies"`
	Price        string   `json:"price"`
	PhotoURL     string   `json:"photoUrl,omitempty"`
	DoneSessions int      `json:"doneSessions"`
}

// MCP Mentor response (extended info for get_mentor and search results)
type MCPMentorExtended struct {
	ID           int      `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	JobTitle     string   `json:"jobTitle"`
	Workplace    string   `json:"workplace"`
	Experience   string   `json:"experience"`
	Tags         []string `json:"tags"`
	Competencies string   `json:"competencies"`
	Price        string   `json:"price"`
	PhotoURL     string   `json:"photoUrl,omitempty"`
	DoneSessions int      `json:"doneSessions"`
	Description  string   `json:"description"`
	About        string   `json:"about"`
}

// Response structures
type ListMentorsResult struct {
	Mentors []MCPMentorBasic `json:"mentors"`
	Count   int              `json:"count"`
}

type GetMentorResult struct {
	Mentor *MCPMentorExtended `json:"mentor"`
}

type SearchMentorsResult struct {
	Mentors []MCPMentorExtended `json:"mentors"`
	Count   int                 `json:"count"`
}

// Helper function to convert Mentor to MCPMentorBasic
func (m *Mentor) ToMCPBasic() MCPMentorBasic {
	return MCPMentorBasic{
		ID:           m.ID,
		Slug:         m.Slug,
		Name:         m.Name,
		JobTitle:     m.Job,
		Workplace:    m.Workplace,
		Experience:   m.Experience,
		Tags:         m.Tags,
		Competencies: m.Competencies,
		Price:        m.Price,
		PhotoURL:     m.PhotoURL,
		DoneSessions: m.MenteeCount,
	}
}

// Helper function to convert Mentor to MCPMentorExtended
func (m *Mentor) ToMCPExtended() MCPMentorExtended {
	return MCPMentorExtended{
		ID:           m.ID,
		Slug:         m.Slug,
		Name:         m.Name,
		JobTitle:     m.Job,
		Workplace:    m.Workplace,
		Experience:   m.Experience,
		Tags:         m.Tags,
		Competencies: m.Competencies,
		Price:        m.Price,
		PhotoURL:     m.PhotoURL,
		DoneSessions: m.MenteeCount,
		Description:  m.Description,
		About:        m.About,
	}
}
