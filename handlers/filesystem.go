package handlers

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gobwas/glob"
	"github.com/mcpt/Sentinel/config"
)

// ErrFileSystem represents filesystem backup specific errors
type ErrFileSystem struct {
	Op  string // Operation that failed
	Err error  // Original error
}

func (e *ErrFileSystem) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("filesystem backup error during %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("filesystem backup error during %s", e.Op)
}

// FileSystemHandler handles filesystem backup operations
type FileSystemHandler struct {
	tempDir string
}

// NewFileSystemHandler creates a new filesystem backup handler
func NewFileSystemHandler() (*FileSystemHandler, error) {
	tmpDir, err := os.MkdirTemp("", "filesystem")
	if err != nil {
		return nil, &ErrFileSystem{Op: "create temp directory", Err: err}
	}

	return &FileSystemHandler{
		tempDir: tmpDir,
	}, nil
}

// matchesPatterns checks if a path matches any of the provided glob patterns
func matchesPatterns(path string, patterns []string) bool {
	for _, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			continue
		}
		if g.Match(path) {
			return true
		}
	}
	return false
}

// shouldIncludePath determines if a path should be included in the backup
func (h *FileSystemHandler) shouldIncludePath(relPath string) bool {
	// Check exclude patterns first
	if matchesPatterns(relPath, config.Cfg.Filesystem.ExcludePatterns) {
		return false
	}

	// If no include patterns are specified, include everything not excluded
	if len(config.Cfg.Filesystem.IncludePatterns) == 0 {
		return true
	}

	// Check include patterns
	return matchesPatterns(relPath, config.Cfg.Filesystem.IncludePatterns)
}

// copyFile safely copies a file from src to dst
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	_, err = io.Copy(destination, source)
	if closeErr := destination.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// createArchive creates a tar archive of the backup directory
func (h *FileSystemHandler) createArchive() (string, error) {
	archivePath := filepath.Join(config.Cfg.TempDir, "filesystem.tar")
	cmd := exec.Command("tar", "-cf", archivePath, "-C", h.tempDir, ".")

	if err := cmd.Run(); err != nil {
		return "", &ErrFileSystem{Op: "create archive", Err: err}
	}

	return archivePath, nil
}

// processPath handles the backup of a single file or directory
func (h *FileSystemHandler) processPath(basePath, path string, info os.FileInfo) error {
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	if !h.shouldIncludePath(relPath) {
		return nil
	}

	destPath := filepath.Join(h.tempDir, relPath)
	if info.IsDir() {
		return os.MkdirAll(destPath, info.Mode())
	}

	return copyFile(path, destPath)
}

// Backup performs the filesystem backup operation
func (h *FileSystemHandler) Backup(ctx context.Context) (string, error) {
	// Ensure temp directory exists
	if err := os.MkdirAll(h.tempDir, 0750); err != nil {
		return "", &ErrFileSystem{Op: "create temp directory", Err: err}
	}

	// Walk through filesystem
	err := filepath.Walk(config.Cfg.Filesystem.BasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return h.processPath(config.Cfg.Filesystem.BasePath, path, info)
		}
	})

	if err != nil {
		return "", &ErrFileSystem{Op: "backup files", Err: err}
	}

	// Create archive
	return h.createArchive()
}

// Name returns the handler name
func (h *FileSystemHandler) Name() string {
	return "Filesystem Backup"
}

// Cleanup removes temporary files
func (h *FileSystemHandler) Cleanup() error {
	if h.tempDir != "" {
		if err := os.RemoveAll(h.tempDir); err != nil {
			return &ErrFileSystem{Op: "cleanup", Err: err}
		}
	}
	return nil
}
