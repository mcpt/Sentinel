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
	"time"
)

type FileSystemHandler struct {
	tempDir string
}

func NewFileSystemHandler() (*FileSystemHandler, error) {
	return &FileSystemHandler{
		tempDir: "/tmp/backups/fs",
	}, nil
}

func (h *FileSystemHandler) Backup(ctx context.Context) (string, error) {
	tempDir := filepath.Join(config.Cfg.TempDir, "fs_backup")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
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
		destPath := filepath.Join(tempDir, relPath)
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
	timestamp := time.Now().Format("20060102_150405")
	archivePath := filepath.Join(config.Cfg.TempDir, fmt.Sprintf("fs_backup_%s.tar.gz", timestamp))

	cmd := exec.Command("tar", "-czf", archivePath, "-C", tempDir, ".")
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