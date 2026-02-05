package providers

import (
	"context"
	"fmt"

	"github.com/codewandler/dex/internal/config"
	"github.com/codewandler/dex/internal/k8s"
)

// K8sProvider fetches Kubernetes cluster status
type K8sProvider struct{}

func NewK8sProvider() *K8sProvider {
	return &K8sProvider{}
}

func (p *K8sProvider) Name() string {
	return "k8s"
}

func (p *K8sProvider) IsConfigured(_ *config.Config) bool {
	// K8s uses kubeconfig, always try to use it
	return true
}

func (p *K8sProvider) Fetch(ctx context.Context) (map[string]any, error) {
	data := map[string]any{
		"Context":   "",
		"Namespace": "",
		"Issues":    "",
	}

	// Get current context
	contexts, err := k8s.ListContexts()
	if err != nil {
		return nil, err
	}

	var currentContext, namespace string
	for _, c := range contexts {
		if c.Current {
			currentContext = c.Name
			namespace = c.Namespace
			if namespace == "" {
				namespace = "default"
			}
			break
		}
	}

	if currentContext == "" {
		return nil, fmt.Errorf("no current context")
	}

	data["Context"] = currentContext
	data["Namespace"] = namespace

	// Try to get pod status (may fail if not connected)
	client, err := k8s.NewClient(namespace)
	if err != nil {
		// Return context info even if we can't connect
		return data, nil
	}

	pods, err := client.ListPods(ctx, false)
	if err != nil {
		// Return context info even if we can't list pods
		return data, nil
	}

	// Count pod issues
	var pending, failed int
	for _, pod := range pods {
		switch pod.Status.Phase {
		case "Pending":
			pending++
		case "Failed":
			failed++
		}
	}

	if pending > 0 || failed > 0 {
		var issues []string
		if pending > 0 {
			issues = append(issues, fmt.Sprintf("%d pending", pending))
		}
		if failed > 0 {
			issues = append(issues, fmt.Sprintf("%d failed", failed))
		}
		data["Issues"] = fmt.Sprintf("%s", joinIssues(issues))
	}

	return data, nil
}

func joinIssues(issues []string) string {
	if len(issues) == 0 {
		return ""
	}
	if len(issues) == 1 {
		return issues[0]
	}
	return issues[0] + ", " + issues[1]
}
