package handlers

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/mcpt/Sentinel/config"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type FileSystemHandler struct {
	tempDir string
}

func NewFileSystemHandler() (*FileSystemHandler, error) {
	tmpDir, _ := os.MkdirTemp("", "filesystem")
	return &FileSystemHandler{
		tempDir: tmpDir,
	}, nil
}

func (h *FileSystemHandler) Backup(ctx context.Context) (string, error) {
	if err := os.MkdirAll(h.tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}

	err := filepath.Walk(config.Cfg.Filesystem.BasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(config.Cfg.Filesystem.BasePath, path)
		if err != nil {
			return err
		}

		// Skip if path matches exclude patterns
		for _, pattern := range config.Cfg.Filesystem.ExcludePatterns {
			g, err := glob.Compile(pattern)
			if err != nil {
				continue
			}
			if g.Match(relPath) {
				return nil
			}
		}

		// Check if path matches include patterns
		included := false
		for _, pattern := range config.Cfg.Filesystem.IncludePatterns {
			g, err := glob.Compile(pattern)
			if err != nil {
				continue
			}
			if g.Match(relPath) {
				included = true
				break
			}
		}

		if !included {
			return nil
		}

		// Create destination directory
		destPath := filepath.Join(h.tempDir, relPath)
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Copy file
		return copyFile(path, destPath)
	})

	if err != nil {
		return "", fmt.Errorf("failed to backup filesystem: %v", err)
	}

	// Create tar.gz archive
	archivePath := filepath.Join(config.Cfg.TempDir, fmt.Sprintf("filesystem.tar"))

	cmd := exec.Command("tar", "-cf", archivePath, "-C", h.tempDir, ".")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create archive: %v", err)
	}
	return archivePath, nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func (h *FileSystemHandler) Name() string {
	return "Filesystem Backup"
}
