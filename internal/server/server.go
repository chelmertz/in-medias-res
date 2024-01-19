package server

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/chelmertz/partille-goodreads/internal/partille"
)

//go:embed index.html
var templates embed.FS

func index() http.Handler {
	tmpl := template.Must(template.ParseFS(templates, "index.html"))
	type view struct {
		Books []partille.Book
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data view
		data.Books = []partille.Book{
			{
				Title: "The Hobbit",
			},
		}
		tmpl.Execute(w, data)
	})
}

func users() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})
}

func availabilitiesPartille(storage *partille.Storage) http.Handler {
	poller := partille.PollPartilleBibliotek
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		err := storage.RefreshBookAvailabilities(poller)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
	})
}

func test(storage *partille.Storage, poller *partille.Poller) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res, err := partille.PollPartilleBibliotek(partille.BookQuery{
			Id:     1,
			Title:  "The Big Sleep",
			Author: "Raymond Chandler",
		})
		fmt.Printf("res: %+v\nerr: %+v\n", res, err)
	})
}

func NewMux(storage *partille.Storage, poller *partille.Poller) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", index())
	mux.Handle("/users", users())
	mux.Handle("/availabilities/library/partille", availabilitiesPartille(storage))
	mux.Handle("/test", test(storage, poller))

	return mux
}
