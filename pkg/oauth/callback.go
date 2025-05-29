package oauth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// CallbackServer handles OAuth callbacks
type CallbackServer struct {
	Port     int
	Path     string
	listener net.Listener
	result   chan CallbackResult
}

// CallbackResult contains the OAuth callback parameters
type CallbackResult struct {
	Token    string
	Verifier string
	Error    error
}

// NewCallbackServer creates a new callback server
func NewCallbackServer(port int, path string) *CallbackServer {
	return &CallbackServer{
		Port:   port,
		Path:   path,
		result: make(chan CallbackResult, 1),
	}
}

// Start starts the callback server
func (s *CallbackServer) Start(ctx context.Context) error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.Port))
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	
	mux := http.NewServeMux()
	mux.HandleFunc(s.Path, s.handleCallback)
	
	server := &http.Server{Handler: mux}
	
	go func() {
		<-ctx.Done()
		server.Close()
	}()
	
	go func() {
		if err := server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.result <- CallbackResult{Error: err}
		}
	}()
	
	return nil
}

// Wait waits for the callback or timeout
func (s *CallbackServer) Wait(timeout time.Duration) (*CallbackResult, error) {
	select {
	case result := <-s.result:
		return &result, result.Error
	case <-time.After(timeout):
		return nil, fmt.Errorf("oauth callback timeout")
	}
}

// URL returns the callback URL
func (s *CallbackServer) URL() string {
	return fmt.Sprintf("http://localhost:%d%s", s.Port, s.Path)
}

// handleCallback processes the OAuth callback
func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		s.result <- CallbackResult{Error: err}
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	token := values.Get("oauth_token")
	verifier := values.Get("oauth_verifier")
	
	if token == "" || verifier == "" {
		s.result <- CallbackResult{Error: fmt.Errorf("missing oauth parameters")}
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}
	
	// Show success page
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>imgupv2 - Authorization Complete</title>
    <style>
        body { 
            font-family: -apple-system, system-ui, sans-serif;
            max-width: 600px;
            margin: 100px auto;
            text-align: center;
            padding: 20px;
        }
        .success { color: #22c55e; font-size: 48px; }
        h1 { margin: 20px 0; }
        p { color: #666; margin: 20px 0; }
        code { 
            background: #f3f4f6;
            padding: 2px 6px;
            border-radius: 4px;
            font-family: monospace;
        }
    </style>
</head>
<body>
    <div class="success">✓</div>
    <h1>Authorization Complete!</h1>
    <p>You can now close this window and return to <code>imgupv2</code>.</p>
</body>
</html>`)
	
	// Send result
	s.result <- CallbackResult{
		Token:    token,
		Verifier: verifier,
	}
	
	// Close the server after a short delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		if s.listener != nil {
			s.listener.Close()
		}
	}()
}
