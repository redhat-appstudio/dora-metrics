# DORA Metrics Server

Automated collection and forwarding of DORA metrics data from ArgoCD deployments and WebRCA incidents to [Apache DevLake](https://devlake.apache.org/). Enables engineering teams to measure software delivery performance through the four key DORA metrics without manual data entry.

## Overview

```text
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│     ArgoCD      │     │  DORA Metrics   │     │  Apache DevLake │
│   Deployments   │────▶│     Server      │────▶│   Dashboards    │
└─────────────────┘     └────────┬────────┘     └─────────────────┘
                                 │
┌─────────────────┐              │
│     WebRCA      │──────────────┘
│    Incidents    │
└─────────────────┘
```

This server watches ArgoCD applications for deployment events, collects incident data from WebRCA, and forwards everything to DevLake via webhooks. DevLake then calculates and visualizes DORA metrics in Grafana dashboards.

## What are DORA Metrics?

[DORA](https://dora.dev/) (DevOps Research and Assessment) metrics are industry-standard indicators for measuring software delivery performance:

| Metric | What It Measures | Data Source |
|--------|------------------|-------------|
| **Deployment Frequency** | How often you deploy to production | ArgoCD deployments |
| **Lead Time for Changes** | Time from commit to production | Commit timestamps → Deployment time |
| **Change Failure Rate** | Percentage of failed deployments | ArgoCD sync failures |
| **Mean Time to Recovery** | Time to recover from incidents | WebRCA incidents |

Learn more in [Apache DevLake's DORA documentation](https://devlake.apache.org/docs/DORA/).

## How It Works

### ArgoCD Integration

The server monitors ArgoCD applications across configured namespaces and:

1. **Detects deployments** when applications sync to new revisions
2. **Extracts commit information** from container images with SHA-based tags
3. **Retrieves commit metadata** from GitHub (author, message, timestamp)
4. **Sends deployment events** to DevLake with full commit history

### WebRCA Integration

For incident tracking, the server:

1. **Polls WebRCA API** at configured intervals
2. **Collects incident data** including creation time and resolution
3. **Forwards incidents** to DevLake for MTTR calculation

### Team Routing

Deployments can be routed to multiple DevLake projects:

- **Global Project**: Receives all deployments for organization-wide visibility
- **Team Projects**: Receive only deployments for specific components

This enables both executive dashboards and team-specific metrics views.

## Quick Start

### Prerequisites

- Go 1.21+
- Redis (for deployment deduplication)
- Access to ArgoCD cluster (for deployment monitoring)
- DevLake instance with webhook configured

### Running Locally

```bash
# Clone and install dependencies
git clone <repository-url>
cd dora-metrics
go mod tidy

# Set required environment variables
export OFFLINE_TOKEN="your_ocm_offline_token"
export DEVLAKE_WEBHOOK_TOKEN="your_devlake_token"

# Run the server
go run cmd/server/main.go
```

### Deploying to Kubernetes

Kubernetes manifests are provided in `manifests/`:

```bash
# For production
kubectl apply -k manifests/production/

# For local/staging
kubectl apply -k manifests/local/
```

## Configuration

Configuration is managed via YAML files with environment variable overrides.

### Key Configuration Areas

| Area | Description |
|------|-------------|
| **ArgoCD** | Namespaces to watch, components to ignore, known clusters |
| **WebRCA** | API endpoint, polling interval |
| **DevLake** | Base URL, project IDs, team routing |
| **Redis** | Connection settings for deployment caching |

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OFFLINE_TOKEN` | Yes | Red Hat OCM offline token for WebRCA API |
| `DEVLAKE_WEBHOOK_TOKEN` | Yes | Authentication token for DevLake webhooks |
| `REDIS_HOST` | No | Redis host (default: from config) |
| `REDIS_PASSWORD` | No | Redis password |

### Example Configuration

```yaml
argocd:
  enabled: true
  namespaces:
    - konflux-public-production
  components_to_ignore:
    - monitoring
    - openshift-gitops

integration:
  devlake:
    enabled: true
    base_url: "https://devlake.example.com"
    project_id: "1"  # Global project
    teams:
      - name: "my-team"
        project_id: "2"
        argocd_components:
          - my-service
          - my-api
```

## Documentation

| Document | Description |
|----------|-------------|
| [Configuration Guide](docs/configuration-sop.md) | Comprehensive configuration reference |
| [Deployment Guide](docs/deployment-guide.md) | Kubernetes deployment instructions |
| [Architecture](docs/architecture.md) | System design and data flow diagrams |

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Server info and available endpoints |
| `GET /api/v1/health` | Health check with uptime and version |

## Development

```bash
# Run tests
go test ./...

# Build binary
go build -o bin/dora-metrics cmd/server/main.go

# Build Docker image
docker build -t dora-metrics .
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
