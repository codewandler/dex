package main

import (
	"fmt"
	"os"
	"time"

	"dev-activity/internal/config"
	"dev-activity/internal/gitlab"
	"dev-activity/internal/models"
	"dev-activity/internal/output"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	client, err := gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create GitLab client: %v\n", err)
		os.Exit(1)
	}

	since := time.Now().AddDate(0, 0, -cfg.ActivityDays)

	fmt.Printf("Fetching projects with activity since %s...\n", since.Format("2006-01-02"))

	projects, err := client.GetActiveProjects(since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch projects: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d projects with recent activity, fetching details...\n", len(projects))

	var activities []models.ProjectActivity

	for i, project := range projects {
		fmt.Printf("\r  Processing %d/%d: %s", i+1, len(projects), project.PathWithNamespace)

		activity := models.ProjectActivity{
			ProjectID:   project.ID,
			ProjectName: project.Name,
			ProjectPath: project.PathWithNamespace,
			WebURL:      project.WebURL,
		}

		commits, err := client.GetCommits(project.ID, since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nWarning: Failed to fetch commits for %s: %v\n", project.PathWithNamespace, err)
		} else {
			activity.Commits = commits
		}

		mrs, err := client.GetMergeRequests(project.ID, since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nWarning: Failed to fetch merge requests for %s: %v\n", project.PathWithNamespace, err)
		} else {
			activity.MergeRequests = mrs
		}

		tags, err := client.GetTags(project.ID, since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nWarning: Failed to fetch tags for %s: %v\n", project.PathWithNamespace, err)
		} else {
			activity.Tags = tags
		}

		if activity.HasActivity() {
			activities = append(activities, activity)
		}
	}

	fmt.Print("\r" + "                                                                              " + "\r")

	output.PrintHeader(cfg.ActivityDays)

	if len(activities) == 0 {
		output.PrintNoActivity()
		return
	}

	for _, activity := range activities {
		output.PrintProject(activity)
	}

	summary := models.CalculateSummary(activities)
	output.PrintSummary(summary)
}
