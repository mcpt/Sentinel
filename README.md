# Sentinel

Sentinel is a robust, modular backup system designed to secure your MySQL/MariaDB databases and files to S3-compatible storage solutions, with support for various compression formats and flexible scheduling.

## Features

- Modular backup system supporting multiple backup sources:
   - MySQL/MariaDB database backups using mysqldump
   - File and directory backups with pattern matching
   - Easy to extend with new backup handlers
- Advanced compression options:
   - Support for gzip and zstd compression
   - Configurable compression levels
- Flexible storage options:
   - Compatible with S3, Cloudflare R2, MinIO, and other S3-compatible storage
   - Configurable upload parameters (part size, concurrency)
- Configurable via TOML configuration file
- Debug mode for troubleshooting
- Temporary directory management
- Docker support with optional container database backup

## Prerequisites

- Go 1.16+ (for building from source)
- Docker (optional, for containerized deployment)
- MySQL/MariaDB (if database backup is enabled)
- S3-compatible storage account

## Configuration

Sentinel uses a TOML configuration file. Here's a sample configuration with all available options:

```toml
# Backup system configuration
schedule = "0 4 * * *"  # Daily at 4 AM (if not specified, backup will run immediately)
temp_dir = ""          # Optional: temporary directory for backups
debug = false          # Enable debug logging

[compression]
format = "gzip"        # Supported: "gzip", "zstd"
level = 3             # Compression level (1-9)

[mysql]
enabled = false        # Enable/disable MySQL backup
host = "localhost"
port = "3306"
user = "backup_user"
password = "backup_password"
database = "myapp"
docker_container = ""  # Optional: MySQL docker container name

[filesystem]
enabled = true
base_path = "/path/to/backup"
include_patterns = [   # Glob patterns for files to include
    "*.txt",
    "*.pdf",
    "config/**",
    "data/**"
]
exclude_patterns = [   # Glob patterns for files to exclude
    ".git/**",
    "node_modules/**",
    "tmp/**",
    "*.tmp"
]

[s3]
endpoint = "https://your-endpoint.com"
region = "auto"       # Use "auto" for services like R2
bucket = "your-bucket"
access_key_id = "your-access-key"
secret_access_key = "your-secret-key"
max_concurrency = 10  # Maximum concurrent uploads
part_size = 0        # Multipart upload part size (0 for auto)
```

## Installation

### Using Docker

1. Clone the repository:
   ```bash
   git clone https://github.com/mcpt/sentinel.git
   ```

2. Build the Docker image:
   ```bash
   docker build -t sentinel .
   ```

3. Create your config.toml file based on the example above.

4. Run the container:
   ```bash
   docker run -d --name sentinel \
     -v /path/to/your/config.toml:/app/config.toml \
     -v /path/to/backup:/data \
     sentinel
   ```

### Building from Source

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/sentinel.git
   ```

2. Navigate to the project directory:
   ```bash
   cd sentinel
   ```

3. Build the binary:
   ```bash
   go build -o sentinel cmd/backup-system/main.go
   ```

4. Create your config.toml and run:
   ```bash
   ./sentinel --config /path/to/config.toml
   ```

## Architecture

Sentinel uses a modular architecture with the following components:

- Backup Handlers: Implement the `BackupHandler` interface for different backup sources
- Storage: S3-compatible storage implementation
- Compression: Supports multiple compression formats
- Configuration: TOML-based configuration system

## Development

To contribute to Sentinel:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Adding New Backup Handlers

Implement the `BackupHandler` interface to add support for new backup sources:

```go
type BackupHandler interface {
    Backup(ctx context.Context) (string, error)
}
```

## License

Distributed under the GPL-3.0 License. See `LICENSE` for more information.

## Acknowledgments

- The Go community
- Contributors to the AWS SDK for Go
- Open-source backup solution developers