package portforward

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/codewandler/dex/internal/config"
)

// Info holds metadata about a running port-forward.
type Info struct {
	PID        int       `json:"pid"`
	Name       string    `json:"name"`
	Namespace  string    `json:"namespace"`
	Pod        string    `json:"pod"`
	LocalPort  int       `json:"local_port"`
	RemotePort int       `json:"remote_port"`
	StartedAt  time.Time `json:"started_at"`
}

func pidFilePath(name string) (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("portforward-%s.json", name)), nil
}

func loadInfo(name string) (*Info, error) {
	path, err := pidFilePath(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func saveInfo(info *Info) error {
	path, err := pidFilePath(info.Name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func removePIDFile(name string) error {
	path, err := pidFilePath(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func isAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// IsRunning checks if a named port-forward is active.
// Returns the info and true if running, or nil and false otherwise.
// Cleans up stale PID files.
func IsRunning(name string) (*Info, bool) {
	info, err := loadInfo(name)
	if err != nil {
		return nil, false
	}
	if !isAlive(info.PID) {
		removePIDFile(name)
		return nil, false
	}
	return info, true
}

// Start launches a detached kubectl port-forward process.
// If the same target is already running, it returns the existing info.
// If a different target is running under the same name, it stops the old one first.
func Start(name, namespace, pod string, localPort, remotePort int) (*Info, error) {
	if existing, running := IsRunning(name); running {
		if existing.Namespace == namespace && existing.Pod == pod &&
			existing.LocalPort == localPort && existing.RemotePort == remotePort {
			return existing, nil
		}
		// Different target — stop old one
		Stop(name)
	}

	args := []string{"port-forward", "-n", namespace, pod, fmt.Sprintf("%d:%d", localPort, remotePort)}
	cmd := exec.Command("kubectl", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting kubectl port-forward: %w", err)
	}

	info := &Info{
		PID:        cmd.Process.Pid,
		Name:       name,
		Namespace:  namespace,
		Pod:        pod,
		LocalPort:  localPort,
		RemotePort: remotePort,
		StartedAt:  time.Now(),
	}

	if err := saveInfo(info); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("saving PID file: %w", err)
	}

	// Wait for port to become available
	if err := waitForPort(localPort, 3*time.Second); err != nil {
		// Check if process died
		if !isAlive(cmd.Process.Pid) {
			removePIDFile(name)
			return nil, fmt.Errorf("kubectl port-forward exited immediately (is the pod running?)")
		}
		return nil, fmt.Errorf("port %d not ready after timeout: %w", localPort, err)
	}

	return info, nil
}

// Stop terminates a named port-forward and removes its PID file.
func Stop(name string) error {
	info, err := loadInfo(name)
	if err != nil {
		return fmt.Errorf("no port-forward named %q found", name)
	}

	if isAlive(info.PID) {
		proc, err := os.FindProcess(info.PID)
		if err == nil {
			proc.Signal(syscall.SIGTERM)
		}
	}

	return removePIDFile(name)
}

// List returns all tracked port-forwards, cleaning up stale entries.
func List() ([]*Info, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return nil, err
	}

	matches, err := filepath.Glob(filepath.Join(dir, "portforward-*.json"))
	if err != nil {
		return nil, err
	}

	var result []*Info
	for _, path := range matches {
		name := strings.TrimPrefix(filepath.Base(path), "portforward-")
		name = strings.TrimSuffix(name, ".json")

		info, running := IsRunning(name)
		if running {
			result = append(result, info)
		}
	}
	return result, nil
}

// FreePort finds a free TCP port starting from the preferred port.
// It tries the preferred port first, then increments until one is available.
func FreePort(preferred int) int {
	for port := preferred; port <= preferred+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	// Fallback: let the OS pick
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return preferred
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}
