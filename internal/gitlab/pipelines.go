package gitlab

import (
	"fmt"
	"io"
	"strings"

	"github.com/codewandler/dex/internal/models"

	"github.com/xanzy/go-gitlab"
)

// ListPipelinesOptions configures the pipeline list query
type ListPipelinesOptions struct {
	ProjectID string
	Status    string // running, pending, success, failed, canceled, skipped, manual, created
	Ref       string // branch or tag name
	Source    string // push, web, trigger, schedule, api, merge_request_event
	Sort      string // asc, desc
	Limit     int
}

// ListPipelines fetches pipelines for a project
func (c *Client) ListPipelines(opts ListPipelinesOptions) ([]models.PipelineSummary, error) {
	pid, err := c.resolveProjectID(opts.ProjectID)
	if err != nil {
		return nil, err
	}

	if opts.Limit == 0 {
		opts.Limit = 20
	}
	if opts.Sort == "" {
		opts.Sort = "desc"
	}

	listOpts := &gitlab.ListProjectPipelinesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: min(opts.Limit, 100),
			Page:    1,
		},
		Sort: gitlab.Ptr(opts.Sort),
	}

	if opts.Status != "" {
		listOpts.Status = gitlab.Ptr(gitlab.BuildStateValue(opts.Status))
	}
	if opts.Ref != "" {
		listOpts.Ref = gitlab.Ptr(opts.Ref)
	}
	if opts.Source != "" {
		listOpts.Source = gitlab.Ptr(opts.Source)
	}

	var result []models.PipelineSummary

	for {
		pipelines, resp, err := c.gl.Pipelines.ListProjectPipelines(pid, listOpts)
		if err != nil {
			return nil, err
		}

		for _, p := range pipelines {
			ps := models.PipelineSummary{
				ID:        p.ID,
				IID:       p.IID,
				ProjectID: p.ProjectID,
				Status:    p.Status,
				Source:    p.Source,
				Ref:       p.Ref,
				SHA:       p.SHA,
				WebURL:    p.WebURL,
			}
			if p.CreatedAt != nil {
				ps.CreatedAt = *p.CreatedAt
			}
			if p.UpdatedAt != nil {
				ps.UpdatedAt = *p.UpdatedAt
			}
			result = append(result, ps)

			if len(result) >= opts.Limit {
				return result, nil
			}
		}

		if resp.NextPage == 0 || len(result) >= opts.Limit {
			break
		}
		listOpts.Page = resp.NextPage
	}

	return result, nil
}

// GetPipeline fetches a single pipeline with full details
func (c *Client) GetPipeline(projectID any, pipelineID int) (*models.PipelineDetail, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	p, _, err := c.gl.Pipelines.GetPipeline(pid, pipelineID)
	if err != nil {
		return nil, err
	}

	detail := &models.PipelineDetail{
		ID:             p.ID,
		IID:            p.IID,
		ProjectID:      p.ProjectID,
		Status:         p.Status,
		Source:         p.Source,
		Ref:            p.Ref,
		SHA:            p.SHA,
		BeforeSHA:      p.BeforeSHA,
		Tag:            p.Tag,
		YamlErrors:     p.YamlErrors,
		WebURL:         p.WebURL,
		Duration:       p.Duration,
		QueuedDuration: p.QueuedDuration,
		Coverage:       p.Coverage,
	}

	if p.User != nil {
		detail.User = p.User.Username
	}
	if p.CreatedAt != nil {
		detail.CreatedAt = *p.CreatedAt
	}
	if p.UpdatedAt != nil {
		detail.UpdatedAt = *p.UpdatedAt
	}
	detail.StartedAt = p.StartedAt
	detail.FinishedAt = p.FinishedAt

	return detail, nil
}

// ListPipelineJobs fetches jobs for a specific pipeline
func (c *Client) ListPipelineJobs(projectID any, pipelineID int, scope string) ([]models.PipelineJob, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	opts := &gitlab.ListJobsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	if scope != "" {
		scopes := []gitlab.BuildStateValue{gitlab.BuildStateValue(scope)}
		opts.Scope = &scopes
	}

	var result []models.PipelineJob

	for {
		jobs, resp, err := c.gl.Jobs.ListPipelineJobs(pid, pipelineID, opts)
		if err != nil {
			return nil, err
		}

		for _, j := range jobs {
			job := models.PipelineJob{
				ID:             j.ID,
				Name:           j.Name,
				Stage:          j.Stage,
				Status:         j.Status,
				Ref:            j.Ref,
				Tag:            j.Tag,
				AllowFailure:   j.AllowFailure,
				Duration:       j.Duration,
				QueuedDuration: j.QueuedDuration,
				FailureReason:  j.FailureReason,
				WebURL:         j.WebURL,
			}
			if j.User != nil {
				job.User = j.User.Username
			}
			if j.CreatedAt != nil {
				job.CreatedAt = *j.CreatedAt
			}
			job.StartedAt = j.StartedAt
			job.FinishedAt = j.FinishedAt
			result = append(result, job)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return result, nil
}

// RetryPipeline retries failed jobs in a pipeline
func (c *Client) RetryPipeline(projectID any, pipelineID int) (*models.PipelineDetail, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	p, _, err := c.gl.Pipelines.RetryPipelineBuild(pid, pipelineID)
	if err != nil {
		return nil, err
	}

	detail := &models.PipelineDetail{
		ID:     p.ID,
		Status: p.Status,
		Ref:    p.Ref,
		WebURL: p.WebURL,
	}
	if p.User != nil {
		detail.User = p.User.Username
	}

	return detail, nil
}

// CancelPipeline cancels a running pipeline
func (c *Client) CancelPipeline(projectID any, pipelineID int) (*models.PipelineDetail, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	p, _, err := c.gl.Pipelines.CancelPipelineBuild(pid, pipelineID)
	if err != nil {
		return nil, err
	}

	detail := &models.PipelineDetail{
		ID:     p.ID,
		Status: p.Status,
		Ref:    p.Ref,
		WebURL: p.WebURL,
	}
	if p.User != nil {
		detail.User = p.User.Username
	}

	return detail, nil
}

// CreatePipelineOptions contains options for creating a new pipeline
type CreatePipelineOptions struct {
	Ref       string
	Variables map[string]string // KEY=VALUE pairs
}

// CreatePipeline triggers a new pipeline on a ref
func (c *Client) CreatePipeline(projectID any, opts CreatePipelineOptions) (*models.PipelineDetail, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	createOpts := &gitlab.CreatePipelineOptions{
		Ref: gitlab.Ptr(opts.Ref),
	}

	if len(opts.Variables) > 0 {
		var vars []*gitlab.PipelineVariableOptions
		for k, v := range opts.Variables {
			vars = append(vars, &gitlab.PipelineVariableOptions{
				Key:          gitlab.Ptr(k),
				Value:        gitlab.Ptr(v),
				VariableType: gitlab.Ptr("env_var"),
			})
		}
		createOpts.Variables = &vars
	}

	p, _, err := c.gl.Pipelines.CreatePipeline(pid, createOpts)
	if err != nil {
		return nil, err
	}

	detail := &models.PipelineDetail{
		ID:     p.ID,
		IID:    p.IID,
		Status: p.Status,
		Source: p.Source,
		Ref:    p.Ref,
		SHA:    p.SHA,
		WebURL: p.WebURL,
	}
	if p.User != nil {
		detail.User = p.User.Username
	}
	if p.CreatedAt != nil {
		detail.CreatedAt = *p.CreatedAt
	}

	return detail, nil
}

// GetJobLogs fetches the log output for a specific job
func (c *Client) GetJobLogs(projectID any, jobID int) (string, error) {
	pid, err := c.resolveProjectID(projectID)
	if err != nil {
		return "", err
	}

	trace, _, err := c.gl.Jobs.GetTraceFile(pid, jobID)
	if err != nil {
		return "", err
	}

	// Read all the trace data
	var logs strings.Builder
	_, err = io.Copy(&logs, trace)
	if err != nil {
		return "", err
	}

	return logs.String(), nil
}

// ParseVariables parses KEY=VALUE strings into a map
func ParseVariables(vars []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid variable format %q, expected KEY=VALUE", v)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}
