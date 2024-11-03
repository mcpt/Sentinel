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

type MySQLHandler struct {
	tempDir string
}

func NewMySQLHandler() (*MySQLHandler, error) {
	return &MySQLHandler{
		tempDir: "/tmp/backups/mysql",
	}, nil
}

func (h *MySQLHandler) Backup(ctx context.Context) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(h.tempDir, fmt.Sprintf("mysql_%s.sql", timestamp))

	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// Estimate total size for the progress bar (example: 10MB)

	// To get total size in bytes for the progress bar, you can use the following command:
	//mysql --skip-column-names <parameters> <<< 'SELECT ROUND(SUM(data_length) * 0.8) AS "size_bytes" FROM information_schema.TABLES;')

	cmd := exec.CommandContext(ctx, "mariadb",
		"--skip-column-names",
		"--ssl=false",
		"-h", config.Cfg.MySQL.Host,
		"-P", config.Cfg.MySQL.Port,
		"-u", config.Cfg.MySQL.User,
		fmt.Sprintf("-p%s", config.Cfg.MySQL.Password),
		"-e 'SELECT ROUND(SUM(data_length) * 0.8) AS \"size_bytes\" FROM information_schema.TABLES;'",
	)

	totalSizeRaw, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get total size: %v", err)
	}
	totalSizeRaw = bytes.TrimSpace(totalSizeRaw)
	totalSize, err := strconv.ParseInt(string(totalSizeRaw), 10, 64)
	if err != nil {
		fmt.Println("Error parsing total size:", err)
		totalSize = 563091866
	}

	bar := progressbar.DefaultBytes(
		totalSize,
		"mysqldump progress",
	)

	cmd = exec.CommandContext(ctx, "mysqldump",
		"--single-transaction",
		"--extended-insert",
		"--create-options",
		"--quick",
		"-h", config.Cfg.MySQL.Host,
		"-P", config.Cfg.MySQL.Port,
		"-u", config.Cfg.MySQL.User,
		fmt.Sprintf("-p%s", config.Cfg.MySQL.Password),
		config.Cfg.MySQL.Database,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout pipe: %v", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start mysqldump: %v", err)
	}

	multiWriter := io.MultiWriter(file, bar)
	_, err = io.Copy(multiWriter, stdout)
	if err != nil {
		return "", fmt.Errorf("failed to copy data: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("mysqldump process error: %v", err)
	}

	return filename, nil
}

func (h *MySQLHandler) Name() string {
	return "MySQL Backup"
}
