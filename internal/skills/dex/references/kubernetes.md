# Kubernetes Commands (`dex k8s`)

Aliases: `kube`, `kubernetes`

## Contexts
```bash
dex k8s ctx ls                    # List all kubeconfig contexts (* = current)
```

## Namespaces
```bash
dex k8s ns ls                     # List all namespaces
```

## Pods
```bash
dex k8s pod ls                    # List pods in current namespace
dex k8s pod ls -A                 # List pods in all namespaces
dex k8s pod ls -n kube-system     # List pods in specific namespace
dex k8s pod show <name>           # Show pod details (containers, conditions, labels)
dex k8s pod show <name> -n <ns>   # Show pod in specific namespace
```

## Pod Logs (stern-like)
```bash
dex k8s pod logs <name>           # Stream logs (all containers if multi-container)
dex k8s pod logs <name> -f        # Follow logs
dex k8s pod logs <name> --tail 100    # Last N lines
dex k8s pod logs <name> --since 1h    # Logs from last hour (supports: 30m, 1h, 1d)
dex k8s pod logs <name> -c <container>  # Specific container only
dex k8s pod logs <name> -i "error"    # Include only lines matching regex
dex k8s pod logs <name> -e "debug"    # Exclude lines matching regex
dex k8s pod logs <name> -p        # Previous container instance
```

## Services
```bash
dex k8s svc ls                    # List services in current namespace
dex k8s svc ls -A                 # List services in all namespaces
dex k8s svc ls -n kube-system     # List services in specific namespace
dex k8s svc show <name>           # Show service details (ports, selectors, ingress)
```

## Port-Forwarding
```bash
# Smart discovery — auto-detect pod, port, and namespace
dex k8s forward start loki                # Find pod/svc matching "loki", auto-detect everything
dex k8s forward start loki -n monitoring  # Limit discovery to a specific namespace

# Explicit mode — specify pod, port, and namespace
dex k8s forward start <pod> 3100 -n <ns>  # Forward local:3100 → pod:3100 (detached)
dex k8s forward start <pod> 8080:3100 -n <ns>  # Forward local:8080 → pod:3100
dex k8s forward start <pod> 3100 -n <ns> --name loki  # Label the forward

dex k8s forward ls                        # List active port-forwards
dex k8s forward status                    # Show all running forwards
dex k8s forward status <name>             # Show details for one forward
dex k8s forward stop <name>              # Stop a running forward
```

Aliases: `forward`, `fwd`, `portforward`, `pf`

Port-forwards run as detached background processes. PID files are stored in `~/.dex/`.
`start` is idempotent — reuses an existing forward if the target matches, restarts if different.

**Smart discovery:** When only a query is given (no port spec), searches pods across all namespaces
(or in `-n` namespace if specified), then falls back to services. Picks the first running match,
determines the remote port from container/service spec, and finds a free local port.

## Tips

- Use `-n` for namespace, `-A` for all namespaces
- All commands support shell completion for resource names
- Pod logs `-c` flag autocompletes container names
