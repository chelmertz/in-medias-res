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
	type indexData struct {
		Books []partille.Book
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data indexData
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

func NewMux() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", index())
	mux.Handle("/users", users())

	return mux
}
