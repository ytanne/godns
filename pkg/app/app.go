package app

import (
	"context"
	"log"

	"github.com/ytanne/godns/pkg/config"
	dnsServer "github.com/ytanne/godns/pkg/dns-server"
	httpServer "github.com/ytanne/godns/pkg/http-server"
	repo "github.com/ytanne/godns/pkg/repo/leveldb"
	"golang.org/x/sync/errgroup"
)

type Server interface {
	ListenAndServe() error
	Shutdown() error
}

type app struct {
	config    config.Config
	dnsServer Server
	webServer Server
}

func NewApp(config config.Config) app {
	return app{
		config: config,
	}
}

func (a *app) Run(ctx context.Context) error {
	db, err := repo.NewLevelDB(a.config.DbPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Println("could not close database", err)
		}
	}()

	a.dnsServer = dnsServer.NewDnsServer(a.config.DnsPort, db)
	a.webServer = httpServer.NewHttpServer(a.config.WebConfig, db)

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Printf("Starting dns server at :%s\n", a.config.DnsPort)

		err := a.dnsServer.ListenAndServe()
		if err != nil {
			log.Printf("Failed to start server: %s", err)
		}

		return err
	})

	g.Go(func() error {
		log.Printf("Starting web server at :%s\n", a.config.WebConfig.HttpPort)

		err := a.webServer.ListenAndServe()
		if err != nil {
			log.Printf("Failed to start http server: %s", err)
		}

		return err
	})

	err = g.Wait()
	if err != nil {
		return err
	}

	return nil
}

func (a *app) Close() {
	a.webServer.Shutdown()
	a.dnsServer.Shutdown()
}
