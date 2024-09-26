package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsCfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/robfig/cron/v3"
)

type Config struct {
	Database struct {
		Host     string
		Port     string
		User     string
		Password string
		Name     string
	}
	S3Compatible struct {
		Endpoint        string
		Region          string
		AccessKeyID     string
		SecretAccessKey string
		Bucket          string
	}
	BackupPatterns []string
}

func main() {
	configFile := flag.String("config", "backup_include.txt", "Path to the backup include configuration file")
	flag.Parse()

	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	c := cron.New()
	_, err = c.AddFunc("0 4 */14 * *", func() {
		if err := performBackup(config); err != nil {
			log.Printf("Backup failed: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("Failed to schedule backup: %v", err)
	}

	log.Println("Starting Sentinel backup system...")
	c.Start()

	// Keep the program running
	select {}
}

func loadConfig(configFile string) (Config, error) {
	var config Config

	// Load database config from env vars
	config.Database.Host = os.Getenv("DB_HOST")
	config.Database.Port = os.Getenv("DB_PORT")
	config.Database.User = os.Getenv("DB_USER")
	config.Database.Password = os.Getenv("DB_PASSWORD")
	config.Database.Name = os.Getenv("DB_NAME")

	// Load S3 compatible storage config
	config.S3Compatible.Endpoint = os.Getenv("S3_ENDPOINT")
	config.S3Compatible.Region = os.Getenv("S3_REGION")
	config.S3Compatible.AccessKeyID = os.Getenv("S3_ACCESS_KEY_ID")
	config.S3Compatible.SecretAccessKey = os.Getenv("S3_SECRET_ACCESS_KEY")
	config.S3Compatible.Bucket = os.Getenv("S3_BUCKET")

	// Check if all required configurations are set
	if config.Database.Host == "" || config.Database.Port == "" || config.Database.User == "" ||
		config.Database.Password == "" || config.Database.Name == "" || config.S3Compatible.Endpoint == "" ||
		config.S3Compatible.Region == "" || config.S3Compatible.AccessKeyID == "" ||
		config.S3Compatible.SecretAccessKey == "" || config.S3Compatible.Bucket == "" {
		return config, fmt.Errorf("missing required environment variables")
	}

	// Load backup patterns from file
	patterns, err := loadBackupPatterns(configFile)
	if err != nil {
		return config, fmt.Errorf("failed to load backup patterns: %v", err)
	}
	config.BackupPatterns = patterns

	return config, nil
}

func loadBackupPatterns(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		pattern := strings.TrimSpace(scanner.Text())
		if pattern != "" && !strings.HasPrefix(pattern, "#") {
			patterns = append(patterns, pattern)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}

func performBackup(config Config) error {
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	// Backup database
	dbBackupFile := fmt.Sprintf("db_backup_%s.sql", timestamp)
	cmd := exec.Command("mysqldump",
		"-h", config.Database.Host,
		"-P", config.Database.Port,
		"-u", config.Database.User,
		fmt.Sprintf("-p%s", config.Database.Password),
		config.Database.Name)

	outfile, err := os.Create(dbBackupFile)
	if err != nil {
		return fmt.Errorf("failed to create database backup file: %v", err)
	}
	defer outfile.Close()

	cmd.Stdout = outfile
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to perform database backup: %v", err)
	}

	// Upload database backup
	if err := uploadToS3Compatible(config, dbBackupFile); err != nil {
		return fmt.Errorf("failed to upload database backup: %v", err)
	}

	// Cleanup local database backup file
	if err := os.Remove(dbBackupFile); err != nil {
		log.Printf("Failed to remove local database backup file: %v", err)
	}

	// Backup and upload files matching the patterns
	for _, pattern := range config.BackupPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			log.Printf("Error matching pattern %s: %v", pattern, err)
			continue
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				log.Printf("Error stating file %s: %v", match, err)
				continue
			}

			if info.IsDir() {
				err := filepath.Walk(match, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() {
						return uploadToS3Compatible(config, path)
					}
					return nil
				})
				if err != nil {
					log.Printf("Error walking directory %s: %v", match, err)
				}
			} else {
				if err := uploadToS3Compatible(config, match); err != nil {
					log.Printf("Error uploading file %s: %v", match, err)
				}
			}
		}
	}

	return nil
}

func uploadToS3Compatible(config Config, filepath string) error {
	ctx := context.TODO()

	// Configure AWS SDK for S3 compatible storage
	customResolver := aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: config.S3Compatible.Endpoint,
		}, nil
	})

	cfg, err := awsCfg.LoadDefaultConfig(ctx,
		awsCfg.WithEndpointResolver(customResolver),
		awsCfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.S3Compatible.AccessKeyID,
			config.S3Compatible.SecretAccessKey,
			"",
		)),
		awsCfg.WithRegion(config.S3Compatible.Region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", filepath, err)
	}
	defer file.Close()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(config.S3Compatible.Bucket),
		Key:    aws.String(filepath),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file %s: %v", filepath, err)
	}

	log.Printf("Successfully uploaded %s to S3 compatible storage", filepath)
	return nil
}
