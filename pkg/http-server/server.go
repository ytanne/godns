package server

import (
	"net/http"
)

//nolint:gochecknoglobals
var secretKey string

func SetupSecretKey(key string) {
	secretKey = key
}

type httpServer struct {
	secretKey string
	port      string
	server    *http.Server
}

func NewHttpServer(key, port string) *httpServer {
	h := &httpServer{
		secretKey: key,
		port:      port,
	}

	h.server = &http.Server{
		Addr:    ":" + h.port,
		Handler: h.middleware(http.HandlerFunc(h.handleA)),
	}

	return h
}

func (h *httpServer) ListenAndServe() error {
	return h.server.ListenAndServe()
}

func (h *httpServer) Shutdown() error {
	return h.server.Close()
}

func (h *httpServer) handleA(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("not implemented yet"))
}

func (h *httpServer) middleware(next http.Handler) http.Handler {
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
