package cmd

import (
	"log"
	"net/http"
)

type APIConfig struct {
	addr string
}

type API struct {
	config APIConfig
	mux    *http.ServeMux
}

func NewAPI(config APIConfig) API {
	return API{
		config: config,
		mux:    http.NewServeMux(),
	}
}

func (a *API) Mount() {

}

func (a *API) Run() {
	err := http.ListenAndServe(a.config.addr, a.mux)
	if err != nil {
	    log.Fatalf("Server failed to start: %v", err)
	}

}
