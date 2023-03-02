package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ytanne/godns/pkg/app"
)

const (
	dnsport = 1773
)

func main() {
	a := app.NewApp()
	go a.Run(dnsport)

	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, syscall.SIGINT, syscall.SIGTERM)

	osSignal := <-osSignalChan
	log.Printf("Obtained %v signal, closing application", osSignal)
	a.Close()
}
