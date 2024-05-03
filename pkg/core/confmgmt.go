package core

import (
	"errors"
	"fmt"
	log "github.com/golang/glog"
	"github.com/openconfig/gnmi/proto/gnmi"
	"regexp"
	"strconv"
	"time"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
	"github.com/automixer/gtexporter/pkg/gnmiclient"
	"github.com/automixer/gtexporter/pkg/plugins"
)

const (
	minScrapeInterval = time.Second
	minSessionTTL     = 10 * time.Minute
)

type yamlDevConfig struct {
	Keys    map[string]string `yaml:"devices,inline"`
	Plugins []string          `yaml:"plugins"`
	Options map[string]string `yaml:"options"`
}

type yamlConfig struct {
	Global    map[string]string `yaml:"global"`
	Templates yamlDevConfig     `yaml:"device_template"`
	Devices   []yamlDevConfig
}

// parseAppConfig parses the application configuration.
// This is the top function called by the Core object constructor
func (c *Core) parseAppConfig(yCfg *yamlConfig) error {
	var err error
	// Check Global cfg
	err = c.validateGlobalConfig(yCfg)
	if err != nil {
		return err
	}

	// Merge template with device cfg
	if yCfg.Devices == nil {
		return errors.New("no devices configured")
	}
	for i, devCfg := range yCfg.Devices {
		// Keys
		for k, v := range yCfg.Templates.Keys {
			if devCfg.Keys[k] == "" {
				yCfg.Devices[i].Keys[k] = v
			}
		}
		// Plugin list
		if devCfg.Plugins == nil {
			if yCfg.Templates.Plugins == nil {
				return errors.New("no plugins configured")
			}
			yCfg.Devices[i].Plugins = append(yCfg.Devices[i].Plugins, yCfg.Templates.Plugins...)
		}
		// Plugin options
		if devCfg.Options == nil {
			yCfg.Devices[i].Options = yCfg.Templates.Options
		}
	}

	// Check devices cfg
	deviceNames := make(map[string]bool)
	for _, dev := range yCfg.Devices {
		err = c.validateDeviceConfig(&dev)
		if err != nil {
			return err
		}
		// Device names must be unique
		if deviceNames[dev.Keys["name"]] {
			return fmt.Errorf("duplicated device name: %s", dev.Keys["name"])
		}
		deviceNames[dev.Keys["name"]] = true
	}

	// Build exporter config
	c.buildExporterCfg(yCfg)

	// Build Gnmi Clients and plugins config
	c.clientCfg = make(map[string]gnmiclient.Config, len(yCfg.Devices))
	c.plugCfg = make(map[string][]plugins.Config, len(yCfg.Devices))
	for i := range yCfg.Devices {
		c.buildGnmiClientCfg(yCfg, i)
		c.buildPluginCfg(yCfg, i)
	}

	return nil
}

// validateGlobalConfig validates the global configuration section in the configuration file.
func (c *Core) validateGlobalConfig(yCfg *yamlConfig) error {
	// Global section
	if yCfg.Global["instance_name"] == "" {
		yCfg.Global["instance_name"] = "default"
	}
	if yCfg.Global["listen_address"] == "" {
		yCfg.Global["listen_address"] = "0.0.0.0"
	}
	if yCfg.Global["listen_port"] == "" {
		yCfg.Global["listen_port"] = "9456"
	}
	if yCfg.Global["listen_path"] == "" {
		yCfg.Global["listen_path"] = "/metrics"
	}
	rx := regexp.MustCompile("^[a-zA-Z0-9_]*$")
	if !rx.MatchString(yCfg.Global["metric_prefix"]) {
		return fmt.Errorf("%s is not a valid Prometheus metric name", yCfg.Global["metric_prefix"])
	}
	sInt, _ := time.ParseDuration(yCfg.Global["scrape_interval"])
	if sInt < minScrapeInterval {
		return fmt.Errorf("scrape interval must be greater than or equal to %s", minScrapeInterval)
	}
	return nil
}

// validateDeviceConfig checks if mandatory keys are present
func (c *Core) validateDeviceConfig(yCfg *yamlDevConfig) error {
	if _, ok := yCfg.Keys["name"]; !ok {
		return fmt.Errorf("device section must contain a device name")
	}
	if yCfg.Keys["address"] == "" {
		return fmt.Errorf("device section must contain an address")
	}
	if yCfg.Keys["port"] == "" {
		return fmt.Errorf("device section must contain a port")
	}
	return nil
}

// buildExporterCfg builds the exporter configuration struct based on the provided yamlConfig object.
func (c *Core) buildExporterCfg(yCfg *yamlConfig) {
	c.exporterCfg = exporter.Config{
		ListenAddress: yCfg.Global["listen_address"],
		ListenPort:    yCfg.Global["listen_port"],
		ListenPath:    yCfg.Global["listen_path"],
		InstanceName:  yCfg.Global["instance_name"],
		MetricPrefix:  yCfg.Global["metric_prefix"],
	}
}

// buildGnmiClientCfg builds the device configuration struct based on the provided yamlConfig object.
func (c *Core) buildGnmiClientCfg(yCfg *yamlConfig, index int) {
	src := yCfg.Devices[index]
	// String values
	newDev := gnmiclient.Config{
		IPAddress:     src.Keys["address"],
		Port:          src.Keys["port"],
		User:          src.Keys["user"],
		Password:      src.Keys["password"],
		TLSCert:       src.Keys["tls_cert"],
		TLSKey:        src.Keys["tls_key"],
		TLSCa:         src.Keys["tls_ca"],
		ForceEncoding: src.Keys["force_encoding"],
		DevName:       src.Keys["name"],
		Vendor:        src.Keys["vendor"],
	}
	// Bool values
	flag, _ := strconv.ParseBool(src.Keys["tls"])
	newDev.TLS = flag
	flag, _ = strconv.ParseBool(src.Keys["tls_insecure_skip_verify"])
	newDev.TLSInsecureSkipVerify = flag
	flag, _ = strconv.ParseBool(src.Keys["on_change"])
	if flag {
		newDev.GnmiSubscriptionMode = gnmi.SubscriptionMode_ON_CHANGE
	} else {
		newDev.GnmiSubscriptionMode = gnmi.SubscriptionMode_SAMPLE
	}
	// Int values
	newDev.OverSampling, _ = strconv.ParseInt(src.Keys["oversampling"], 10, 64)
	// Duration values
	scrapeInterval, _ := time.ParseDuration(yCfg.Global["scrape_interval"])
	newDev.ScrapeInterval = scrapeInterval
	maxLife, err := time.ParseDuration(src.Keys["max_life"])
	if err == nil && maxLife < minSessionTTL {
		log.Warningf("%s: max_life cannot be less than %s.", newDev.DevName, minSessionTTL)
		maxLife = 0
	}
	newDev.MaxLife = maxLife
	// Plugin mode
	switch src.Keys["mode"] {
	case "cache":
		newDev.GnmiUpdatesOnly = false
	default:
		newDev.GnmiUpdatesOnly = true
	}

	c.clientCfg[src.Keys["name"]] = newDev
}

// buildPluginCfg builds the plugin configuration struct based on the provided yamlConfig object.
func (c *Core) buildPluginCfg(yCfg *yamlConfig, index int) {
	src := yCfg.Devices[index]
	c.plugCfg[src.Keys["name"]] = make([]plugins.Config, 0, len(src.Plugins))
	for _, plugName := range src.Plugins {
		// String values
		newPlug := plugins.Config{
			DevName:      src.Keys["name"],
			PlugName:     plugName,
			CustomLabel:  src.Keys["custom_label"],
			DescSanitize: src.Keys["desc_sanitize"],
			Options:      make(map[string]string),
		}
		// Default string values
		if newPlug.DescSanitize == "" {
			newPlug.DescSanitize = "[a-zA-Z0-9_:\\-/]"
		}

		// Bool values
		flag, _ := strconv.ParseBool(src.Keys["use_go_defaults"])
		newPlug.UseGoDefaults = flag
		// Plugin mode
		if src.Keys["mode"] == "cache" {
			newPlug.CacheData = true
		}
		// Duration values
		scrapeInterval, _ := time.ParseDuration(yCfg.Global["scrape_interval"])
		newPlug.ScrapeInterval = scrapeInterval
		c.plugCfg[src.Keys["name"]] = append(c.plugCfg[src.Keys["name"]], newPlug)
		// Plugin options
		for k, v := range src.Options {
			newPlug.Options[k] = v
		}
	}
}
