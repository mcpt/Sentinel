package handlers

import (
	"bytes"
	"context"
	"fmt"
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
	if err := os.MkdirAll(tempDir, 0750); err != nil {
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
	cmd := exec.CommandContext(ctx, "mariadb", // #nosec  G204 -- All data here is coming from the config file,
		// which if someone can modify, they can do anything they want
		"--skip-column-names",
		"-sss", // Removes boxing around the output
		"--ssl=false",
		"-h", config.Cfg.MySQL.Host,
		"-P", config.Cfg.MySQL.Port,
		"-u", config.Cfg.MySQL.User,
		fmt.Sprintf("-p%s", config.Cfg.MySQL.Password),
		"-e 'SELECT ROUND(SUM(data_length) * 0.8) AS \"size_bytes\" FROM information_schema.TABLES;'",
	)
	output, err := cmd.Output()
	fmt.Println(cmd.String())

	if err != nil {
		return 0, &ErrMySQLBackup{Op: "size estimation", Err: err}
	}

	sizeStr := string(bytes.TrimSpace(output))
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		fmt.Println("Failed to convert size string to int64", err)
		// Fallback to a default size if parsing fails
		return 563091866, nil
	}
	return size, nil
}

// createMySQLDumpCommand creates the mysqldump command with proper parameters
func (h *MySQLHandler) createMySQLDumpCommand(ctx context.Context, filename string) *exec.Cmd {
	return exec.CommandContext(ctx, "mysqldump", // #nosec  G204 -- All data here is coming from the config file,
		// which if someone can modify, they can do anything they want
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

	// Start the backup process
	if err := cmd.Start(); err != nil {
		return "", &ErrMySQLBackup{Op: "start mysqldump", Err: err}
	}

	// Update progress bar based on size of the output file
	for checkRunning(cmd) {
		fileInfo, err := os.Stat(filename)
		if err != nil {
			return "", err
		}
		err = bar.Set64(fileInfo.Size())
		if err != nil {
			return "", err
		}
		time.Sleep(time.Second)
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

func checkRunning(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.ProcessState != nil && cmd.ProcessState.Exited() || cmd.Process == nil {
		return false
	}

	return true
}
