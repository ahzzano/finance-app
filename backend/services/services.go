package services

import "net/http"

type Handler interface {
    Mount(*http.ServeMux)
}
