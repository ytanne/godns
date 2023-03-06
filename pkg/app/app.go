package app

import (
	"context"
	"log"

	"github.com/miekg/dns"
	"github.com/ytanne/godns/pkg/config"
	dnsServer "github.com/ytanne/godns/pkg/dns-server"
	httpServer "github.com/ytanne/godns/pkg/http-server"
	"golang.org/x/sync/errgroup"
)

type Server interface {
	ListenAndServe() error
	Shutdown() error
}

type app struct {
	config config.Config
	server Server
}

func NewApp(config config.Config) app {
	return app{
		config: config,
	}
}

func (a *app) Run(ctx context.Context) error {
	c := dnsServer.NewDnsServer()

	server := &dns.Server{
		Addr:    ":" + a.config.DnsPort,
		Net:     "udp",
		Handler: c,
	}

	a.server = server

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Printf("Starting dns server at :%s\n", a.config.DnsPort)

		err := server.ListenAndServe()
		if err != nil {
			log.Printf("Failed to start server: %s", err)
		}

		return err
	})

	g.Go(func() error {
		log.Printf("Starting http server at :%s\n", a.config.HttpPort)

		httpServer.SetupSecretKey(a.config.SecretKey)

		err := httpServer.RunServer(a.config.HttpPort)
		if err != nil {
			log.Printf("Failed to start http server: %s", err)
		}

		return err
	})

	err := g.Wait()
	if err != nil {
		return err
	}

	return nil
}

func (a *app) Close() {
	a.server.Shutdown()
}
