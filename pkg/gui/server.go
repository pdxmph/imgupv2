package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/mph/imgupv2/pkg/config"
	"github.com/mph/imgupv2/pkg/metadata"
	"github.com/mph/imgupv2/pkg/upload"
)

// Server handles GUI protocol communication
type Server struct {
	input    io.Reader
	output   io.Writer
	encoder  *json.Encoder
	config   *config.Config
	uploader *upload.Service

	// Session management
	sessions sync.Map // sessionID -> *Session
}

// Session represents an active upload session
type Session struct {
	ID       string
	Files    []string
	Metadata ExtractedMetadata
	Cancel   context.CancelFunc
}

// NewServer creates a new GUI protocol server
func NewServer(input io.Reader, output io.Writer, cfg *config.Config, uploader *upload.Service) *Server {
	return &Server{
		input:    input,
		output:   output,
		encoder:  json.NewEncoder(output),
		config:   cfg,
		uploader: uploader,
	}
}

// Run starts the server loop
func (s *Server) Run(ctx context.Context) error {
	decoder := json.NewDecoder(s.input)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var msg Message
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF {
					return nil
				}
				s.sendError("", fmt.Sprintf("Invalid JSON: %v", err), "PARSE_ERROR")
				continue
			}

			s.handleMessage(ctx, &msg)
		}
	}
}

// handleMessage routes messages to appropriate handlers
func (s *Server) handleMessage(ctx context.Context, msg *Message) {
	switch msg.Command {
	case CmdPrepare:
		s.handlePrepare(ctx, msg)
	case CmdUpload:
		s.handleUpload(ctx, msg)
	case CmdCancel:
		s.handleCancel(ctx, msg)
	default:
		s.sendError(msg.ID, fmt.Sprintf("Unknown command: %s", msg.Command), "UNKNOWN_COMMAND")
	}
}

// handlePrepare processes file selection and extracts metadata
func (s *Server) handlePrepare(ctx context.Context, msg *Message) {
	var req PrepareRequest
	if err := decodeData(msg.Data, &req); err != nil {
		s.sendError(msg.ID, "Invalid prepare request", "INVALID_REQUEST")
		return
	}

	// Create session
	sessionID := uuid.New().String()
	session := &Session{
		ID:    sessionID,
		Files: req.Files,
	}

	// Extract metadata from first file (or merge from all?)
	if len(req.Files) > 0 {
		// Extract metadata from the first file
		title, description, tags, err := metadata.ExtractMetadata(req.Files[0])
		if err != nil {
			// Log error but continue with empty metadata
			fmt.Fprintf(os.Stderr, "Warning: Could not extract metadata: %v\n", err)
			session.Metadata = ExtractedMetadata{}
		} else {
			fmt.Fprintf(os.Stderr, "DEBUG: Extracted metadata - Title: %q, Desc: %q, Tags: %v\n", title, description, tags)
			session.Metadata = ExtractedMetadata{
				Title:       title,
				Description: description,
				Tags:        tags,
			}
			fmt.Fprintf(os.Stderr, "DEBUG: Session metadata set to: %+v\n", session.Metadata)
		}
	}

	// Store session
	s.sessions.Store(sessionID, session)

	// Build file info
	files := make([]FileInfo, 0, len(req.Files))
	for _, path := range req.Files {
		if info, err := os.Stat(path); err == nil {
			files = append(files, FileInfo{
				Path:     path,
				Name:     filepath.Base(path),
				Size:     info.Size(),
				Modified: info.ModTime(),
			})
		}
	}

	// Send response
	resp := PrepareResponse{
		SessionID: sessionID,
		Files:     files,
		Metadata:  session.Metadata,
		Backends:  s.getAvailableBackends(),
	}
	
	fmt.Fprintf(os.Stderr, "DEBUG: Sending PrepareResponse with metadata: %+v\n", resp.Metadata)

	s.sendResponse(msg.ID, resp)
}

// handleUpload processes the upload request
func (s *Server) handleUpload(ctx context.Context, msg *Message) {
	var req UploadRequest
	if err := decodeData(msg.Data, &req); err != nil {
		s.sendError(msg.ID, "Invalid upload request", "INVALID_REQUEST")
		return
	}

	// Get session
	sessionData, ok := s.sessions.Load(req.SessionID)
	if !ok {
		s.sendError(msg.ID, "Session not found", "SESSION_NOT_FOUND")
		return
	}
	session := sessionData.(*Session)

	// Create cancellable context
	uploadCtx, cancel := context.WithCancel(ctx)
	session.Cancel = cancel

	// Start upload in goroutine
	go s.performUpload(uploadCtx, session, &req, msg.ID)
}

// performUpload executes the actual upload with progress tracking
func (s *Server) performUpload(ctx context.Context, session *Session, req *UploadRequest, messageID string) {
	outputs := make([]string, 0, len(session.Files))
	
	for i, file := range session.Files {
		select {
		case <-ctx.Done():
			s.sendEvent(EventError, ProgressEvent{
				SessionID: session.ID,
				Status:    "cancelled",
				Message:   "Upload cancelled",
			})
			return
		default:
			// Send progress: extracting metadata
			s.sendEvent(EventProgress, ProgressEvent{
				SessionID: session.ID,
				FileIndex: i,
				FileName:  filepath.Base(file),
				Progress:  10,
				Status:    "extracting",
				Message:   "Reading metadata",
			})

			// Embed metadata if needed
			if hasMetadata := req.Metadata.Title != "" || req.Metadata.Description != "" || len(req.Metadata.Tags) > 0; hasMetadata && metadata.HasExiftool() {
				fmt.Fprintf(os.Stderr, "DEBUG: Embedding metadata - Title: %q, Desc: %q, Tags: %v\n", req.Metadata.Title, req.Metadata.Description, req.Metadata.Tags)
				writer, err := metadata.NewWriter()
				if err == nil {
					tempPath, err := writer.CopyWithMetadata(file, req.Metadata.Title, req.Metadata.Description, req.Metadata.Tags)
					if err == nil {
						// Use temp file for upload
						fmt.Fprintf(os.Stderr, "DEBUG: Created temp file with metadata: %s\n", tempPath)
						file = tempPath
						defer os.Remove(tempPath)
					} else {
						fmt.Fprintf(os.Stderr, "ERROR: Failed to embed metadata: %v\n", err)
					}
				} else {
					fmt.Fprintf(os.Stderr, "ERROR: Failed to create metadata writer: %v\n", err)
				}
			} else {
				fmt.Fprintf(os.Stderr, "DEBUG: No metadata to embed or exiftool not available\n")
			}

			// Send progress: uploading
			s.sendEvent(EventProgress, ProgressEvent{
				SessionID: session.ID,
				FileIndex: i,
				FileName:  filepath.Base(file),
				Progress:  30,
				Status:    "uploading",
				Message:   fmt.Sprintf("Uploading to %s", req.Backend),
			})

			// Perform upload
			result, err := s.uploader.Upload(ctx, file, upload.Options{
				Backend:     req.Backend,
				Format:      req.Format,
				Title:       req.Metadata.Title,
				Description: req.Metadata.Description,
				Tags:        req.Metadata.Tags,
				Alt:         req.Metadata.Alt,
			})

			if err != nil {
				s.sendEvent(EventError, ProgressEvent{
					SessionID: session.ID,
					FileIndex: i,
					FileName:  filepath.Base(file),
					Status:    "error",
					Message:   err.Error(),
				})
				continue
			}

			outputs = append(outputs, result.FormattedOutput)

			// Send progress: complete
			s.sendEvent(EventProgress, ProgressEvent{
				SessionID: session.ID,
				FileIndex: i,
				FileName:  filepath.Base(file),
				Progress:  100,
				Status:    "complete",
			})
		}
	}

	// Send final result
	s.sendResponse(messageID, UploadResult{
		SessionID: session.ID,
		Success:   true,
		Outputs:   outputs,
		Files:     session.Files,
	})

	// Clean up session
	s.sessions.Delete(session.ID)
}

// handleCancel cancels an in-progress upload
func (s *Server) handleCancel(ctx context.Context, msg *Message) {
	var req CancelRequest
	if err := decodeData(msg.Data, &req); err != nil {
		s.sendError(msg.ID, "Invalid cancel request", "INVALID_REQUEST")
		return
	}

	sessionData, ok := s.sessions.Load(req.SessionID)
	if !ok {
		s.sendError(msg.ID, "Session not found", "SESSION_NOT_FOUND")
		return
	}

	session := sessionData.(*Session)
	if session.Cancel != nil {
		session.Cancel()
	}

	s.sessions.Delete(req.SessionID)
}

// Helper methods

func (s *Server) sendResponse(id string, data interface{}) {
	msg := Message{
		Type:    TypeResponse,
		Command: "", // Response doesn't need command
		Data:    data,
		ID:      id,
	}
	s.encoder.Encode(&msg)
}

func (s *Server) sendEvent(eventType string, data interface{}) {
	msg := Message{
		Type:    TypeEvent,
		Command: eventType,
		Data:    data,
	}
	s.encoder.Encode(&msg)
}

func (s *Server) sendError(id string, message string, code string) {
	s.sendResponse(id, ErrorResponse{
		Error: message,
		Code:  code,
	})
}

func (s *Server) getAvailableBackends() []string {
	// TODO: Check which backends are configured
	backends := []string{}
	if s.config.Flickr.ConsumerKey != "" && s.config.Flickr.AccessToken != "" {
		backends = append(backends, "flickr")
	}
	// Add smugmug when implemented
	return backends
}

func decodeData(data interface{}, target interface{}) error {
	// Re-encode and decode to handle interface{} -> struct conversion
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, target)
}
