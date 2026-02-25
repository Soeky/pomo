package main

import (
	"log"

	"github.com/Soeky/pomo/cmd"
	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
)

func main() {
	config.LoadConfig()
	if err := db.InitDB(); err != nil {
		log.Fatal(err)
	}
	cmd.Execute()
}
