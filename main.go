package main

import (
	"github.com/Soeky/pomo/cmd"
	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
)

func main() {
	config.LoadConfig()
	db.InitDB()
	cmd.Execute()
}
