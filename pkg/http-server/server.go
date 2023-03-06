package server

import (
	"net/http"
)

//nolint:gochecknoglobals
var secretKey string

func SetupSecretKey(key string) {
	secretKey = key
}

func RunServer(port string) error {
	server := http.Server{
		Addr:    ":" + port,
		Handler: middleware(http.HandlerFunc(handleA)),
	}

	return server.ListenAndServe()
}

func handleA(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("not implemented yet"))
}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("secret-key")

		if key == "" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("no secret key provided"))

			return
		}

		if key != secretKey {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("invalid secret key provided"))

			return
		}

		next.ServeHTTP(w, r)
	})
}
