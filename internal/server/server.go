package server

import (
	"embed"
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

func NewMux(storage *partille.Storage) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", index())
	mux.Handle("/users", users())
	mux.Handle("/availabilities/library/partille", availabilitiesPartille(storage))

	return mux
}
