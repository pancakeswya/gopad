package main

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/pancakeswya/gopad/pkg/gopad"
	"github.com/pancakeswya/gopad/pkg/database/sqlite"
)

func main() {
	database, err := sqlite.NewDatabase()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create database")
	}
	defer database.Close()

	state := gopad.NewState(database)
	defer state.Close()

	state.RunCleaner(1)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/socket/{id}", state.SocketHandle)
	mux.HandleFunc("/api/text/{id}", state.TextHandle)
	mux.HandleFunc("/api/stats", state.StatsHandle)
	mux.Handle("/", http.FileServer(http.Dir("dist")))

	if err = http.ListenAndServe(":8080", mux); err != nil {
		log.Error().Err(err).Msg("failed to listen and serve")
	}
}
