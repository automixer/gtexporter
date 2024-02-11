package main

import (
	"context"
	"flag"
	"fmt"
	log "github.com/golang/glog"
	"os"
	"os/signal"

	// Local Packages
	"github.com/automixer/gtexporter/pkg/core"
)

var (
	appName    = ""
	appVersion = ""
	buildDate  = ""
	cfgFile    = flag.String("config", "", "Config file")
	ver        = flag.Bool("version", false, "Print version info")
)

func main() {
	_ = flag.Set("logtostderr", "true")
	flag.Parse()

	// Print Version
	if *ver {
		fmt.Printf("-- %s -- A YANG gNMI telemetries exporter for Prometheus! --\n", appName)
		fmt.Println("Release:", appVersion)
		fmt.Println("Build date:", buildDate)
		os.Exit(0)
	}

	log.Infof("Starting %s %s ...", appName, appVersion)
	log.Infof("Build date : %s", buildDate)

	// Check config file
	if *cfgFile == "" {
		log.Errorf("Missing configuration argument. Exiting...")
		os.Exit(1)
	}
	if fInfo, err := os.Stat(*cfgFile); err != nil {
		log.Errorf("Configuration file not found. Exiting...")
		os.Exit(1)
	} else if fInfo.IsDir() {
		log.Errorf("Configuration file is a directory. Exiting...")
		os.Exit(1)
	}

	// Setup os signal
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)
		<-c
		cancel()
	}()

	// Load app core
	app, err := core.New(*cfgFile)
	if err != nil {
		log.Error(err)
		os.Exit(2)
	}
	err = app.Run(ctx)
	if err != nil {
		log.Error(err)
		os.Exit(3)
	}

	// Exiting normally
	log.Info("Bye bye...")
	os.Exit(0)
}
