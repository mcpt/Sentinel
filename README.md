# Sentinel

Sentinel is an automated backup system designed to secure your MariaDB databases and specified files to S3-compatible storage solutions.

## Features

- Automated MariaDB database backups using mysqldump
- File and directory backups using gitignore-style pattern matching
- Scheduled backups using cron syntax
- Compatible with S3, Cloudflare R2, MinIO, and other S3-compatible storage
- Configuration via environment variables for enhanced security
- Dockerized for easy deployment
- Optional configuration file path specified via command-line flag

## Prerequisites

- Go 1.16+ (for building from source)
- Docker (for containerized deployment)
- MariaDB
- S3-compatible storage account

## Installation

### Using Docker

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/sentinel.git
   ```

2. Build the Docker image:
   ```
   docker build -t sentinel .
   ```

3. Create a `backup_include.txt` file with your backup patterns:
   ```
   /path/to/important/files/*
   !/path/to/important/files/temp
   /var/www/html/**/*.php
   ```

4. Run the container:
   ```
   docker run -d --name sentinel \
     -v /path/to/your/backup_include.txt:/root/backup_include.txt \
     -e DB_HOST=your_db_host \
     -e DB_PORT=your_db_port \
     -e DB_USER=your_db_user \
     -e DB_PASSWORD=your_db_password \
     -e DB_NAME=your_db_name \
     -e S3_ENDPOINT=your_s3_endpoint \
     -e S3_REGION=your_s3_region \
     -e S3_ACCESS_KEY_ID=your_s3_access_key \
     -e S3_SECRET_ACCESS_KEY=your_s3_secret_key \
     -e S3_BUCKET=your_s3_bucket \
     sentinel
   ```

   To use a custom path for the configuration file:
   ```
   docker run -d --name sentinel \
     -v /path/to/your/custom_config.txt:/root/custom_config.txt \
     ... [other environment variables] ...
     sentinel --config /root/custom_config.txt
   ```

### Building from Source

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/sentinel.git
   ```

2. Navigate to the project directory:
   ```
   cd sentinel
   ```

3. Build the binary:
   ```
   go build -o sentinel
   ```

4. Set up environment variables and run:
   ```
   export DB_HOST=your_db_host
   export DB_PORT=your_db_port
   # ... set other environment variables ...
   ./sentinel
   ```

   Or, to use a custom configuration file:
   ```
   ./sentinel --config /path/to/your/custom_config.txt
   ```

## Configuration

1. Create a configuration file (default: `backup_include.txt`) with your backup patterns:
   ```
   /path/to/important/files/*
   !/path/to/important/files/temp
   /var/www/html/**/*.php
   ```

2. Set the following environment variables:
   - `DB_HOST`: MariaDB host
   - `DB_PORT`: MariaDB port
   - `DB_USER`: MariaDB username
   - `DB_PASSWORD`: MariaDB password
   - `DB_NAME`: Database name
   - `S3_ENDPOINT`: S3-compatible storage endpoint
   - `S3_REGION`: S3 region
   - `S3_ACCESS_KEY_ID`: S3 access key
   - `S3_SECRET_ACCESS_KEY`: S3 secret key
   - `S3_BUCKET`: S3 bucket name

## Development

To contribute to Sentinel:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

Distributed under the GPL-3.0 License. See `LICENSE` for more information.

## Acknowledgments

- The Go community
- Contributors to the AWS SDK for Go
- Open-source backup solution developers