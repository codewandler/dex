package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"dev-activity/internal/k8s"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
)

var (
	k8sHeaderColor  = color.New(color.FgCyan, color.Bold)
	k8sNameColor    = color.New(color.FgYellow)
	k8sCurrentColor = color.New(color.FgGreen, color.Bold)
	k8sDimColor     = color.New(color.FgHiBlack)
	k8sStatusColor  = color.New(color.FgGreen)
	k8sErrorColor   = color.New(color.FgRed)
)

var k8sCmd = &cobra.Command{
	Use:     "k8s",
	Aliases: []string{"kube", "kubernetes"},
	Short:   "Kubernetes cluster management",
	Long:    `Commands for interacting with Kubernetes clusters.`,
}

// Context commands
var k8sCtxCmd = &cobra.Command{
	Use:   "ctx",
	Short: "Manage kubeconfig contexts",
}

var k8sCtxLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List available contexts",
	Long: `List all contexts defined in kubeconfig.

Examples:
  dex k8s ctx ls`,
	Run: func(cmd *cobra.Command, args []string) {
		contexts, err := k8s.ListContexts()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(contexts) == 0 {
			k8sDimColor.Println("No contexts found in kubeconfig.")
			return
		}

		line := strings.Repeat("─", 80)
		fmt.Println()
		k8sHeaderColor.Printf("  Kubernetes Contexts (%d)\n", len(contexts))
		fmt.Println("  " + line)
		fmt.Println()

		fmt.Printf("  %-2s %-30s %-25s %s\n", "", "CONTEXT", "CLUSTER", "NAMESPACE")
		fmt.Printf("  %s\n", strings.Repeat("─", 76))

		for _, ctx := range contexts {
			marker := "  "
			nameColor := k8sNameColor
			if ctx.Current {
				marker = "* "
				nameColor = k8sCurrentColor
			}

			ns := ctx.Namespace
			if ns == "" {
				ns = "default"
			}

			fmt.Printf("  %s", marker)
			nameColor.Printf("%-30s ", truncateK8s(ctx.Name, 30))
			fmt.Printf("%-25s ", truncateK8s(ctx.Cluster, 25))
			k8sDimColor.Printf("%s\n", ns)
		}
		fmt.Println()
	},
}

// Namespace commands
var k8sNsCmd = &cobra.Command{
	Use:     "ns",
	Aliases: []string{"namespace", "namespaces"},
	Short:   "Manage namespaces",
}

var k8sNsLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List namespaces",
	Long: `List all namespaces in the current cluster.

Examples:
  dex k8s ns ls`,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := k8s.NewClient("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		namespaces, err := client.ListNamespaces(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(namespaces) == 0 {
			k8sDimColor.Println("No namespaces found.")
			return
		}

		line := strings.Repeat("─", 80)
		fmt.Println()
		k8sHeaderColor.Printf("  Namespaces (%d)\n", len(namespaces))
		fmt.Println("  " + line)
		fmt.Println()

		fmt.Printf("  %-30s %-12s %s\n", "NAME", "STATUS", "AGE")
		fmt.Printf("  %s\n", strings.Repeat("─", 60))

		for _, ns := range namespaces {
			status := string(ns.Status.Phase)
			statusColor := k8sStatusColor
			if status != "Active" {
				statusColor = k8sErrorColor
			}

			age := formatAge(ns.CreationTimestamp.Time)

			k8sNameColor.Printf("  %-30s ", truncateK8s(ns.Name, 30))
			statusColor.Printf("%-12s ", status)
			k8sDimColor.Printf("%s\n", age)
		}
		fmt.Println()
	},
}

// Pod commands
var k8sPodCmd = &cobra.Command{
	Use:     "pod",
	Aliases: []string{"pods"},
	Short:   "Manage pods",
}

var k8sPodLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List pods",
	Long: `List pods in the current namespace or all namespaces.

Examples:
  dex k8s pod ls              # Current namespace
  dex k8s pod ls -A           # All namespaces
  dex k8s pod ls -n kube-system`,
	Run: func(cmd *cobra.Command, args []string) {
		namespace, _ := cmd.Flags().GetString("namespace")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")

		client, err := k8s.NewClient(namespace)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		pods, err := client.ListPods(ctx, allNamespaces)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(pods) == 0 {
			k8sDimColor.Println("No pods found.")
			return
		}

		line := strings.Repeat("─", 100)
		fmt.Println()
		if allNamespaces {
			k8sHeaderColor.Printf("  Pods - All Namespaces (%d)\n", len(pods))
		} else {
			k8sHeaderColor.Printf("  Pods - %s (%d)\n", client.Namespace(), len(pods))
		}
		fmt.Println("  " + line)
		fmt.Println()

		if allNamespaces {
			fmt.Printf("  %-20s %-35s %-10s %-8s %s\n", "NAMESPACE", "NAME", "STATUS", "RESTARTS", "AGE")
			fmt.Printf("  %s\n", strings.Repeat("─", 90))
		} else {
			fmt.Printf("  %-40s %-12s %-8s %s\n", "NAME", "STATUS", "RESTARTS", "AGE")
			fmt.Printf("  %s\n", strings.Repeat("─", 75))
		}

		for _, pod := range pods {
			status := string(pod.Status.Phase)
			statusColor := getPodStatusColor(status)

			var restarts int32
			for _, cs := range pod.Status.ContainerStatuses {
				restarts += cs.RestartCount
			}

			age := formatAge(pod.CreationTimestamp.Time)

			if allNamespaces {
				k8sDimColor.Printf("  %-20s ", truncateK8s(pod.Namespace, 20))
				k8sNameColor.Printf("%-35s ", truncateK8s(pod.Name, 35))
			} else {
				k8sNameColor.Printf("  %-40s ", truncateK8s(pod.Name, 40))
			}
			statusColor.Printf("%-10s ", status)
			fmt.Printf("%-8d ", restarts)
			k8sDimColor.Printf("%s\n", age)
		}
		fmt.Println()
	},
}

// Service commands
var k8sSvcCmd = &cobra.Command{
	Use:     "svc",
	Aliases: []string{"service", "services"},
	Short:   "Manage services",
}

var k8sSvcLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List services",
	Long: `List services in the current namespace or all namespaces.

Examples:
  dex k8s svc ls              # Current namespace
  dex k8s svc ls -A           # All namespaces
  dex k8s svc ls -n kube-system`,
	Run: func(cmd *cobra.Command, args []string) {
		namespace, _ := cmd.Flags().GetString("namespace")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")

		client, err := k8s.NewClient(namespace)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		services, err := client.ListServices(ctx, allNamespaces)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(services) == 0 {
			k8sDimColor.Println("No services found.")
			return
		}

		line := strings.Repeat("─", 100)
		fmt.Println()
		if allNamespaces {
			k8sHeaderColor.Printf("  Services - All Namespaces (%d)\n", len(services))
		} else {
			k8sHeaderColor.Printf("  Services - %s (%d)\n", client.Namespace(), len(services))
		}
		fmt.Println("  " + line)
		fmt.Println()

		if allNamespaces {
			fmt.Printf("  %-20s %-30s %-12s %-20s %s\n", "NAMESPACE", "NAME", "TYPE", "CLUSTER-IP", "PORTS")
			fmt.Printf("  %s\n", strings.Repeat("─", 95))
		} else {
			fmt.Printf("  %-35s %-12s %-20s %s\n", "NAME", "TYPE", "CLUSTER-IP", "PORTS")
			fmt.Printf("  %s\n", strings.Repeat("─", 85))
		}

		for _, svc := range services {
			svcType := string(svc.Spec.Type)
			clusterIP := svc.Spec.ClusterIP
			if clusterIP == "" {
				clusterIP = "None"
			}

			ports := formatPorts(svc.Spec.Ports)

			if allNamespaces {
				k8sDimColor.Printf("  %-20s ", truncateK8s(svc.Namespace, 20))
				k8sNameColor.Printf("%-30s ", truncateK8s(svc.Name, 30))
			} else {
				k8sNameColor.Printf("  %-35s ", truncateK8s(svc.Name, 35))
			}
			fmt.Printf("%-12s %-20s ", svcType, clusterIP)
			k8sDimColor.Printf("%s\n", ports)
		}
		fmt.Println()
	},
}

func formatPorts(ports []corev1.ServicePort) string {
	if len(ports) == 0 {
		return "<none>"
	}

	var parts []string
	for _, p := range ports {
		if p.NodePort > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
	}
	return strings.Join(parts, ",")
}

func getPodStatusColor(status string) *color.Color {
	switch status {
	case "Running":
		return k8sStatusColor
	case "Pending":
		return color.New(color.FgYellow)
	case "Succeeded":
		return color.New(color.FgCyan)
	case "Failed":
		return k8sErrorColor
	default:
		return k8sDimColor
	}
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func truncateK8s(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	// Add k8s command to root
	rootCmd.AddCommand(k8sCmd)

	// Context commands
	k8sCmd.AddCommand(k8sCtxCmd)
	k8sCtxCmd.AddCommand(k8sCtxLsCmd)

	// Namespace commands
	k8sCmd.AddCommand(k8sNsCmd)
	k8sNsCmd.AddCommand(k8sNsLsCmd)

	// Pod commands
	k8sCmd.AddCommand(k8sPodCmd)
	k8sPodCmd.AddCommand(k8sPodLsCmd)
	k8sPodLsCmd.Flags().StringP("namespace", "n", "", "Namespace to list pods from")
	k8sPodLsCmd.Flags().BoolP("all-namespaces", "A", false, "List pods from all namespaces")

	// Service commands
	k8sCmd.AddCommand(k8sSvcCmd)
	k8sSvcCmd.AddCommand(k8sSvcLsCmd)
	k8sSvcLsCmd.Flags().StringP("namespace", "n", "", "Namespace to list services from")
	k8sSvcLsCmd.Flags().BoolP("all-namespaces", "A", false, "List services from all namespaces")
}
