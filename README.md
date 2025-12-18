# go-client

A Go client library for interacting with the Openlane API.

## Installation

```bash
go get github.com/theopenlane/go-client
```

## Features

- Full GraphQL and REST API support
- Built-in pagination support
- Type-safe queries and mutations
- Authentication handling
- Comprehensive query operations (CRUD)

## Usage

### Basic Setup

```go
    import (
        openlane "github.com/theopenlane/go-client"
    )

    // get the api token
    apiToken := os.Getenv("OPENLANE_API_TOKEN")

	// Initialize the client
	client, err := openlane.New(
		openlane.WithAPIToken(apiToken),
	)
```

### Basic Queries

#### Get a Resource by ID

```go
ctx := context.Background()

// Get a control by ID
control, err := client.GetControlByID(ctx, "control-id")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Control: %s\n", control.Control.RefCode)
```

#### Get All Resources

```go
// Get all controls you have access to view
controls, err := client.GetAllControls(ctx, first, last, after, before, orderBy)
if err != nil {
    log.Fatal(err)
}

for _, ctrl := range controls.Edges {
    fmt.Printf("Control: %s - %s\n", ctrl.Node.RefCode, ctrl.Node.Title)
}
```

### Pagination

The client supports cursor-based pagination using `first`, `last`, `after`, and `before` parameters.

#### Forward Pagination

```go
// Get first 10 controls
pageSize := int64(10)
after := (*string)(nil)

for {
		response, err := client.GetControls(ctx, &pageSize, nil, after, nil, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch controls: %w", err)
		}

		// Collect results from current page
		for _, edge := range response.Controls.Edges {
			allControls = append(allControls, edge.Node)
		}

		// Check if there are more pages
		if !response.Controls.PageInfo.HasNextPage {
			break
		}

		// Update cursor for next page
		after = response.Controls.PageInfo.EndCursor
	}
```

#### Backward Pagination

```go
// Get last 10 controls
pageSize := int64(10)
before := (*string)(nil)

for {
		response, err := client.GetControls(ctx, &pageSize, nil, nil, before, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch controls: %w", err)
		}

		// Collect results from current page
		for _, edge := range response.Controls.Edges {
			allControls = append(allControls, edge.Node)
		}

		// Check if there are more pages
		if !response.Controls.PageInfo.HasPreviousPage {
			break
		}

		// Update cursor for next page
		after = response.Controls.PageInfo.StartCursor
	}
```

For a full example of pagination through all results, refer to the [example](example/main.go):

```bash
go run example/main.go
Fetched page with 10 controls (total so far: 10 of 61)
Fetched page with 10 controls (total so far: 20 of 61)
Fetched page with 10 controls (total so far: 30 of 61)
Fetched page with 10 controls (total so far: 40 of 61)
Fetched page with 10 controls (total so far: 50 of 61)
Fetched page with 10 controls (total so far: 60 of 61)
Fetched page with 1 controls (total so far: 61 of 61)
Total controls fetched: 61

1. [A1.1]
   Category: Availability
...
```

### Filtering and Ordering

#### Using Where Clauses

All `Get` and `GetAll` functions support pagination and ordering, the `Get` functions support `where` filters as well.

```go
// Get controls with specific filters
	pageSize := int64(10)

    // where filter
	where := &graphclient.ControlWhereInput{
		ReferenceFramework: lo.ToPtr("SOC 2"),
		SystemOwned:        lo.ToPtr(true),
	}

    // order fields
	orderBy := &graphclient.ControlOrder{
		Field:     graphclient.ControlOrderFieldRefCode,
		Direction: graphclient.OrderDirectionAsc,
	}
	after := (*string)(nil)

    client.GetControls(ctx, &pageSize, nil, after, nil, where, []*graphclient.ControlOrder{orderBy})
    if err != nil {
        log.Fatal(err)
    }
```


### Mutations

#### Create a Resource

```go
	input := graphclient.CreateControlInput{
		RefCode: "AC-001",
	}

	resp, err := client.CreateControl(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create control: %w", err)
	}

	fmt.Printf("Created control with ID: %s, RefCode: %s\n", resp.CreateControl.Control.ID, resp.CreateControl.Control.RefCode)
```

#### Update a Resource

```go
	input := graphclient.UpdateControlInput{
		Description: lo.ToPtr("description of the control"),
	}

	resp, err := client.UpdateControl(ctx, "control-id", input)
	if err != nil {
		return nil, fmt.Errorf("failed to create control: %w", err)
	}

	fmt.Printf("Updated control: %s\n", resp.UpdateControl.Control.ID)
```

#### Delete a Resource

```go
	resp, err := client.DeleteControl(ctx, "control-id")
	if err != nil {
		return nil, fmt.Errorf("failed to delete control: %w", err)
	}
	fmt.Printf("Deleted Control: %s\n", resp.DeleteControl.DeletedID)
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

Apache License Version 2.0,

## Support

For questions and support, please open an issue on GitHub.