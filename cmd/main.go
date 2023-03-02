package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ytanne/godns/pkg/app"
)

func main() {
	a := app.NewApp()
	go a.Run(1773)

	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, syscall.SIGINT, syscall.SIGTERM)

	osSignal := <-osSignalChan
	fmt.Println()
	log.Printf("Obtained %v signal, closing application", osSignal)
	a.Close()
}
