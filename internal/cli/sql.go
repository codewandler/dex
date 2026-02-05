package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/sql"
	"github.com/spf13/cobra"
)

var sqlCmd = &cobra.Command{
	Use:   "sql",
	Short: "SQL database operations",
	Long:  `Commands for querying SQL databases.`,
}

var sqlQueryCmd = &cobra.Command{
	Use:   "query <QUERY>",
	Short: "Execute a SQL query",
	Long: `Execute a SQL query against a configured datasource.

Examples:
  dex sql query -d eu:read "SELECT * FROM users LIMIT 10"
  dex sql query -d eu:read "SELECT COUNT(*) FROM orders WHERE created_at > '2024-01-01'"`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		datasource, _ := cmd.Flags().GetString("datasource")
		if datasource == "" {
			fmt.Fprintf(os.Stderr, "Error: --datasource is required\n")
			os.Exit(1)
		}

		query := args[0]

		client, err := sql.NewClient(datasource)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer client.Close()

		result, err := client.Query(ctx, query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(result.Rows) == 0 {
			fmt.Println("No results.")
			return
		}

		// Calculate column widths
		widths := make([]int, len(result.Columns))
		for i, col := range result.Columns {
			widths[i] = len(col)
		}
		for _, row := range result.Rows {
			for i, val := range row {
				str := formatValue(val)
				if len(str) > widths[i] {
					widths[i] = len(str)
				}
			}
		}

		// Cap widths at 50 chars
		for i := range widths {
			if widths[i] > 50 {
				widths[i] = 50
			}
		}

		// Print header
		var header strings.Builder
		var separator strings.Builder
		for i, col := range result.Columns {
			if i > 0 {
				header.WriteString(" | ")
				separator.WriteString("-+-")
			}
			header.WriteString(fmt.Sprintf("%-*s", widths[i], truncateStr(col, widths[i])))
			separator.WriteString(strings.Repeat("-", widths[i]))
		}
		fmt.Println(header.String())
		fmt.Println(separator.String())

		// Print rows
		for _, row := range result.Rows {
			var line strings.Builder
			for i, val := range row {
				if i > 0 {
					line.WriteString(" | ")
				}
				str := formatValue(val)
				line.WriteString(fmt.Sprintf("%-*s", widths[i], truncateStr(str, widths[i])))
			}
			fmt.Println(line.String())
		}

		fmt.Printf("\n%d rows\n", len(result.Rows))
	},
}

var sqlDatasourcesCmd = &cobra.Command{
	Use:   "datasources",
	Short: "List configured datasources",
	Run: func(cmd *cobra.Command, args []string) {
		datasources, err := sql.ListDatasources()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(datasources) == 0 {
			fmt.Println("No datasources configured.")
			return
		}

		for _, ds := range datasources {
			fmt.Println(ds)
		}
	},
}

func init() {
	sqlCmd.AddCommand(sqlQueryCmd)
	sqlCmd.AddCommand(sqlDatasourcesCmd)

	sqlQueryCmd.Flags().StringP("datasource", "d", "", "Datasource name from config")
	sqlQueryCmd.MarkFlagRequired("datasource")
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	return fmt.Sprintf("%v", v)
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
