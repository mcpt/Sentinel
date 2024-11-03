package handlers

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mcpt/Sentinel/config"
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
	cmd := exec.CommandContext(ctx, "mysqldump",
		"--single-transaction",
		"-h", config.Cfg.MySQL.Host,
		"-P", config.Cfg.MySQL.Port,
		"-u", config.Cfg.MySQL.User,
		fmt.Sprintf("-p%s", config.Cfg.MySQL.Password),
		config.Cfg.MySQL.Database,
		"--quick",
		"-r ", filename)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to perform database backup: %v", err)
	}
	fmt.Printf(string(output))

	return filename, nil
}

func (h *MySQLHandler) Name() string {
	return "MySQL Backup"
}
