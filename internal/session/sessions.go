package session

import (
	"fmt"

	"github.com/Soeky/pomo/internal/db"
)

func StopSession() {
	err := db.StopCurrentSession()
	if err != nil {
		fmt.Println("❌ Fehler beim Stoppen:", err)
		return
	}
	fmt.Println("🛑 Session gestoppt")
}

func StopIfRunning() {
	err := db.StopCurrentSession()
	if err == nil {
		fmt.Println("vorherige Session gestoppt")
	}
}
