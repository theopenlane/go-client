package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/samber/lo"
	openlane "github.com/theopenlane/go-client"
	"github.com/theopenlane/go-client/graphclient"
)

func main() {
	apiToken := os.Getenv("OPENLANE_API_TOKEN")
	if apiToken == "" {
		log.Fatal("OPENLANE_API_TOKEN environment variable is required")
	}

	// Initialize the client
	client, err := openlane.New(
		openlane.WithAPIToken(apiToken),
	)

	ctx := context.Background()

	// Query all controls with pagination
	controls, err := getAllControls(client, ctx)
	if err != nil {
		log.Fatalf("Error fetching controls: %v", err)
	}

	// Display results
	fmt.Printf("Total controls fetched: %d\n\n", len(controls))

	for i, control := range controls {
		fmt.Printf("%d. [%s] %s\n", i+1, control.RefCode, *control.Title)
		fmt.Printf("   Category: %s\n",
			*control.Category)
		if control.Description != nil {
			fmt.Printf("   Description: %s\n", *control.Description)
		}
		fmt.Println()
	}
}

// getAllControls fetches all controls using pagination and returns them as a slice
func getAllControls(client *openlane.Client, ctx context.Context) ([]*graphclient.GetControls_Controls_Edges_Node, error) {
	var allControls []*graphclient.GetControls_Controls_Edges_Node

	// Set page size
	pageSize := int64(10)
	where := &graphclient.ControlWhereInput{
		ReferenceFramework: lo.ToPtr("SOC 2"),
		SystemOwned:        lo.ToPtr(true),
	}

	orderBy := &graphclient.ControlOrder{
		Field:     graphclient.ControlOrderFieldRefCode,
		Direction: graphclient.OrderDirectionAsc,
	}
	after := (*string)(nil)

	// Iterate through all pages
	for {
		response, err := client.GetControls(ctx, &pageSize, nil, after, nil, where, []*graphclient.ControlOrder{orderBy})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch controls: %w", err)
		}

		// Collect results from current page
		for _, edge := range response.Controls.Edges {
			allControls = append(allControls, edge.Node)
		}

		fmt.Printf("Fetched page with %d controls (total so far: %d of %d)\n",
			len(response.Controls.Edges), len(allControls), response.Controls.TotalCount)

		// Check if there are more pages
		if !response.Controls.PageInfo.HasNextPage {
			break
		}

		// Update cursor for next page
		after = response.Controls.PageInfo.EndCursor
	}

	return allControls, nil
}
