package web

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func RunWithSignals(cfg ServerConfig) error {
	srv, err := NewServer(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return srv.Run(ctx)
}

func FindAvailablePort(preferred int) (int, error) {
	if preferred > 0 {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferred))
		if err == nil {
			_ = ln.Close()
			return preferred, nil
		}
	}

	start := 3210
	if preferred >= 3210 {
		start = preferred + 1
	}
	for p := start; p <= 3299; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			_ = ln.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port found in range 3210-3299")
}

func HasHostMapping(hostname string) (bool, error) {
	f, err := os.Open("/etc/hosts")
	if err != nil {
		return false, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "127.0.0.1" {
			for _, f := range fields[1:] {
				if f == hostname {
					return true, nil
				}
			}
		}
	}
	if err := s.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func WaitForHealthy(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := HealthCheck(url); err == nil {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
