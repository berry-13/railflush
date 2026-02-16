package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const railwayAPI = "https://backboard.railway.com/graphql/v2"

// Config holds all configuration loaded from environment variables.
type Config struct {
	APIToken      string
	ServiceIDs    []string
	ProjectID     string
	EnvironmentID string
}

// graphqlRequest represents a GraphQL request body.
type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphqlResponse represents a raw GraphQL response.
type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// deploymentsData represents the response from the deployments query.
type deploymentsData struct {
	Deployments struct {
		Edges []struct {
			Node struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"deployments"`
}

// loadConfig reads and validates configuration from environment variables.
func loadConfig() (Config, error) {
	token := os.Getenv("RAILWAY_API_TOKEN")
	if token == "" {
		return Config{}, fmt.Errorf("RAILWAY_API_TOKEN is required")
	}

	raw := os.Getenv("SERVICE_IDS")
	if raw == "" {
		return Config{}, fmt.Errorf("SERVICE_IDS is required")
	}

	var serviceIDs []string
	for _, id := range strings.Split(raw, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			serviceIDs = append(serviceIDs, id)
		}
	}
	if len(serviceIDs) == 0 {
		return Config{}, fmt.Errorf("SERVICE_IDS must contain at least one service ID")
	}

	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		projectID = os.Getenv("RAILWAY_PROJECT_ID")
	}
	if projectID == "" {
		return Config{}, fmt.Errorf("PROJECT_ID (or RAILWAY_PROJECT_ID) is required")
	}

	environmentID := os.Getenv("ENVIRONMENT_ID")
	if environmentID == "" {
		environmentID = os.Getenv("RAILWAY_ENVIRONMENT_ID")
	}
	if environmentID == "" {
		return Config{}, fmt.Errorf("ENVIRONMENT_ID (or RAILWAY_ENVIRONMENT_ID) is required")
	}

	return Config{
		APIToken:      token,
		ServiceIDs:    serviceIDs,
		ProjectID:     projectID,
		EnvironmentID: environmentID,
	}, nil
}

// doGraphQL sends a GraphQL request to the Railway API and returns the parsed response.
func doGraphQL(client *http.Client, token string, query string, variables map[string]any) (*graphqlResponse, error) {
	body, err := json.Marshal(graphqlRequest{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, railwayAPI, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var gqlResp graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
	}

	return &gqlResp, nil
}

const queryLatestDeployment = `
query ($projectId: String!, $environmentId: String!, $serviceId: String!) {
  deployments(
    first: 1
    input: {
      projectId: $projectId
      environmentId: $environmentId
      serviceId: $serviceId
      status: { in: [SUCCESS] }
    }
  ) {
    edges {
      node {
        id
        status
      }
    }
  }
}`

const mutationRestart = `
mutation ($id: String!) {
  deploymentRestart(id: $id)
}`

// getLatestDeployment fetches the latest active deployment for a service.
func getLatestDeployment(client *http.Client, token string, projectID, environmentID, serviceID string) (string, error) {
	resp, err := doGraphQL(client, token, queryLatestDeployment, map[string]any{
		"projectId":     projectID,
		"environmentId": environmentID,
		"serviceId":     serviceID,
	})
	if err != nil {
		return "", fmt.Errorf("querying deployments: %w", err)
	}

	var data deploymentsData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", fmt.Errorf("parsing deployments: %w", err)
	}

	if len(data.Deployments.Edges) == 0 {
		return "", fmt.Errorf("no active deployment found")
	}

	return data.Deployments.Edges[0].Node.ID, nil
}

// restartDeployment triggers a restart for the given deployment ID.
func restartDeployment(client *http.Client, token string, deploymentID string) error {
	_, err := doGraphQL(client, token, mutationRestart, map[string]any{
		"id": deploymentID,
	})
	if err != nil {
		return fmt.Errorf("restarting deployment: %w", err)
	}
	return nil
}

func main() {
	start := time.Now()

	fmt.Println("🚂 railflush — restarting Railway deployments")

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Configuration error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📋 Targeting %d service(s) in project %s\n", len(cfg.ServiceIDs), cfg.ProjectID)

	client := &http.Client{Timeout: 30 * time.Second}

	var succeeded, failed int

	for _, serviceID := range cfg.ServiceIDs {
		fmt.Printf("🔍 Fetching latest deployment for service %s\n", serviceID)

		deploymentID, err := getLatestDeployment(client, cfg.APIToken, cfg.ProjectID, cfg.EnvironmentID, serviceID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Service %s: %v\n", serviceID, err)
			failed++
			continue
		}

		fmt.Printf("🔄 Restarting deployment %s for service %s\n", deploymentID, serviceID)

		if err := restartDeployment(client, cfg.APIToken, deploymentID); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Service %s: %v\n", serviceID, err)
			failed++
			continue
		}

		fmt.Printf("✅ Service %s restarted successfully\n", serviceID)
		succeeded++
	}

	elapsed := time.Since(start).Milliseconds()
	fmt.Printf("🏁 Done: %d restarted, %d failed (%dms)\n", succeeded, failed, elapsed)

	if failed > 0 {
		os.Exit(1)
	}
}
