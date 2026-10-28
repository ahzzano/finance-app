package cmd

import (
	"log"
	"net/http"

	"backend/internal/database"
	"backend/middleware"
	"backend/services/users"

	_ "backend/docs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	httpSwagger "github.com/swaggo/http-swagger/v2"
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
	a.mux.Handle("GET /swagger/", httpSwagger.WrapHandler)
	users.NewUserHandler(users.NewUserService(db), middleware.RequireSession(database.New(db))).Mount(a.mux)
}

func (a *API) Handler() http.Handler { return middleware.JSONErrors(a.mux) }

func (a *API) Run() {
	err := http.ListenAndServe(a.config.Addr, a.Handler())
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
