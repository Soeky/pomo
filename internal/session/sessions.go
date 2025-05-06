package session

import (
	"fmt"

	"github.com/Soeky/pomo/internal/db"
)

func StopSession() {
	err := db.StopCurrentSession()
	if err != nil {
		fmt.Println("❌ error while stopping:", err)
		return
	}
	fmt.Println("🛑 session has been stopped")
}

func StopIfRunning() {
	err := db.StopCurrentSession()
	if err == nil {
		fmt.Println("previous session has been stopped")
	}
}
