package cmd

import (
	"log"
	"net/http"

	"backend/services/users"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

type APIConfig struct {
	Addr string
	DB   *pgxpool.Pool
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
	db := stdlib.OpenDBFromPool(a.config.DB)
	users.NewUserHandler(users.NewUserService(db)).Mount(a.mux)
}

func (a *API) Handler() http.Handler { return a.mux }

func (a *API) Run() {
	err := http.ListenAndServe(a.config.Addr, a.mux)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
