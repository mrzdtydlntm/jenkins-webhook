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
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Jenkins webhook payload structures
type JenkinsWebhook struct {
	BuildName   string `json:"buildName"`
	BuildUrl    string `json:"buildUrl"`
	BuildVars   string `json:"buildVars"`
	Event       string `json:"event"`
	ProjectName string `json:"projectName"`
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

	log.Printf("Received Jenkins webhook: %s - %s - %s",
		payload.ProjectName, payload.BuildName, payload.Event)

	discordPayload := w.convertToDiscordPayload(payload)

	if err := w.sendToDiscord(discordPayload); err != nil {
		log.Printf("Error sending to Discord: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to send to Discord"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "success"})
}

func (w *WebhookHandler) convertToDiscordPayload(jenkins JenkinsWebhook) DiscordWebhook {
	// Determine color based on event status
	color := w.getEventColor(jenkins.Event)

	// Current timestamp
	timestamp := time.Now().Format(time.RFC3339)

	// Parse build variables
	buildVarsFormatted := w.formatBuildVars(jenkins.BuildVars)

	// Create embed fields
	fields := []DiscordEmbedField{
		{
			Name:   "Build",
			Value:  jenkins.BuildName,
			Inline: true,
		},
		{
			Name:   "Status",
			Value:  w.getEventText(jenkins.Event),
			Inline: true,
		},
		{
			Name:   "Project",
			Value:  jenkins.ProjectName,
			Inline: true,
		},
	}

	// Add build variables if available
	if buildVarsFormatted != "" {
		fields = append(fields, DiscordEmbedField{
			Name:   "Build Variables",
			Value:  buildVarsFormatted,
			Inline: false,
		})
	}

	embed := DiscordEmbed{
		Title:       fmt.Sprintf("%s - %s", jenkins.ProjectName, jenkins.BuildName),
		Description: fmt.Sprintf("Build %s", jenkins.Event),
		URL:         jenkins.BuildUrl,
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

func (w *WebhookHandler) getEventColor(event string) int {
	switch event {
	case "success":
		return 0x00FF00 // Green
	case "failure", "failed":
		return 0xFF0000 // Red
	case "unstable":
		return 0xFFA500 // Orange
	case "aborted":
		return 0x808080 // Gray
	case "started":
		return 0x0099FF // Blue
	default:
		return 0x808080 // Gray
	}
}

func (w *WebhookHandler) getEventText(event string) string {
	switch event {
	case "success":
		return "✅ Success"
	case "failure", "failed":
		return "❌ Failure"
	case "unstable":
		return "⚠️ Unstable"
	case "aborted":
		return "🛑 Aborted"
	case "started":
		return "🔄 Started"
	default:
		return event
	}
}

func (w *WebhookHandler) formatBuildVars(buildVars string) string {
	if buildVars == "" {
		return ""
	}

	// Remove curly braces and format the variables
	cleanVars := strings.Trim(buildVars, "{}")
	if cleanVars == "" {
		return ""
	}

	// Split by comma and format each variable
	vars := strings.Split(cleanVars, ", ")
	var formatted []string

	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			formatted = append(formatted, fmt.Sprintf("**%s**: %s", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])))
		}
	}

	return strings.Join(formatted, "\n")
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
