package gui

import "time"

// Message types
const (
	TypeRequest  = "request"
	TypeResponse = "response"
	TypeEvent    = "event"
)

// Commands
const (
	CmdPrepare = "prepare" // GUI → CLI: Here are my files
	CmdUpload  = "upload"  // GUI → CLI: Go with these settings
	CmdCancel  = "cancel"  // GUI → CLI: User hit Escape
)

// Event types
const (
	EventProgress = "progress"
	EventComplete = "complete"
	EventError    = "error"
)

// Message wraps all communication
type Message struct {
	Type    string      `json:"type"`    // request, response, event
	Command string      `json:"command"` // prepare, upload, cancel
	Data    interface{} `json:"data"`
	ID      string      `json:"id,omitempty"`
}

// PrepareRequest - Initial request from GUI with selected files
type PrepareRequest struct {
	Files []string `json:"files"`
}

// PrepareResponse - Response with extracted metadata for review
type PrepareResponse struct {
	SessionID string            `json:"sessionId"`
	Files     []FileInfo        `json:"files"`
	Metadata  ExtractedMetadata `json:"metadata"`
	Backends  []string          `json:"backends"` // Available backends
}

// FileInfo contains basic file information
type FileInfo struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// ExtractedMetadata - Pre-filled metadata from EXIF/IPTC
type ExtractedMetadata struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Location    *Location `json:"location,omitempty"`
	Date        string    `json:"date,omitempty"`
	// Add camera info for display?
	Camera      string    `json:"camera,omitempty"`
	Lens        string    `json:"lens,omitempty"`
}

// Location from GPS EXIF
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
}

// UploadRequest - Final upload request after user review
type UploadRequest struct {
	SessionID    string   `json:"sessionId"`
	Metadata     Metadata `json:"metadata"`
	Backend      string   `json:"backend"`
	PostToSocial []string `json:"postToSocial,omitempty"`
	Format       string   `json:"format"`
}

// Metadata - User's edited metadata
type Metadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Alt         string   `json:"alt,omitempty"` // For accessibility
}

// ProgressEvent - Progress updates during upload
type ProgressEvent struct {
	SessionID string  `json:"sessionId"`
	FileIndex int     `json:"fileIndex"`
	FileName  string  `json:"fileName"`
	Progress  float64 `json:"progress"` // 0-100
	Status    string  `json:"status"`   // "extracting", "uploading", "processing", "complete"
	Message   string  `json:"message,omitempty"`
}

// UploadResult - Final result
type UploadResult struct {
	SessionID string   `json:"sessionId"`
	Success   bool     `json:"success"`
	Outputs   []string `json:"outputs"` // Formatted per requested format
	Files     []string `json:"files"`   // Original filenames for reference
	Error     string   `json:"error,omitempty"`
}

// CancelRequest - Cancel an in-progress upload
type CancelRequest struct {
	SessionID string `json:"sessionId"`
}

// ErrorResponse - Error response for any command
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`    // Error code for GUI handling
	Details string `json:"details,omitempty"` // Technical details
}
