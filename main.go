package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	sensorService "github.com/energieip/swh200-sensorservice-go/internal/service"
	"github.com/energieip/swh200-service-go/pkg/service"
)

func main() {
	var confFile string
	var service service.Service

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.StringVar(&confFile, "config", "", "Specify an alternate configuration file.")
	flag.StringVar(&confFile, "c", "", "Specify an alternate configuration file.")
	flag.Parse()

	s := sensorService.SensorService{}
	service = &s
	err := service.Initialize(confFile)
	if err != nil {
		log.Println("Error during service connexion " + err.Error())
		os.Exit(1)
	}
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Received SIGTERM")
		service.Stop()
		os.Exit(0)
	}()

	err = service.Run()
	if err != nil {
		log.Println("Error during service execution " + err.Error())
		os.Exit(1)
	}
}
