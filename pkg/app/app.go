package app

import (
	"log"
	"strconv"

	"github.com/miekg/dns"
	server "github.com/ytanne/godns/pkg/dns-server"
)

type Server interface {
	ListenAndServe() error
	Shutdown() error
}

type app struct {
	server Server
}

func NewApp() app {
	return app{}
}

func (a *app) Run(port int) {
	c := server.NewCustomServer()
	// start server

	server := &dns.Server{
		Addr:    ":" + strconv.Itoa(port),
		Net:     "udp",
		Handler: c,
	}

	log.Printf("Starting at %d\n", port)

	a.server = server

	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start server: %s\n ", err.Error())
	}
}

func (a *app) Close() {
	a.server.Shutdown()
}
