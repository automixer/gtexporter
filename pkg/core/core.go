package core

import (
	"context"
	"fmt"
	log "github.com/golang/glog"
	"gopkg.in/yaml.v2"
	"os"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
	"github.com/automixer/gtexporter/pkg/gnmiclient"
	"github.com/automixer/gtexporter/pkg/plugins"

	// Plugins registration
	_ "github.com/automixer/gtexporter/pkg/plugins/ocinterfaces"
	_ "github.com/automixer/gtexporter/pkg/plugins/oclldp"
)

type Core struct {
	exporterCfg exporter.Config
	clientCfg   map[string]gnmiclient.Config // Key: device name
	plugCfg     map[string][]plugins.Config  // Key: device name
}

func New(cfgFile string) (*Core, error) {
	app := Core{}
	yCfg := &yamlConfig{}

	f, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, err
	}

	err = yaml.UnmarshalStrict(f, yCfg)
	if err != nil {
		return nil, err
	}

	err = app.parseAppConfig(yCfg)
	if err != nil {
		return nil, err
	}
	return &app, err
}

func (c *Core) Run(ctx context.Context) error {
	// Load Prometheus exporter
	pExp, err := exporter.New(c.exporterCfg)
	if err != nil {
		return err
	}

	// Load devices (gNMI Clients)
	clientList := make([]*gnmiclient.GnmiClient, 0, len(c.clientCfg))
	clientCount := 0
	plugCount := 0
	for clientName, clientCfg := range c.clientCfg {
		gClt, err := gnmiclient.New(clientCfg)
		if err != nil {
			return err
		}
		// Load and register plugins to the newly created device
		for _, plugCfg := range c.plugCfg[clientName] {
			newPlug, err := plugins.New(plugCfg)
			if err != nil {
				return err
			}
			err = gClt.RegisterPlugin(plugCfg.PlugName, newPlug)
			if err != nil {
				return err
			}
			plugCount++
		}
		clientList = append(clientList, gClt)
		clientCount++
	}
	if len(clientList) == 0 {
		return fmt.Errorf("device list is empty")
	}
	log.Infof("%d gNMI client(s) loaded - %d plugin(s) loaded...", clientCount, plugCount)

	// Start the exporter
	if err := pExp.Start(); err != nil {
		return err
	}

	// Start devices
	for _, dev := range clientList {
		err := dev.Start()
		if err != nil {
			log.Error(err)
		}
	}

	// Wait for exiting
	<-ctx.Done()
	// First stop the exporter
	pExp.Close()
	// Then unload all devices
	for _, dev := range clientList {
		dev.Close()
	}
	return nil
}
