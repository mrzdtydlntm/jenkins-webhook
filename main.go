package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Jenkins webhook payload structures
type JenkinsWebhook struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Build       Build  `json:"build"`
	DisplayName string `json:"displayName"`
}

type Build struct {
	FullDisplayName string         `json:"fullDisplayName"`
	Number          int            `json:"number"`
	QueueID         int            `json:"queueId"`
	Timestamp       int64          `json:"timestamp"`
	StartTimeMillis int64          `json:"startTimeMillis"`
	Result          string         `json:"result"`
	Duration        int64          `json:"duration"`
	URL             string         `json:"url"`
	Builtby         []any          `json:"builtby"`
	Actions         []any          `json:"actions"`
	Parameters      map[string]any `json:"parameters"`
	Cause           string         `json:"cause"`
	ChangeSets      []any          `json:"changeSets"`
	Culprits        []any          `json:"culprits"`
	NextBuild       any            `json:"nextBuild"`
	PreviousBuild   any            `json:"previousBuild"`
	Phase           string         `json:"phase"`
	Status          string         `json:"status"`
}

// Discord webhook payload structures
type DiscordWebhook struct {
	Content string         `json:"content,omitempty"`
	Embeds  []DiscordEmbed `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	URL         string              `json:"url,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
}

type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type DiscordEmbedFooter struct {
	Text string `json:"text"`
}

type WebhookHandler struct {
	client     *http.Client
	discordURL string
	jenkinsURL string
}

func NewWebhookHandler(discordURL, jenkinsURL string) *WebhookHandler {
	timeout := 30 * time.Second
	client := &http.Client{
		Timeout: timeout,
	}

	return &WebhookHandler{
		client:     client,
		discordURL: discordURL,
		jenkinsURL: jenkinsURL,
	}
}

func (w *WebhookHandler) HandleJenkinsWebhook(c echo.Context) error {
	var payload JenkinsWebhook

	if err := c.Bind(&payload); err != nil {
		log.Printf("Error binding payload: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
	}

	log.Printf("Received Jenkins webhook: %s - Build #%d - %s",
		payload.Name, payload.Build.Number, payload.Build.Status)

	discordPayload := w.convertToDiscordPayload(payload)

	if err := w.sendToDiscord(discordPayload); err != nil {
		log.Printf("Error sending to Discord: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to send to Discord"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "success"})
}

func (w *WebhookHandler) convertToDiscordPayload(jenkins JenkinsWebhook) DiscordWebhook {
	// Determine color based on build result/status
	color := w.getStatusColor(jenkins.Build.Result, jenkins.Build.Status)

	// Format timestamp
	timestamp := time.Unix(jenkins.Build.Timestamp/1000, 0).Format(time.RFC3339)

	// Calculate duration
	duration := w.formatDuration(jenkins.Build.Duration)

	// Create embed fields
	fields := []DiscordEmbedField{
		{
			Name:   "Build Number",
			Value:  fmt.Sprintf("#%d", jenkins.Build.Number),
			Inline: true,
		},
		{
			Name:   "Status",
			Value:  w.getStatusText(jenkins.Build.Result, jenkins.Build.Status),
			Inline: true,
		},
		{
			Name:   "Duration",
			Value:  duration,
			Inline: true,
		},
		{
			Name:   "Phase",
			Value:  jenkins.Build.Phase,
			Inline: true,
		},
	}

	// Add cause if available
	if jenkins.Build.Cause != "" {
		fields = append(fields, DiscordEmbedField{
			Name:   "Cause",
			Value:  jenkins.Build.Cause,
			Inline: false,
		})
	}

	embed := DiscordEmbed{
		Title:       fmt.Sprintf("%s - Build #%d", jenkins.DisplayName, jenkins.Build.Number),
		Description: jenkins.Build.FullDisplayName,
		URL:         jenkins.Build.URL,
		Color:       color,
		Fields:      fields,
		Timestamp:   timestamp,
		Footer: &DiscordEmbedFooter{
			Text: "Jenkins CI/CD",
		},
	}

	return DiscordWebhook{
		Embeds: []DiscordEmbed{embed},
	}
}

func (w *WebhookHandler) getStatusColor(result, status string) int {
	// Check result first, then status
	switch result {
	case "SUCCESS":
		return 0x00FF00 // Green
	case "FAILURE":
		return 0xFF0000 // Red
	case "UNSTABLE":
		return 0xFFA500 // Orange
	case "ABORTED":
		return 0x808080 // Gray
	}

	// If no result, check status
	switch status {
	case "STARTED":
		return 0x0099FF // Blue
	case "COMPLETED":
		return 0x00FF00 // Green
	default:
		return 0x808080 // Gray
	}
}

func (w *WebhookHandler) getStatusText(result, status string) string {
	if result != "" {
		switch result {
		case "SUCCESS":
			return "✅ Success"
		case "FAILURE":
			return "❌ Failure"
		case "UNSTABLE":
			return "⚠️ Unstable"
		case "ABORTED":
			return "🛑 Aborted"
		default:
			return result
		}
	}

	switch status {
	case "STARTED":
		return "🔄 Started"
	case "COMPLETED":
		return "✅ Completed"
	default:
		return status
	}
}

func (w *WebhookHandler) formatDuration(duration int64) string {
	if duration == 0 {
		return "N/A"
	}

	d := time.Duration(duration) * time.Millisecond

	if d.Hours() >= 1 {
		return fmt.Sprintf("%.1fh", d.Hours())
	} else if d.Minutes() >= 1 {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
}

func (w *WebhookHandler) sendToDiscord(payload DiscordWebhook) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling Discord payload: %w", err)
	}

	req, err := http.NewRequest("POST", w.discordURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord API returned status: %d", resp.StatusCode)
	}

	log.Printf("Successfully sent webhook to Discord")
	return nil
}

func (w *WebhookHandler) HandlePrintRequestBody(c echo.Context) error {
	// Read the request body using io.ReadAll
	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to read request body"})
	}

	bodyContent := string(bodyBytes)

	// Print the request body to console
	log.Printf("Request Body Content:\n%s", bodyContent)

	// Also return it in the response
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":         "success",
		"body_content":   bodyContent,
		"content_length": len(bodyContent),
	})
}

func main() {
	// Get environment variables
	discordURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if discordURL == "" {
		log.Fatal("DISCORD_WEBHOOK_URL environment variable is required")
	}

	jenkinsURL := os.Getenv("JENKINS_URL")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Validate port
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatalf("Invalid PORT value: %s", port)
	}

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Create webhook handler
	handler := NewWebhookHandler(discordURL, jenkinsURL)

	// Routes
	e.POST("/webhook/jenkins", handler.HandleJenkinsWebhook)
	e.POST("/webhook/print", handler.HandlePrintRequestBody)
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	// Start server
	log.Printf("Starting server on port %s", port)
	log.Printf("Jenkins webhook endpoint: http://localhost:%s/webhook/jenkins", port)
	log.Printf("Print request body endpoint: http://localhost:%s/webhook/print", port)
	log.Printf("Health check endpoint: http://localhost:%s/health", port)

	if err := e.Start(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
