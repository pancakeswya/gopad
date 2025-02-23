package main

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pancakeswya/gopad/pkg/server"
	"github.com/rs/zerolog/log"
)

func main() {
	config := server.NewDefaultConfig()
	defer config.Cleanup()

	state := server.NewState(config.Database)
	defer state.Close()

	state.RunCleaner(config.ExpiryDays)

	router := mux.NewRouter()

	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/socket/{id}", state.HandleSocket)
	api.HandleFunc("/text/{id}", state.HandleText)
	api.HandleFunc("/stats", state.HandleStats)

	router.PathPrefix("/").Handler(http.FileServer(http.Dir("dist")))

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Error().Err(err).Msg("failed to listen and serve")
	}
}
