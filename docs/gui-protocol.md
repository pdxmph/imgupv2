# imgupv2 GUI Protocol

## Overview

The imgupv2 GUI protocol enables platform-specific GUI applications to communicate with the imgup CLI core via JSON messages over stdin/stdout. This design keeps the core CLI independent of GUI frameworks while enabling rich native experiences.

## Design Goals

1. **Fast workflow** - Minimize steps from file selection to upload
2. **Keyboard-driven** - Support quick review/edit without mouse
3. **Async progress** - Real-time upload progress feedback
4. **Simple integration** - Easy for GUI developers to implement

## Communication Model

The GUI spawns `imgup gui-server` as a subprocess and communicates via:
- **stdin**: GUI sends JSON requests to imgup
- **stdout**: imgup sends JSON responses and events
- **stderr**: Error/debug output (not part of protocol)

## Message Format

All messages follow this structure:

```json
{
  "type": "request|response|event",
  "command": "prepare|upload|cancel",  // For requests/events
  "data": { ... },                     // Message payload
  "id": "unique-id"                    // For request/response correlation
}
```

## Workflow

### 1. Prepare Upload

GUI sends selected files for metadata extraction:

**Request:**
```json
{
  "type": "request",
  "command": "prepare",
  "data": {
    "files": ["/path/to/image1.jpg", "/path/to/image2.jpg"]
  },
  "id": "req-123"
}
```

**Response:**
```json
{
  "type": "response",
  "data": {
    "sessionId": "550e8400-e29b-41d4-a716-446655440000",
    "files": [
      {
        "path": "/path/to/image1.jpg",
        "name": "image1.jpg",
        "size": 2048576,
        "modified": "2024-01-15T10:30:00Z"
      }
    ],
    "metadata": {
      "title": "Sunset at the Beach",
      "description": "EXIF description if present",
      "tags": ["landscape", "sunset"],
      "date": "2024-01-14T18:45:00Z",
      "camera": "Sony A7III",
      "lens": "24-70mm f/2.8",
      "location": {
        "latitude": 37.7749,
        "longitude": -122.4194
      }
    },
    "backends": ["flickr", "smugmug"]
  },
  "id": "req-123"
}
```

### 2. Upload

After user reviews/edits metadata:

**Request:**
```json
{
  "type": "request",
  "command": "upload",
  "data": {
    "sessionId": "550e8400-e29b-41d4-a716-446655440000",
    "metadata": {
      "title": "Beautiful Sunset",
      "description": "User edited description",
      "tags": ["sunset", "beach", "california"],
      "alt": "Orange sunset over ocean waves"
    },
    "backend": "flickr",
    "format": "markdown",
    "postToSocial": ["twitter", "mastodon"]
  },
  "id": "req-124"
}
```

### 3. Progress Events

Server sends progress updates during upload:

```json
{
  "type": "event",
  "command": "progress",
  "data": {
    "sessionId": "550e8400-e29b-41d4-a716-446655440000",
    "fileIndex": 0,
    "fileName": "image1.jpg",
    "progress": 45.5,
    "status": "uploading",
    "message": "Uploading to Flickr"
  }
}
```

Status values:
- `extracting` - Reading file metadata
- `uploading` - Uploading to backend
- `processing` - Backend processing
- `complete` - Upload finished
- `error` - Upload failed

### 4. Final Result

**Response:**
```json
{
  "type": "response",
  "data": {
    "sessionId": "550e8400-e29b-41d4-a716-446655440000",
    "success": true,
    "outputs": [
      "![Beautiful Sunset](https://live.staticflickr.com/65535/12345_secret_c.jpg)",
      "![Beach Morning](https://live.staticflickr.com/65535/67890_secret_c.jpg)"
    ],
    "files": ["/path/to/image1.jpg", "/path/to/image2.jpg"]
  },
  "id": "req-124"
}
```

## Error Handling

Errors can occur at message or command level:

```json
{
  "type": "response",
  "data": {
    "error": "No authenticated backends available",
    "code": "NO_BACKENDS",
    "details": "Run 'imgup auth flickr' first"
  },
  "id": "req-123"
}
```

Error codes:
- `PARSE_ERROR` - Invalid JSON
- `UNKNOWN_COMMAND` - Unknown command
- `INVALID_REQUEST` - Malformed request data
- `SESSION_NOT_FOUND` - Invalid session ID
- `NO_BACKENDS` - No authenticated backends
- `UPLOAD_FAILED` - Backend upload error

## GUI Implementation Tips

1. **Session Management**: Store sessionId from prepare response
2. **Progress UI**: Update progress bar based on events
3. **Keyboard Navigation**: Tab through metadata fields
4. **Quick Actions**: Cmd+Return to upload, Esc to cancel
5. **Auto-close**: Close window on successful upload
6. **Error Display**: Show errors inline, not modal dialogs

## Testing

Use the included test client:

```bash
python3 tools/test_gui_client.py /path/to/test.jpg
```

Or test manually:

```bash
echo '{"type":"request","command":"prepare","data":{"files":["/path/to/test.jpg"]},"id":"1"}' | imgup gui-server
```
