package main

import (
	"net/http"

	"github.com/chelmertz/partille-goodreads/internal/server"
)

func main() {
	mux := server.NewMux()
	http.ListenAndServe(":8080", mux)
}
