package duplicate

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
)

// CalculateFileMD5 computes the MD5 hash of a file
func CalculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// CalculateStreamMD5 computes the MD5 hash from a reader
func CalculateStreamMD5(r io.Reader) (string, error) {
	hash := md5.New()
	if _, err := io.Copy(hash, r); err != nil {
		return "", fmt.Errorf("read stream: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// FileInfo contains file metadata used for duplicate detection
type FileInfo struct {
	Path     string
	MD5      string
	Size     int64
	Filename string
}

// GetFileInfo retrieves file information including MD5 hash
func GetFileInfo(filePath string) (*FileInfo, error) {
	// Get file stats
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// Calculate MD5
	md5Hash, err := CalculateFileMD5(filePath)
	if err != nil {
		return nil, fmt.Errorf("calculate MD5: %w", err)
	}

	return &FileInfo{
		Path:     filePath,
		MD5:      md5Hash,
		Size:     stat.Size(),
		Filename: stat.Name(),
	}, nil
}
