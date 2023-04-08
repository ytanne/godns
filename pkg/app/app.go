package app

import (
	"context"

	"github.com/ytanne/godns/pkg/config"
	dnsServer "github.com/ytanne/godns/pkg/dns-server"
	httpServer "github.com/ytanne/godns/pkg/http-server"
	repo "github.com/ytanne/godns/pkg/repo/leveldb"
	"go.uber.org/zap"
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
	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}

	defer logger.Sync()

	db, err := repo.NewLevelDB(a.config.DbPath)
	if err != nil {
		return err
	}

	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("could not close database", zap.Error(err))
		}
	}()

	a.dnsServer = dnsServer.NewDnsServer(a.config.DnsPort, db, dnsServer.WithLogger(logger))
	a.webServer = httpServer.NewHttpServer(a.config.WebConfig, db, httpServer.WithLogger(logger))

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		logger.Info("Starting dns server", zap.String("port", a.config.DnsPort))

		err := a.dnsServer.ListenAndServe()
		if err != nil {
			logger.Error("Failed to start server", zap.Error(err))
		}

		return err
	})

	g.Go(func() error {
		logger.Info("Starting web server at", zap.String("port", a.config.WebConfig.HttpPort))

		err := a.webServer.ListenAndServe()
		if err != nil {
			logger.Error("Failed to start http server", zap.Error(err))
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
