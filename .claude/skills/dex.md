# dex - Engineer's CLI Tool

Use `dex` for Kubernetes, GitLab, and Jira operations. Run commands via Bash tool.

## Kubernetes (`dex k8s`)

### Contexts
```bash
dex k8s ctx ls                    # List all kubeconfig contexts (* = current)
```

### Namespaces
```bash
dex k8s ns ls                     # List all namespaces
```

### Pods
```bash
dex k8s pod ls                    # List pods in current namespace
dex k8s pod ls -A                 # List pods in all namespaces
dex k8s pod ls -n kube-system     # List pods in specific namespace
dex k8s pod show <name>           # Show pod details (containers, conditions, labels)
dex k8s pod show <name> -n <ns>   # Show pod in specific namespace
```

### Pod Logs (stern-like)
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

### Services
```bash
dex k8s svc ls                    # List services in current namespace
dex k8s svc ls -A                 # List services in all namespaces
dex k8s svc ls -n kube-system     # List services in specific namespace
dex k8s svc show <name>           # Show service details (ports, selectors, ingress)
```

## GitLab (`dex gl` or `dex gitlab`)

### Activity
```bash
dex gl activity                   # Show activity from last 14 days
dex gl activity --since 7d        # Activity from last 7 days
dex gl activity --since 4h        # Activity from last 4 hours
```

### Project Index
```bash
dex gl index                      # Index all accessible projects (cached 24h)
dex gl index --force              # Force re-index
```

### Projects
```bash
dex gl proj ls                    # List projects (from index)
dex gl proj ls -n 50              # List 50 projects
dex gl proj ls --sort name        # Sort by name (also: created, activity, path)
dex gl proj ls --no-cache         # Fetch from API
dex gl proj show <id|path>        # Show project details
```

## Jira (`dex jira`)

### Authentication
```bash
dex jira auth                     # Authenticate via OAuth (opens browser)
```

### Issues
```bash
dex jira view <KEY>               # View single issue (e.g., TEL-117)
dex jira my                       # Show issues assigned to me
dex jira search "<JQL>"           # Search with JQL query
dex jira lookup KEY1 KEY2 KEY3    # Look up multiple issues
```

## Tips

- All k8s commands support shell completion for resource names
- Pod logs `-c` flag autocompletes container names
- GitLab project names autocomplete from local index
- Use `-n` for namespace, `-A` for all namespaces in k8s commands
