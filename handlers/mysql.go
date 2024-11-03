package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mcpt/Sentinel/config"
	"github.com/schollz/progressbar/v3"
)

// MySQLHandler handles MySQL backup operations
type MySQLHandler struct {
	tempDir string
}

// ErrMySQLBackup represents MySQL backup specific errors
type ErrMySQLBackup struct {
	Op  string // Operation that failed
	Err error  // Original error
}

func (e *ErrMySQLBackup) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("mysql backup error during %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("mysql backup error during %s", e.Op)
}

// NewMySQLHandler creates a new MySQL backup handler
func NewMySQLHandler() (*MySQLHandler, error) {
	tempDir := filepath.Join(config.Cfg.TempDir, "mysql")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, &ErrMySQLBackup{Op: "init", Err: err}
	}
	return &MySQLHandler{tempDir: tempDir}, nil
}

// generateBackupFilename creates a timestamped filename for the backup
func (h *MySQLHandler) generateBackupFilename() string {
	timestamp := time.Now().Format("20060102_150405")
	return filepath.Join(h.tempDir, fmt.Sprintf("mysql_%s.sql", timestamp))
}

// getTotalSize estimates the total size of the database
func (h *MySQLHandler) getTotalSize(ctx context.Context) (int64, error) {
	cmd := exec.CommandContext(ctx, "mariadb",
		"--skip-column-names",
		"--ssl=false",
		"-h", config.Cfg.MySQL.Host,
		"-P", config.Cfg.MySQL.Port,
		"-u", config.Cfg.MySQL.User,
		fmt.Sprintf("-p%s", config.Cfg.MySQL.Password),
		"-e 'SELECT ROUND(SUM(data_length) * 0.8) AS \"size_bytes\" FROM information_schema.TABLES;'",
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, &ErrMySQLBackup{Op: "size estimation", Err: err}
	}

	sizeStr := string(bytes.TrimSpace(output))
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		// Fallback to a default size if parsing fails
		return 563091866, nil
	}
	return size, nil
}

// createMySQLDumpCommand creates the mysqldump command with proper parameters
func (h *MySQLHandler) createMySQLDumpCommand(ctx context.Context, filename string) *exec.Cmd {
	return exec.CommandContext(ctx, "mysqldump",
		"--single-transaction",
		"--extended-insert",
		"--create-options",
		"--quick",
		"--result-file="+filename,
		"-h", config.Cfg.MySQL.Host,
		"-P", config.Cfg.MySQL.Port,
		"-u", config.Cfg.MySQL.User,
		fmt.Sprintf("-p%s", config.Cfg.MySQL.Password),
		config.Cfg.MySQL.Database,
	)
}

// Backup performs the MySQL backup operation
func (h *MySQLHandler) Backup(ctx context.Context) (string, error) {
	// Generate backup filename
	filename := h.generateBackupFilename()

	// Create backup file
	file, err := os.Create(filename)
	if err != nil {
		return "", &ErrMySQLBackup{Op: "create backup file", Err: err}
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = &ErrMySQLBackup{Op: "close backup file", Err: cerr}
		}
	}()

	// Get total size for progress bar
	totalSize, err := h.getTotalSize(ctx)
	if err != nil {
		// Log the error but continue with default size
		fmt.Printf("Warning: Failed to get total size: %v\n", err)
	}

	// Create progress bar
	bar := progressbar.DefaultBytes(
		totalSize,
		"mysqldump progress",
	)

	// Create and configure mysqldump command
	cmd := h.createMySQLDumpCommand(ctx, filename)

	// Set up pipes for output handling
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", &ErrMySQLBackup{Op: "create stdout pipe", Err: err}
	}
	cmd.Stderr = os.Stderr

	// Start the backup process
	if err := cmd.Start(); err != nil {
		return "", &ErrMySQLBackup{Op: "start mysqldump", Err: err}
	}

	// Write output to both file and progress bar
	multiWriter := io.MultiWriter(file, bar)
	if _, err := io.Copy(multiWriter, stdout); err != nil {
		return "", &ErrMySQLBackup{Op: "copy backup data", Err: err}
	}

	// Wait for the process to complete
	if err := cmd.Wait(); err != nil {
		return "", &ErrMySQLBackup{Op: "complete mysqldump", Err: err}
	}

	return filename, nil
}

// Name returns the handler name
func (h *MySQLHandler) Name() string {
	return "MySQL Backup"
}
