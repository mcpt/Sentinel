package config

import (
	"fmt"
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Schedule    string `toml:"schedule"`
	TempDir     string `toml:"temp_dir"`
	Debug       bool   `toml:"debug"`
	Compression struct {
		Format string `toml:"format"` // "zstd" or "lz4"
		Level  int    `toml:"level"`
	} `toml:"compression"`

	MySQL struct {
		Enabled  bool   `toml:"enabled"`
		Host     string `toml:"host"`
		Port     string `toml:"port"`
		User     string `toml:"user"`
		Password string `toml:"password"`
		Database string `toml:"database"`
	} `toml:"mysql"`

	Filesystem struct {
		Enabled         bool     `toml:"enabled"`
		BasePath        string   `toml:"base_path"`
		IncludePatterns []string `toml:"include_patterns"`
		ExcludePatterns []string `toml:"exclude_patterns"`
	} `toml:"filesystem"`

	S3 struct {
		Endpoint        string `toml:"endpoint"`
		Region          string `toml:"region"`
		Bucket          string `toml:"bucket"`
		AccessKeyID     string `toml:"access_key_id"`
		SecretAccessKey string `toml:"secret_access_key"`
		MaxConcurrency  int    `toml:"max_concurrency"`
		PartSize        int64  `toml:"part_size"`
	} `toml:"s3"`
}

var Cfg Config

func Load(path string) error {
	if _, err := toml.DecodeFile(path, &Cfg); err != nil {
		return fmt.Errorf("failed to decode config file: %v", err)
	}

	// Validate configuration
	if err := validateConfig(&Cfg); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	return nil

}

func validateConfig(config *Config) error {
	if config.Debug {
		fmt.Println("Debug mode enabled")
	}

	if config.TempDir == "" {
		dir, err := os.MkdirTemp("", "backups")
		if err != nil {
			log.Fatal(err)
		}
		config.TempDir = dir
	}

	// check if ompression format is supported
	if config.Compression.Format != "zlib" && config.Compression.Format != "gzip" && config.Compression.
		Format != "zstd" || config.Compression.Format == "" {
		fmt.Println("Compression format not supported. Please use zlib, gzip or zstd. Defaulting to zstd.")
		config.Compression.Format = "zstd"
	}

	fmt.Printf("Temp dir: %s\n", config.TempDir)
	if _, err := os.Stat(config.TempDir); os.IsNotExist(err) {
		err := os.MkdirAll(config.TempDir, 0750)
		if err != nil {
			log.Fatal(err)
		}
	}
	return nil
}
