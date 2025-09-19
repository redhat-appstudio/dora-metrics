# DORA Metrics Server

A professional Go Fiber server with a clean, minimal API focused on health monitoring.

## Features

- 🚀 Built with [Go Fiber](https://gofiber.io/) - Fast HTTP framework
- 🏥 Health check endpoint
- 🔍 WebRCA incident monitoring (checks OpenShift WebRCA API every 30 minutes)
- 🔧 Environment-based configuration
- 📝 Structured logging
- 🌐 CORS support
- 🛡️ Error handling middleware
- 📁 Standard Go project layout

## Quick Start

### Prerequisites

- Go 1.21 or higher
- Git

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd dora-metrics
```

2. Install dependencies:
```bash
go mod tidy
```

3. Copy environment file:
```bash
cp .env.example .env
```

4. Run the server:
```bash
go run .
```

The server will start on `http://localhost:3000` by default.

## API Endpoints

### Health Check
```http
GET /api/v1/health
```

Returns server health status, uptime, and version information.

### Root
```http
GET /
```

Returns basic server information and available endpoints.

## Configuration

The server can be configured using a YAML file (`configs/config.yaml`) with environment variable overrides:

### YAML Configuration

Create `configs/config.yaml`:

```yaml
server:
  port: "3000"
  environment: "development"
  log_level: "info"

webrca:
  enabled: true
  api_url: "https://api.openshift.com/api/web-rca/v1/incidents"
  interval: "30m"
  # OCM token should be set via environment variable OCM_TOKEN for security
```

### Environment Variables

Environment variables override YAML settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | Server port |
| `ENVIRONMENT` | `development` | Environment (development/production) |
| `LOG_LEVEL` | `info` | Log level |
| `WEBRCA_ENABLED` | `true` | Enable WebRCA incident checking |
| `WEBRCA_API_URL` | `https://api.openshift.com/api/web-rca/v1/incidents` | WebRCA API URL |
| `WEBRCA_INTERVAL` | `30m` | WebRCA check interval |
| `OCM_TOKEN` | `` | OCM token for WebRCA API authentication |

## Project Structure

Following the [Go Standard Project Layout](https://github.com/golang-standards/project-layout):

```
.
├── cmd/
│   └── server/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── handlers/
│   │   └── handlers.go      # HTTP handlers and routes
│   └── server/
│       └── server.go        # Server setup and configuration
├── configs/
│   └── config.yaml          # Application configuration
├── go.mod                   # Go module definition
├── Dockerfile               # Docker configuration
└── README.md                # This file
```

## Development

### Running in Development

```bash
# Set your offline token
export OFFLINE_TOKEN="your_offline_token_here"

# Run the application
go run cmd/server/main.go
```

### Building

```bash
# Build the application
go build -o bin/dora-metrics cmd/server/main.go

# Run the binary
./bin/dora-metrics
```

### Running Tests

```bash
go test ./...
```

## Docker Support

To run with Docker:

```bash
# Build the image
docker build -t dora-metrics .

# Run the container
docker run -p 3000:3000 dora-metrics
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.