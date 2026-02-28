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

	"github.com/Soeky/pomo/internal/config"
	rt "github.com/Soeky/pomo/internal/runtime"
	"github.com/Soeky/pomo/internal/web"
	"github.com/spf13/cobra"
)

type webState struct {
	PID             int       `json:"pid"`
	Host            string    `json:"host"`
	Port            int       `json:"port"`
	Mode            string    `json:"mode"`
	AutoSleepSecond int       `json:"auto_sleep_seconds,omitempty"`
	Started         time.Time `json:"started"`
}

var (
	webHost      string
	webPort      int
	webDaemon    bool
	webMode      string
	webOpen      bool
	webServeMode string
	webAutoSleep time.Duration
)

const (
	browserHost     = "pomo"
	webModeDaemon   = "daemon"
	webModeOnDemand = "on_demand"
	daemonAutoSleep = 15 * time.Minute
)

var webCmd = &cobra.Command{
	Use:     "web",
	Aliases: []string{"webserver"},
	Short:   "manage the pomo web server",
	Long: `Manage the local web server with runtime modes.
Modes:
  daemon      background process with auto-sleep after inactivity
  on_demand   foreground server that warms health before browser open

Mode resolution order:
  1) --mode flag
  2) compatibility --daemon flag
  3) config key web_mode`,
}

var webStartCmd = &cobra.Command{
	Use:   "start",
	Short: "start pomo web server",
	Long: `Start the web server using configured or explicit runtime mode.
Set default mode via:
  pomo config set web_mode daemon
  pomo config set web_mode on_demand`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := ensureHostsMapping(); err != nil {
			fmt.Printf("⚠️ %v\n", err)
			fmt.Println("ℹ️ continuing; fallback URL remains http://127.0.0.1:<port>")
		}

		mode, err := resolveWebMode(cmd)
		if err != nil {
			fmt.Println("❌", err)
			os.Exit(1)
		}

		if mode == webModeDaemon {
			if st, ok := readState(); ok {
				if processAlive(st.PID) {
					url := fmt.Sprintf("http://%s:%d", browserHost, st.Port)
					existingMode := strings.TrimSpace(st.Mode)
					if existingMode == "" {
						existingMode = webModeDaemon
					}
					fmt.Printf("ℹ️ web daemon already running at %s (pid %d mode=%s)\n", url, st.PID, existingMode)
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
		healthURL := fmt.Sprintf("http://%s:%d", webHost, port)

		if mode == webModeDaemon {
			if err := startDaemon(webHost, port); err != nil {
				fmt.Println("❌", err)
				os.Exit(1)
			}
			fmt.Printf("✅ web daemon started at %s (localhost fallback: http://127.0.0.1:%d, auto-sleep=%s)\n", url, port, daemonAutoSleep)
			maybeOpenBrowser(url)
			return
		}

		if webOpen {
			go func(browserURL, healthURL string) {
				if web.WaitForHealthy(healthURL, 5*time.Second) {
					maybeOpenBrowser(browserURL)
					return
				}
				fmt.Printf("⚠️ on-demand warm health-check timed out for %s\n", healthURL)
			}(url, healthURL)
		}
		fmt.Printf("🚀 web server (on-demand) at %s (localhost fallback: http://127.0.0.1:%d)\n", url, port)
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
		defer func() {
			_ = os.Remove(rt.PIDFilePath())
			_ = os.Remove(rt.StateFilePath())
		}()

		cfg := web.ServerConfig{Host: webHost, Port: webPort}
		if webServeMode == webModeDaemon {
			cfg.AutoSleepAfter = webAutoSleep
		}
		if err := web.RunWithSignals(cfg); err != nil {
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
		mode := strings.TrimSpace(st.Mode)
		if mode == "" {
			mode = webModeDaemon
		}
		autoSleep := ""
		if st.AutoSleepSecond > 0 {
			autoSleep = fmt.Sprintf(" auto_sleep=%ds", st.AutoSleepSecond)
		}
		fmt.Printf("running pid=%d mode=%s%s url=http://%s:%d fallback=http://127.0.0.1:%d health=%s\n", st.PID, mode, autoSleep, browserHost, st.Port, st.Port, health)
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
	Short: "check if /etc/hosts contains pomo",
	Run: func(cmd *cobra.Command, args []string) {
		if err := ensureHostsMapping(); err != nil {
			fmt.Println("⚠️", err)
			fmt.Printf("✅ fallback host is always available: %s\n", browserHost)
			return
		}
		fmt.Printf("✅ hosts mapping for %s is present\n", browserHost)
	},
}

func init() {
	webStartCmd.Flags().StringVar(&webHost, "host", "127.0.0.1", "bind host")
	webStartCmd.Flags().IntVar(&webPort, "port", 3210, "port")
	webStartCmd.Flags().StringVar(&webMode, "mode", "", "runtime mode: daemon or on_demand (defaults to config web_mode)")
	webStartCmd.Flags().BoolVar(&webDaemon, "daemon", true, "compatibility override for runtime mode (true=daemon, false=on_demand)")
	webStartCmd.Flags().BoolVar(&webOpen, "open", true, "open browser")

	webServeCmd.Flags().StringVar(&webHost, "host", "127.0.0.1", "bind host")
	webServeCmd.Flags().IntVar(&webPort, "port", 3210, "port")
	webServeCmd.Flags().StringVar(&webServeMode, "mode", webModeDaemon, "internal runtime mode")
	webServeCmd.Flags().DurationVar(&webAutoSleep, "auto-sleep", daemonAutoSleep, "internal daemon auto-sleep duration")
	_ = webServeCmd.Flags().MarkHidden("mode")
	_ = webServeCmd.Flags().MarkHidden("auto-sleep")

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

	child := exec.Command(exe, "web", "serve", "--host", host, "--port", strconv.Itoa(port), "--mode", webModeDaemon, "--auto-sleep", daemonAutoSleep.String())
	child.Stdout = logFile
	child.Stderr = logFile
	child.Stdin = nil
	if err := child.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = logFile.Close()

	st := webState{
		PID:             child.Process.Pid,
		Host:            host,
		Port:            port,
		Mode:            webModeDaemon,
		AutoSleepSecond: int(daemonAutoSleep / time.Second),
		Started:         time.Now(),
	}
	if err := writeState(st); err != nil {
		_ = child.Process.Kill()
		return err
	}
	if err := os.WriteFile(rt.PIDFilePath(), []byte(strconv.Itoa(st.PID)), 0644); err != nil {
		_ = child.Process.Kill()
		_ = os.Remove(rt.StateFilePath())
		return err
	}

	healthURL := fmt.Sprintf("http://%s:%d", host, port)
	if !web.WaitForHealthy(healthURL, 5*time.Second) {
		_ = child.Process.Kill()
		_ = os.Remove(rt.PIDFilePath())
		_ = os.Remove(rt.StateFilePath())
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

func resolveWebMode(cmd *cobra.Command) (string, error) {
	daemonChanged := false
	if cmd != nil {
		daemonChanged = cmd.Flags().Changed("daemon")
	}
	return resolveWebModeValue(webMode, webDaemon, daemonChanged, config.AppConfig.WebMode)
}

func resolveWebModeValue(flagMode string, daemonFlag bool, daemonChanged bool, configMode string) (string, error) {
	if strings.TrimSpace(flagMode) != "" {
		mode, err := normalizeWebMode(flagMode)
		if err != nil {
			return "", err
		}
		return mode, nil
	}
	if daemonChanged {
		if daemonFlag {
			return webModeDaemon, nil
		}
		return webModeOnDemand, nil
	}
	if strings.TrimSpace(configMode) == "" {
		return webModeDaemon, nil
	}
	mode, err := normalizeWebMode(configMode)
	if err != nil {
		return "", err
	}
	return mode, nil
}

func normalizeWebMode(raw string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case webModeDaemon:
		return webModeDaemon, nil
	case webModeOnDemand:
		return webModeOnDemand, nil
	default:
		return "", fmt.Errorf("invalid web mode %q (expected daemon or on_demand)", raw)
	}
}
