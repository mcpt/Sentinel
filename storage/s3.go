package storage

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	cfg "github.com/mcpt/Sentinel/config"
	"github.com/schollz/progressbar/v3"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type S3Uploader struct {
	client   *s3.Client
	uploader *manager.Uploader
}

func NewS3Uploader() (*S3Uploader, error) {
	ctx := context.Background()

	// Set default values if not specified
	if cfg.Cfg.S3.MaxConcurrency == 0 {
		cfg.Cfg.S3.MaxConcurrency = 10
	}
	if cfg.Cfg.S3.PartSize == 0 {
		cfg.Cfg.S3.PartSize = 5 * 1024 * 1024 // 5MB default part size
	}

	customResolver := aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
		if cfg.Cfg.S3.Endpoint != "" {
			return aws.Endpoint{
				URL: cfg.Cfg.S3.Endpoint,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithEndpointResolver(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.Cfg.S3.AccessKeyID,
			cfg.Cfg.S3.SecretAccessKey,
			"",
		)),
		config.WithRegion(cfg.Cfg.S3.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	client := s3.NewFromConfig(awsCfg)
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.Concurrency = cfg.Cfg.S3.MaxConcurrency
		u.PartSize = cfg.Cfg.S3.PartSize
	})

	return &S3Uploader{
		client:   client,
		uploader: uploader,
	}, nil
}

// UploadDirectory uploads an entire directory to S3
func (u *S3Uploader) UploadDirectory(ctx context.Context, localPath string) error {
	// Create a buffered channel to control concurrency
	uploadChan := make(chan string, cfg.Cfg.S3.MaxConcurrency)
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < cfg.Cfg.S3.MaxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range uploadChan {
				if err := u.uploadFile(ctx, path, localPath); err != nil {
					select {
					case errChan <- err:
					default:
					}
					return
				}
			}
		}()
	}

	// Walk the directory and send files to upload
	go func() {
		err := filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				select {
				case uploadChan <- path:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
		close(uploadChan)
		if err != nil {
			select {
			case errChan <- err:
			default:
			}
		}
	}()

	// Wait for all uploads to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	if err := <-errChan; err != nil {
		return fmt.Errorf("upload failed: %v", err)
	}

	return nil
}

func (u *S3Uploader) uploadFile(ctx context.Context, filePath, basePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", filePath, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	bar := progressbar.DefaultBytes(
		fileInfo.Size(),
		fmt.Sprintf("uploading %s", filePath),
	)

	// Calculate relative path for S3 key
	relPath, err := filepath.Rel(basePath, filePath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %v", err)
	}

	// use a human-readable timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	s3Key := filepath.Join(timestamp, relPath)

	// Create a pipe for streaming
	pr, pw := io.Pipe()

	// Start upload
	var uploadErr error
	var uploadWg sync.WaitGroup
	uploadWg.Add(1)

	go func() {
		defer uploadWg.Done()
		_, uploadErr = u.uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: aws.String(cfg.Cfg.S3.Bucket),
			Key:    aws.String(s3Key),
			Body:   pr,
		})
		err := pr.Close()
		if err != nil {
			log.Printf("Failed to close pipe: %v", err)
		}
	}()

	// Copy file to pipe with progress bar
	_, err = io.Copy(io.MultiWriter(pw, bar), file)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}
	err = pw.Close()
	if err != nil {
		return fmt.Errorf("failed to close pipe: %v", err)
	}

	// Wait for upload to complete
	uploadWg.Wait()
	if uploadErr != nil {
		return fmt.Errorf("failed to upload to S3: %v", uploadErr)
	}

	log.Printf("Successfully uploaded %s to s3://%s/%s", filePath, cfg.Cfg.S3.Bucket, s3Key)
	return nil
}

// UploadFile uploads a single file to S3
func (u *S3Uploader) UploadFile(ctx context.Context, filePath string) error {
	return u.uploadFile(ctx, filePath, filepath.Dir(filePath))
}
