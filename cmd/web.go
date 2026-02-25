package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	rt "github.com/Soeky/pomo/internal/runtime"
	"github.com/Soeky/pomo/internal/web"
	"github.com/spf13/cobra"
)

type webState struct {
	PID     int       `json:"pid"`
	Host    string    `json:"host"`
	Port    int       `json:"port"`
	Started time.Time `json:"started"`
}

var (
	webHost   string
	webPort   int
	webDaemon bool
	webOpen   bool
)

const browserHost = "pomo"

var webCmd = &cobra.Command{
	Use:     "web",
	Aliases: []string{"webserver"},
	Short:   "manage the pomo web server",
}

var webStartCmd = &cobra.Command{
	Use:   "start",
	Short: "start pomo web server",
	Run: func(cmd *cobra.Command, args []string) {
		if err := ensureHostsMapping(); err != nil {
			fmt.Printf("⚠️ %v\n", err)
			fmt.Println("ℹ️ continuing; fallback URL remains http://127.0.0.1:<port>")
		}

		if webDaemon {
			if st, ok := readState(); ok {
				if processAlive(st.PID) {
					url := fmt.Sprintf("http://%s:%d", browserHost, st.Port)
					fmt.Printf("ℹ️ web daemon already running at %s (pid %d)\n", url, st.PID)
					maybeOpenBrowser(url)
					return
				}
			}
		}

		port, err := web.FindAvailablePort(webPort)
		if err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}
		url := fmt.Sprintf("http://%s:%d", browserHost, port)

		if webDaemon {
			if err := startDaemon(webHost, port); err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}
			fmt.Printf("✅ web daemon started at %s (localhost fallback: http://127.0.0.1:%d)\n", url, port)
			maybeOpenBrowser(url)
			return
		}

		fmt.Printf("🚀 web server at %s (localhost fallback: http://127.0.0.1:%d)\n", url, port)
		maybeOpenBrowser(url)
		if err := web.RunWithSignals(web.ServerConfig{Host: webHost, Port: port}); err != nil {
			fmt.Println("❌ web server failed:", err)
			os.Exit(1)
		}
	},
}

var webServeCmd = &cobra.Command{
	Use:    "serve",
	Short:  "internal daemon entrypoint",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		if err := web.RunWithSignals(web.ServerConfig{Host: webHost, Port: webPort}); err != nil {
			fmt.Println("❌ web daemon failed:", err)
			os.Exit(1)
		}
	},
}

var webStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop web daemon",
	Run: func(cmd *cobra.Command, args []string) {
		st, ok := readState()
		if !ok {
			fmt.Println("ℹ️ no running daemon found")
			return
		}
		if !processAlive(st.PID) {
			_ = os.Remove(rt.PIDFilePath())
			_ = os.Remove(rt.StateFilePath())
			fmt.Println("ℹ️ stale daemon state removed")
			return
		}

		proc, err := os.FindProcess(st.PID)
		if err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if !processAlive(st.PID) {
				_ = os.Remove(rt.PIDFilePath())
				_ = os.Remove(rt.StateFilePath())
				fmt.Println("✅ web daemon stopped")
				return
			}
			time.Sleep(200 * time.Millisecond)
		}

		_ = proc.Signal(syscall.SIGKILL)
		_ = os.Remove(rt.PIDFilePath())
		_ = os.Remove(rt.StateFilePath())
		fmt.Println("⚠️ web daemon force-killed")
	},
}

var webStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "show daemon status",
	Run: func(cmd *cobra.Command, args []string) {
		st, ok := readState()
		if !ok || !processAlive(st.PID) {
			fmt.Println("stopped")
			return
		}
		base := fmt.Sprintf("http://%s:%d", st.Host, st.Port)
		health := "unhealthy"
		if err := web.HealthCheck(base); err == nil {
			health = "healthy"
		}
		fmt.Printf("running pid=%d url=http://%s:%d fallback=http://127.0.0.1:%d health=%s\n", st.PID, browserHost, st.Port, st.Port, health)
	},
}

var webLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "show recent daemon logs",
	Run: func(cmd *cobra.Command, args []string) {
		b, err := os.ReadFile(rt.LogFilePath())
		if err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}
		lines := strings.Split(string(b), "\n")
		if len(lines) > 200 {
			lines = lines[len(lines)-200:]
		}
		fmt.Println(strings.Join(lines, "\n"))
	},
}

var webHostsCheckCmd = &cobra.Command{
	Use:   "hosts-check",
	Short: "check if /etc/hosts contains pomo.local",
	Run: func(cmd *cobra.Command, args []string) {
		if err := ensureHostsMapping(); err != nil {
			fmt.Println("⚠️", err)
			fmt.Printf("✅ fallback host is always available: %s\n", browserHost)
			return
		}
		fmt.Println("✅ hosts mapping for pomo.local is present")
	},
}

func init() {
	webStartCmd.Flags().StringVar(&webHost, "host", "127.0.0.1", "bind host")
	webStartCmd.Flags().IntVar(&webPort, "port", 3210, "port")
	webStartCmd.Flags().BoolVar(&webDaemon, "daemon", true, "run in background")
	webStartCmd.Flags().BoolVar(&webOpen, "open", true, "open browser")

	webServeCmd.Flags().StringVar(&webHost, "host", "127.0.0.1", "bind host")
	webServeCmd.Flags().IntVar(&webPort, "port", 3210, "port")

	webCmd.AddCommand(webStartCmd)
	webCmd.AddCommand(webStopCmd)
	webCmd.AddCommand(webStatusCmd)
	webCmd.AddCommand(webLogsCmd)
	webCmd.AddCommand(webHostsCheckCmd)
	webCmd.AddCommand(webServeCmd)
	rootCmd.AddCommand(webCmd)
}

func startDaemon(host string, port int) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	logFile, err := os.OpenFile(rt.LogFilePath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	child := exec.Command(exe, "web", "serve", "--host", host, "--port", strconv.Itoa(port))
	child.Stdout = logFile
	child.Stderr = logFile
	child.Stdin = nil
	if err := child.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = logFile.Close()

	st := webState{PID: child.Process.Pid, Host: host, Port: port, Started: time.Now()}
	if err := writeState(st); err != nil {
		return err
	}
	if err := os.WriteFile(rt.PIDFilePath(), []byte(strconv.Itoa(st.PID)), 0644); err != nil {
		return err
	}

	healthURL := fmt.Sprintf("http://%s:%d", host, port)
	if !web.WaitForHealthy(healthURL, 5*time.Second) {
		return errors.New("daemon started but health endpoint did not become ready")
	}

	return nil
}

func ensureHostsMapping() error {
	ok, err := web.HasHostMapping(browserHost)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("missing /etc/hosts entry for %s; add: 127.0.0.1 %s", browserHost, browserHost)
	}
	return nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func writeState(st webState) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rt.StateFilePath(), b, 0644)
}

func readState() (webState, bool) {
	b, err := os.ReadFile(rt.StateFilePath())
	if err != nil {
		return webState{}, false
	}
	var st webState
	if err := json.Unmarshal(b, &st); err != nil {
		return webState{}, false
	}
	return st, true
}

func openBrowser(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u.String())
	case "linux":
		cmd = exec.Command("xdg-open", u.String())
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u.String())
	default:
		return fmt.Errorf("unsupported platform for auto-open")
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Start()
}

func maybeOpenBrowser(url string) {
	if !webOpen {
		return
	}
	if err := openBrowser(url); err != nil {
		fmt.Printf("⚠️ could not open browser automatically: %v\n", err)
	}
}
