package k8s

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Client wraps the kubernetes clientset
type Client struct {
	clientset *kubernetes.Clientset
	namespace string
}

// ContextInfo holds information about a kubeconfig context
type ContextInfo struct {
	Name      string
	Cluster   string
	Namespace string
	User      string
	Current   bool
}

// NewClient creates a new k8s client using the default kubeconfig
func NewClient(namespace string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}

	if namespace != "" {
		configOverrides.Context.Namespace = namespace
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Get effective namespace
	ns := namespace
	if ns == "" {
		ns, _, _ = kubeConfig.Namespace()
		if ns == "" {
			ns = "default"
		}
	}

	return &Client{
		clientset: clientset,
		namespace: ns,
	}, nil
}

// ListContexts returns all contexts from kubeconfig
func ListContexts() ([]ContextInfo, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, &clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	return contextsFromConfig(&config), nil
}

func contextsFromConfig(config *api.Config) []ContextInfo {
	var contexts []ContextInfo
	for name, ctx := range config.Contexts {
		contexts = append(contexts, ContextInfo{
			Name:      name,
			Cluster:   ctx.Cluster,
			Namespace: ctx.Namespace,
			User:      ctx.AuthInfo,
			Current:   name == config.CurrentContext,
		})
	}
	return contexts
}

// ListNamespaces returns all namespaces in the cluster
func (c *Client) ListNamespaces(ctx context.Context) ([]corev1.Namespace, error) {
	list, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}
	return list.Items, nil
}

// ListPods returns pods in the specified namespace (or all namespaces if allNamespaces is true)
func (c *Client) ListPods(ctx context.Context, allNamespaces bool) ([]corev1.Pod, error) {
	ns := c.namespace
	if allNamespaces {
		ns = ""
	}

	list, err := c.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	return list.Items, nil
}

// ListServices returns services in the specified namespace (or all namespaces if allNamespaces is true)
func (c *Client) ListServices(ctx context.Context, allNamespaces bool) ([]corev1.Service, error) {
	ns := c.namespace
	if allNamespaces {
		ns = ""
	}

	list, err := c.clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	return list.Items, nil
}

// Namespace returns the effective namespace for this client
func (c *Client) Namespace() string {
	return c.namespace
}

// GetPod returns a single pod by name
func (c *Client) GetPod(ctx context.Context, name string) (*corev1.Pod, error) {
	pod, err := c.clientset.CoreV1().Pods(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s: %w", name, err)
	}
	return pod, nil
}

// GetService returns a single service by name
func (c *Client) GetService(ctx context.Context, name string) (*corev1.Service, error) {
	svc, err := c.clientset.CoreV1().Services(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s: %w", name, err)
	}
	return svc, nil
}

// PodLogsOptions configures the log stream
type PodLogsOptions struct {
	Container string
	Follow    bool
	TailLines int64
	Previous  bool
}

// GetPodLogs returns a stream of logs from a pod
func (c *Client) GetPodLogs(ctx context.Context, name string, opts PodLogsOptions) (io.ReadCloser, error) {
	podLogOpts := &corev1.PodLogOptions{
		Container: opts.Container,
		Follow:    opts.Follow,
		Previous:  opts.Previous,
	}

	if opts.TailLines > 0 {
		podLogOpts.TailLines = &opts.TailLines
	}

	req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(name, podLogOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs for pod %s: %w", name, err)
	}

	return stream, nil
}
