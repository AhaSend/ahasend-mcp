// Command ahasend-mcp-dev serves the AhaSend MCP tools over stdio, for
// manual testing with a local MCP client. It is not released.
//
// It reads AHASEND_API_KEY and AHASEND_ACCOUNT_ID, and AHASEND_BASE_URL
// for a base URL other than the default.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	ahasendmcp "github.com/AhaSend/ahasend-mcp"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const dashboardBaseURL = "https://dash.ahasend.com"

func main() {
	if err := run(); err != nil {
		log.Fatalf("ahasend-mcp-dev: %v", err)
	}
}

func run() error {
	apiKey := os.Getenv("AHASEND_API_KEY")
	if apiKey == "" {
		return errors.New("AHASEND_API_KEY is not set")
	}
	accountID, err := uuid.Parse(os.Getenv("AHASEND_ACCOUNT_ID"))
	if err != nil {
		return fmt.Errorf("AHASEND_ACCOUNT_ID is not an account ID: %w", err)
	}

	clientOpts := []api.ClientOption{api.WithAPIKey(apiKey)}
	clientOpts = append(clientOpts, func(cfg *api.Configuration) {
		cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	})
	if raw := os.Getenv("AHASEND_BASE_URL"); raw != "" {
		base, err := url.Parse(raw)
		if err != nil || base.Scheme == "" || base.Host == "" {
			return fmt.Errorf("AHASEND_BASE_URL is not a URL such as https://api.ahasend.com: %q", raw)
		}
		clientOpts = append(clientOpts, func(cfg *api.Configuration) {
			cfg.Scheme = base.Scheme
			cfg.Host = base.Host
		})
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("making the confirmation key: %w", err)
	}

	server, err := ahasendmcp.NewServer(
		&mcp.Implementation{Name: "ahasend-mcp-dev", Version: "dev"},
		ahasendmcp.Options{
			Client:           api.NewAPIClient(clientOpts...),
			AccountID:        accountID,
			Scopes:           nil,
			DashboardBaseURL: dashboardBaseURL,
			ConfirmationKey:  key,
		},
	)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
