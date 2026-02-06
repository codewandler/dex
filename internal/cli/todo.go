package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/codewandler/dex/internal/models"
	"github.com/codewandler/dex/internal/todo"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var todoCmd = &cobra.Command{
	Use:   "todo",
	Short: "Todo list management",
	Long:  `Commands for managing a local todo list with references to external systems.`,
}

var todoAddCmd = &cobra.Command{
	Use:   "add <TITLE> <DESCRIPTION>",
	Short: "Add a new todo",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		store, err := todo.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		t := todo.CreateTodo(args[0], args[1])
		store.AddTodo(t)

		if err := todo.Save(store); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Added %s: %s\n", t.ID, t.Title)
	},
}

var todoUpdateCmd = &cobra.Command{
	Use:   "update <ID>",
	Short: "Update an existing todo",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		store, err := todo.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		t := store.FindTodo(args[0])
		if t == nil {
			fmt.Fprintf(os.Stderr, "Error: todo %s not found\n", args[0])
			os.Exit(1)
		}

		changed := false

		if cmd.Flags().Changed("state") {
			v, _ := cmd.Flags().GetString("state")
			if !models.IsValidTodoState(v) {
				fmt.Fprintf(os.Stderr, "Error: invalid state %q (pending, in_progress, on_hold, done)\n", v)
				os.Exit(1)
			}
			t.State = models.TodoState(v)
			changed = true
		}
		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			t.Title = v
			changed = true
		}
		if cmd.Flags().Changed("desc") {
			v, _ := cmd.Flags().GetString("desc")
			t.Description = v
			changed = true
		}

		if !changed {
			fmt.Fprintf(os.Stderr, "Error: no updates specified (use --state, --title, or --desc)\n")
			os.Exit(1)
		}

		t.UpdatedAt = time.Now()

		if err := todo.Save(store); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Updated %s\n", t.ID)
	},
}

var todoShowCmd = &cobra.Command{
	Use:   "show <ID>",
	Short: "Show todo details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		store, err := todo.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		t := store.FindTodo(args[0])
		if t == nil {
			fmt.Fprintf(os.Stderr, "Error: todo %s not found\n", args[0])
			os.Exit(1)
		}

		printTodoDetail(t)
	},
}

var todoLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List todos",
	Run: func(cmd *cobra.Command, args []string) {
		store, err := todo.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		stateFilter, _ := cmd.Flags().GetString("state")
		if stateFilter != "" && !models.IsValidTodoState(stateFilter) {
			fmt.Fprintf(os.Stderr, "Error: invalid state %q (pending, in_progress, on_hold, done)\n", stateFilter)
			os.Exit(1)
		}

		var filtered []models.Todo
		for _, t := range store.Todos {
			if stateFilter == "" || string(t.State) == stateFilter {
				filtered = append(filtered, t)
			}
		}

		if len(filtered) == 0 {
			fmt.Println("No todos found.")
			return
		}

		printTodoList(filtered)
	},
}

var todoRefCmd = &cobra.Command{
	Use:   "ref",
	Short: "Manage todo references",
}

var todoRefAddCmd = &cobra.Command{
	Use:   "add <TODO_ID> <TYPE> <VALUE>",
	Short: "Add a reference to a todo",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		store, err := todo.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		t := store.FindTodo(args[0])
		if t == nil {
			fmt.Fprintf(os.Stderr, "Error: todo %s not found\n", args[0])
			os.Exit(1)
		}

		ref := todo.CreateReference(args[1], args[2])
		t.References = append(t.References, ref)
		t.UpdatedAt = time.Now()

		if err := todo.Save(store); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Added ref %s to %s (%s: %s)\n", ref.ID, t.ID, ref.Type, ref.Value)
	},
}

var todoRefDelCmd = &cobra.Command{
	Use:   "del <TODO_ID> <REF_ID>",
	Short: "Remove a reference from a todo",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		store, err := todo.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		t := store.FindTodo(args[0])
		if t == nil {
			fmt.Fprintf(os.Stderr, "Error: todo %s not found\n", args[0])
			os.Exit(1)
		}

		if !t.RemoveReference(args[1]) {
			fmt.Fprintf(os.Stderr, "Error: ref %s not found on todo %s\n", args[1], args[0])
			os.Exit(1)
		}

		t.UpdatedAt = time.Now()

		if err := todo.Save(store); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Removed ref %s from %s\n", args[1], args[0])
	},
}

func printTodoDetail(t *models.Todo) {
	stateColors := map[models.TodoState]*color.Color{
		models.TodoStatePending:    color.New(color.FgYellow),
		models.TodoStateInProgress: color.New(color.FgBlue),
		models.TodoStateOnHold:     color.New(color.FgRed),
		models.TodoStateDone:       color.New(color.FgGreen),
	}

	labelColor := color.New(color.FgCyan)
	dimColor := color.New(color.FgHiBlack)

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("  %s\n", t.Title)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()

	labelColor.Printf("  %-14s ", "ID:")
	fmt.Println(t.ID)

	labelColor.Printf("  %-14s ", "State:")
	c := stateColors[t.State]
	if c == nil {
		c = color.New(color.FgWhite)
	}
	c.Println(string(t.State))

	if t.Description != "" {
		labelColor.Printf("  %-14s ", "Description:")
		fmt.Println(t.Description)
	}

	labelColor.Printf("  %-14s ", "Created:")
	dimColor.Println(t.CreatedAt.Format("2006-01-02 15:04"))

	labelColor.Printf("  %-14s ", "Updated:")
	dimColor.Println(t.UpdatedAt.Format("2006-01-02 15:04"))

	if len(t.References) > 0 {
		fmt.Println()
		labelColor.Printf("  References (%d):\n", len(t.References))
		for _, ref := range t.References {
			dimColor.Printf("    [%s] ", ref.ID)
			fmt.Printf("%s: %s\n", ref.Type, ref.Value)
		}
	}

	fmt.Println()
}

func printTodoList(todos []models.Todo) {
	stateColors := map[models.TodoState]*color.Color{
		models.TodoStatePending:    color.New(color.FgYellow),
		models.TodoStateInProgress: color.New(color.FgBlue),
		models.TodoStateOnHold:     color.New(color.FgRed),
		models.TodoStateDone:       color.New(color.FgGreen),
	}

	dimColor := color.New(color.FgHiBlack)

	// Print header
	fmt.Printf("%-8s %-14s %-45s %s\n", "ID", "STATE", "TITLE", "REFS")
	fmt.Println(strings.Repeat("─", 76))

	for _, t := range todos {
		c := stateColors[t.State]
		if c == nil {
			c = color.New(color.FgWhite)
		}

		title := t.Title
		if len(title) > 45 {
			title = title[:42] + "..."
		}

		refStr := ""
		if len(t.References) > 0 {
			refStr = fmt.Sprintf("%d", len(t.References))
		}

		fmt.Printf("%-8s ", t.ID)
		c.Printf("%-14s ", string(t.State))
		fmt.Printf("%-45s ", title)
		dimColor.Printf("%s\n", refStr)
	}

	fmt.Printf("\n%d todos\n", len(todos))
}

func init() {
	todoCmd.AddCommand(todoAddCmd)
	todoCmd.AddCommand(todoShowCmd)
	todoCmd.AddCommand(todoUpdateCmd)
	todoCmd.AddCommand(todoLsCmd)
	todoCmd.AddCommand(todoRefCmd)

	todoRefCmd.AddCommand(todoRefAddCmd)
	todoRefCmd.AddCommand(todoRefDelCmd)

	todoUpdateCmd.Flags().StringP("state", "s", "", "New state (pending, in_progress, on_hold, done)")
	todoUpdateCmd.Flags().StringP("title", "t", "", "New title")
	todoUpdateCmd.Flags().StringP("desc", "d", "", "New description")

	todoLsCmd.Flags().StringP("state", "s", "", "Filter by state (pending, in_progress, on_hold, done)")
}
