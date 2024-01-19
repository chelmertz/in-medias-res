package main

import (
	"fmt"
	"net/http"

	"github.com/chelmertz/partille-goodreads/internal/partille"
	"github.com/chelmertz/partille-goodreads/internal/server"
)

func main() {
	storage, err := partille.NewStorage("in_medias_res.sqlite3")
	if err != nil {
		panic(err)
	}
	defer storage.Close()

	poller := partille.NewPoller()
	defer poller.Close()

	mux := server.NewMux(storage, poller)
	fmt.Println("Listening on http://localhost:8080")
	http.ListenAndServe(":8080", mux)
}
