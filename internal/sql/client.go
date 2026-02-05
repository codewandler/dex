package sql

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/codewandler/dex/internal/config"
)

// Client wraps a SQL database connection
type Client struct {
	db         *sql.DB
	datasource string
	readOnly   bool
}

// NewClient creates a new SQL client for the specified datasource (read-only by default)
func NewClient(datasourceName string) (*Client, error) {
	return NewClientWithOptions(datasourceName, true)
}

// NewClientWithOptions creates a new SQL client with configurable read-only mode
func NewClientWithOptions(datasourceName string, readOnly bool) (*Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	ds, ok := cfg.SQL.Datasources[datasourceName]
	if !ok {
		return nil, fmt.Errorf("datasource %q not found in config", datasourceName)
	}

	port := ds.Port
	if port == 0 {
		port = 3306
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		ds.Username, ds.Password, ds.Host, port, ds.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set read-only mode if requested
	if readOnly {
		if _, err := db.Exec("SET SESSION TRANSACTION READ ONLY"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set read-only mode: %w", err)
		}
	}

	return &Client{
		db:         db,
		datasource: datasourceName,
		readOnly:   readOnly,
	}, nil
}

// Close closes the database connection
func (c *Client) Close() error {
	return c.db.Close()
}

// QueryResult holds the results of a query
type QueryResult struct {
	Columns []string
	Rows    [][]any
}

// Query executes a query and returns the results
func (c *Client) Query(ctx context.Context, query string) (*QueryResult, error) {
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	result := &QueryResult{
		Columns: columns,
		Rows:    make([][]any, 0),
	}

	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert []byte to string for readability
		row := make([]any, len(columns))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		result.Rows = append(result.Rows, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}

// ListDatasources returns all configured datasource names
func ListDatasources() ([]string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(cfg.SQL.Datasources))
	for name := range cfg.SQL.Datasources {
		names = append(names, name)
	}
	return names, nil
}
