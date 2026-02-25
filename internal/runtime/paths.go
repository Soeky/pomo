package runtime

import (
	"os"
	"path/filepath"
)

func DataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	dir := filepath.Join(homeDir, ".local", "share", "pomo")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func PIDFilePath() string {
	return filepath.Join(DataDir(), "web.pid")
}

func StateFilePath() string {
	return filepath.Join(DataDir(), "web.state.json")
}

func LogFilePath() string {
	return filepath.Join(DataDir(), "web.log")
}
