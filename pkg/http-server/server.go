package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	_ "embed"

	"github.com/ytanne/godns/pkg/config"
	"github.com/ytanne/godns/pkg/models"
	"go.uber.org/zap"
)

type keyDB interface {
	GetAll() ([]models.Record, error)
	//	Block(key string) error
	Remove(key string) error
}

type httpServer struct {
	username string
	password string
	db       keyDB
	server   *http.Server
	log      *zap.Logger
}

func WithLogger(logger *zap.Logger) func(h *httpServer) {
	return func(h *httpServer) {
		h.log = logger
	}
}

func NewHttpServer(cfg config.WebServerConfig, db keyDB, sets ...func(*httpServer)) *httpServer {
	h := &httpServer{
		username: cfg.Username,
		password: cfg.Password,
		db:       db,
	}

	for _, set := range sets {
		set(h)
	}

	mux := http.NewServeMux()

	mux.Handle("/api/", http.StripPrefix("/api", h.middleware(http.HandlerFunc(h.handleAPI))))
	mux.Handle("/", h.middleware(http.HandlerFunc(h.handlePages)))

	h.server = &http.Server{
		Addr:    ":" + cfg.HttpPort,
		Handler: mux,
	}

	return h
}

func (h *httpServer) handlePages(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/index.html", "/":
		http.ServeFile(w, r, "./pkg/http-server/templates/index.html")
	default:
		h.log.Error("page requested not supported", zap.String("path", r.URL.Path))
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *httpServer) verifyUser(username, password string) bool {
	return h.username == username && h.password == password
}

func (h *httpServer) ListenAndServe() error {
	if err := h.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (h *httpServer) Shutdown() error {
	return h.server.Close()
}

func (h *httpServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/cache":
		h.handleQueries(w, r)

		return
	}

	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("not found"))
}

func (h *httpServer) handleQueries(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.GetQueries(w, r)

		return
	case http.MethodDelete:
		h.DeleteQuery(w, r)

		return
	}

	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *httpServer) DeleteQuery(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("domain")
	if key == "" {
		http.Error(w, "no domain provided", http.StatusBadRequest)

		return
	}

	h.log.Debug("domain to delete", zap.String("key", key))

	if err := h.db.Remove(key); err != nil {
		h.log.Error("could not delete domain:", zap.Error(err))
		http.Error(w, "could not delete domain", http.StatusBadRequest)

		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(key + " is deleted"))
}

func (h *httpServer) GetQueries(w http.ResponseWriter, r *http.Request) {
	records, err := h.db.GetAll()
	if err != nil {
		h.log.Error("failed to get records from keyDB:", zap.Error(err))
		http.Error(w, "could not get DNS records", http.StatusInternalServerError)

		return
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		h.log.Error("failed to marshal records:", zap.Error(err))
		http.Error(w, "could not get DNS records", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (h *httpServer) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the Authorization header from the request
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// If the header is missing, return a 401 Unauthorized error
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Please enter your username and password\"")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		if !h.validAuthHeader(w, authHeader) {
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *httpServer) validAuthHeader(w http.ResponseWriter, authHeader string) bool {
	const (
		requiredCredsLen = 2
	)

	// Extract the username and password from the Authorization header
	authParts := strings.SplitN(authHeader, " ", requiredCredsLen)
	if len(authParts) != requiredCredsLen || authParts[0] != "Basic" {
		// If the header is malformed, return a 400 Bad Request error
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return false
	}

	// Decode the base64-encoded username and password
	authBytes, err := base64.StdEncoding.DecodeString(authParts[requiredCredsLen-1])
	if err != nil {
		// If decoding fails, return a 400 Bad Request error
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return false
	}

	// Extract the username and password from the decoded bytes
	authString := string(authBytes)
	authParts = strings.SplitN(authString, ":", requiredCredsLen)

	if len(authParts) != requiredCredsLen {
		// If the username or password is missing, return a 400 Bad Request error
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return false
	}

	// Verify the username and password against a user database
	username := authParts[0]
	password := authParts[1]

	if !h.verifyUser(username, password) {
		// If the credentials are incorrect, return a 401 Unauthorized error
		w.Header().Set("WWW-Authenticate", "Basic realm=\"Please enter your username and password\"")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)

		return false
	}

	return true
}
