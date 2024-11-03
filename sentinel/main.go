package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/mcpt/Sentinel/config"
	"github.com/mcpt/Sentinel/handlers"
	"github.com/mcpt/Sentinel/storage"
	"github.com/robfig/cron/v3"
)

func main() {
	configFile := flag.String("config-file", "config.toml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize backup handlers
	var backupHandlers []handlers.BackupHandler

	// MySQL handler
	if config.Cfg.MySQL.Enabled {
		mysqlHandler, err := handlers.NewMySQLHandler()
		if err != nil {
			log.Fatalf("Failed to initialize MySQL handler: %v", err)
		}
		backupHandlers = append(backupHandlers, mysqlHandler)
	}

	// Filesystem handler
	if config.Cfg.Filesystem.Enabled {
		fsHandler, err := handlers.NewFileSystemHandler()
		if err != nil {
			log.Fatalf("Failed to initialize filesystem handler: %v", err)
		}
		backupHandlers = append(backupHandlers, fsHandler)
	}

	// Initialize S3 uploader
	s3Uploader, err := storage.NewS3Uploader()
	if err != nil {
		log.Fatalf("Failed to initialize S3 uploader: %v", err)
	}

	// Create cron scheduler
	// If the schedule is empty, don't schedule the backup, just run it immediately
	if config.Cfg.Schedule == "" {
		if err := performBackup(backupHandlers, s3Uploader); err != nil {
			log.Fatalf("Backup failed: %v", err)
		}
		return
	} else {
		c := cron.New()
		_, err = c.AddFunc(config.Cfg.Schedule, func() {
			if err := performBackup(backupHandlers, s3Uploader); err != nil {
				log.Printf("Backup failed: %v", err)
			}
		})
		if err != nil {
			log.Fatalf("Failed to schedule backup: %v", err)
		}

		// Start cron scheduler
		c.Start()
		log.Printf("Backup system started with schedule: %s", config.Cfg.Schedule)

		// Handle shutdown gracefully
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		c.Stop()
	}
}

func performBackup(handlerList []handlers.BackupHandler, uploader *storage.S3Uploader) error {
	ctx := context.Background()
	var backupFiles []string
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make(chan error, len(handlerList))

	backupPath := filepath.Join(config.Cfg.TempDir, "backup")

	// Create temporary backup directory
	if err := os.MkdirAll(config.Cfg.TempDir, 0755); err != nil {
		return err
	}

	// Perform backups concurrently
	for _, h := range handlerList {
		wg.Add(1)
		go func(handler handlers.BackupHandler) {
			defer wg.Done()
			fmt.Printf("Performing backup: %s\n", handler.Name())
			backupFile, err := handler.Backup(ctx)
			if err != nil {
				errors <- err
				return
			}

			mu.Lock()
			backupFiles = append(backupFiles, backupFile)
			mu.Unlock()
		}(h)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			return err
		}
	}

	//// Create final archive
	//compressor, err := compression.NewCompressor(config.Cfg.Compression.Format, config.Cfg.Compression.Level)
	//if err != nil {
	//	return err
	//}
	//fmt.Printf("Compressing backups: %s\n", backupFiles)
	//err = compressor.Compress(backupFiles, backupPath)
	//if err != nil {
	//	return err
	//}

	// Upload final archive
	if config.Cfg.Debug {
		fmt.Printf("Uploading backup to S3: %s\n", backupPath)

	}
	for _, file := range backupFiles {
		fmt.Printf("Uploading backup file: %s\n", file)
		if err := uploader.UploadFile(ctx, file); err != nil {
			return err
		}
	}
	//if err := uploader.UploadFile(ctx, backupPath); err != nil {
	//	return err
	//}

	// Cleanup
	for _, file := range backupFiles {
		err := os.Remove(file)
		if err != nil {
			return err
		}
	}
	err := os.RemoveAll(config.Cfg.TempDir)
	if err != nil {
		log.Printf("Failed to remove temporary backup directory: %v", err)
	}

	return nil
}
