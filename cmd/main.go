package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ytanne/godns/pkg/app"
	"github.com/ytanne/godns/pkg/config"
)

const (
	dnsport = 1773
	argLen  = 2
)

func main() {
	if len(os.Args) != argLen {
		log.Println("must provide a config file")

		return
	}

	config, err := config.NewConfig(os.Args[1])
	if err != nil {
		log.Println("could not get config:", err)

		return
	}

	a := app.NewApp(config)

	go func() {
		err = a.Run(context.Background())
		if err != nil {
			log.Println("application failed to run:", err)
		}
	}()

	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, syscall.SIGINT, syscall.SIGTERM)

	osSignal := <-osSignalChan
	log.Printf("Obtained %v signal, closing application", osSignal)
	a.Close()
}
