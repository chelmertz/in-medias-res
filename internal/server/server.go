package server

import "net/http"

func index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
