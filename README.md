# Jenkins to Discord Webhook Converter

A Go application that converts Jenkins webhook payloads to Discord webhook format using Echo framework and standard HTTP client.

## Features

- Converts Jenkins webhook payloads to Discord-compatible format
- Beautiful Discord embeds with proper formatting
- Color-coded status indicators
- Comprehensive build information display
- Health check endpoint
- JSON-structured logging

## Prerequisites

- Go 1.21 or higher
- Jenkins with Outbound Webhook Plugin
- Discord server with webhook URL

## Setup

### 1. Environment Variables

Create a `.env` file or set these environment variables:

```bash
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_WEBHOOK_URL
JENKINS_URL=http://your-jenkins-instance.com  # Optional
PORT=8080  # Optional, defaults to 8080
```

### 2. Installation

```bash
# Clone or download the project
git clone <repository-url>
cd jenkins-webhook-discord

# Download dependencies
go mod tidy

# Build the application
go build -o jenkins-webhook-discord main.go
```

### 3. Running the Application

```bash
# Set environment variables
export DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/YOUR_WEBHOOK_URL"

# Run the application
./jenkins-webhook-discord

# Or run directly with go
go run main.go
```

## Usage

### Jenkins Configuration

1. Install the "Outbound Webhook Plugin" in Jenkins
2. Configure a webhook in your Jenkins job:
   - **URL**: `http://your-server:8080/webhook/jenkins`
   - **Content Type**: `application/json`
   - **Events**: Select the events you want to monitor (e.g., Build Started, Build Completed)

### Discord Setup

1. Create a webhook in your Discord server:
   - Go to Server Settings → Integrations → Webhooks
   - Click "New Webhook"
   - Copy the webhook URL
   - Set it as the `DISCORD_WEBHOOK_URL` environment variable

## API Endpoints

### POST /webhook/jenkins
Receives Jenkins webhook payloads and converts them to Discord format.

**Example Jenkins Payload:**
```json
{
  "name": "my-project",
  "displayName": "My Project",
  "url": "http://jenkins.example.com/job/my-project/",
  "build": {
    "fullDisplayName": "My Project #42",
    "number": 42,
    "queueId": 123,
    "timestamp": 1642584000000,
    "startTimeMillis": 1642584000000,
    "result": "SUCCESS",
    "duration": 125000,
    "url": "http://jenkins.example.com/job/my-project/42/",
    "phase": "COMPLETED",
    "status": "SUCCESS",
    "cause": "Started by user admin"
  }
}
```

### GET /health
Health check endpoint that returns the service status.

**Response:**
```json
{
  "status": "healthy"
}
```

## Discord Message Format

The application converts Jenkins webhooks into rich Discord embeds with:

- **Title**: Project name and build number
- **Description**: Full display name
- **URL**: Direct link to Jenkins build
- **Color**: Status-based color coding
  - 🟢 Green: SUCCESS
  - 🔴 Red: FAILURE  
  - 🟠 Orange: UNSTABLE
  - ⚫ Gray: ABORTED
  - 🔵 Blue: STARTED
- **Fields**:
  - Build Number
  - Status (with emoji)
  - Duration
  - Phase
  - Cause (if available)
- **Footer**: "Jenkins CI/CD"
- **Timestamp**: Build timestamp

## Status Indicators

| Jenkins Status | Discord Display | Color |
|---------------|-----------------|-------|
| SUCCESS       | ✅ Success      | Green |
| FAILURE       | ❌ Failure      | Red   |
| UNSTABLE      | ⚠️ Unstable     | Orange |
| ABORTED       | 🛑 Aborted      | Gray  |
| STARTED       | 🔄 Started      | Blue  |

## Development

### Building
```bash
go build -o jenkins-webhook-discord main.go
```

### Testing
```bash
# Test the health endpoint
curl http://localhost:8080/health

# Test with a sample Jenkins payload
curl -X POST http://localhost:8080/webhook/jenkins \
  -H "Content-Type: application/json" \
  -d @sample-payload.json
```

### Docker Support

Create a `Dockerfile`:
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o jenkins-webhook-discord main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/jenkins-webhook-discord .
EXPOSE 8080
CMD ["./jenkins-webhook-discord"]
```

Build and run:
```bash
docker build -t jenkins-webhook-discord .
docker run -p 8080:8080 \
  -e DISCORD_WEBHOOK_URL="your_webhook_url" \
  jenkins-webhook-discord
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
