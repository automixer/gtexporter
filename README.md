# gNMI Telemetry Exporter for Prometheus
![GitHub License](https://img.shields.io/github/license/automixer/gtexporter)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/automixer/gtexporter/release.yaml)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/automixer/gtexporter)
[![Go Report Card](https://goreportcard.com/badge/github.com/automixer/gtexporter)](https://goreportcard.com/report/github.com/automixer/gtexporter)


This project is currently under development. Features and database schema can change suddenly without notice.  
**PLEASE DO NOT USE IN PRODUCTION ENVIRONMENTS**  

**GtExporter** is a tool for subscribing and exporting gNMI streaming telemetries to Prometheus in a YANG 
"data-model-aware" way. The intended usage is to gather and store network devices' operational state metrics, 
but it should work with any [gNMI compliant](https://github.com/openconfig/reference/tree/master/rpc/gnmi) 
device that implements any [OpenConfig compliant](https://github.com/openconfig/public/blob/master/doc/openconfig_style_guide.md) YANG data model. 

## Overview
The intent of this project is to create an easy-to-scale, vendor-agnostic Prometheus exporter for gNMI
streaming telemetries. To satisfy these two requirements, the relevant design choice is to put the YANG data 
model at the center: Incoming gNMI streams are treated as part of this known data structure and processed accordingly.
The "awareness" of the underlying data model is a crucial enabler for exporting a stable and 
predictable metric set for Prometheus. This, in turn, allows horizontal scaling of the monitoring system with a 
robust vendor-neutral approach.  

The application design is built around these ideas:
- Several app instances can be spawned on different machines across the network. 
The produced metrics will seamlessly merge into one or more Prometheus server instances.
- A given instance can run several **gNMI clients** concurrently, one for each monitored device.
- Each gNMI client can run several **schema plugins**, one for each set of the YANG path on interest.
- When Prometheus server scrapes, the **exporter** collects all the received metrics from all the **schema plugins** 
and exposes them via its own [client](https://github.com/prometheus/client_golang) library.  

Hence, the central component of the app is the **schema plugin**:
- A **schema plugin** is responsible for subscribing and rendering a set of 
[schema paths](https://openconfig.net/projects/models/paths/) from a selected YANG data model.
- Internally, it is further divided into two components:
  - The **Parser** decodes and loads the received gNMI notifications into the related **GoStruct** data structure.
  - The **Formatter** reads the **GoStruct** and builds up the metrics to be exported when Prometheus asks for them. 
- The **GoStruct** is a "data container" that represents the structure of the selected YANG data model. 
It is generated using the Openconfig [yGot](https://github.com/openconfig/ygot) project and takes as input for code generation the actual ```.yang``` 
files published by the device vendor or the OpenConfig community.

## Getting Started
### Build from sources
Requires ```go 1.22.0``` or higher and ```make```.
```
git clone https://github.com/automixer/gtexporter.git
cd gtexporter
make
```
The binary executable will be created into the ```build/``` project folder. The mandatory argument is 
a valid config file: ```gtexporter -config <path/to/config/file>```.

### Use the provided Docker image
Requires ```Docker Engine```. The supported platforms are ```linux/amd64``` and ```linux/arm64```.
```
docker run -p 9456:9456/tcp --mount type=bind,source=.,target=/etc/gte/ automixer/gtexporter -config /etc/gte/config.yaml
```
The mandatory argument is a valid config file named ```config.yaml``` into the current folder.

### The Configuration File
This [configuration file template](config-keys.yaml) describes the supported configuration keys.  
The file is subdivided into three sections:
1) The ```global``` section contains application-wide settings like the Prometheus client listen address and port.
2) The ```device_template``` section contains the settings shared among all devices. This section can
help keep the configuration file small and readable. It can be empty, and any key can be overridden by a more
specific setting into the ```devices``` section.
3) The ```devices``` section contains the device-specific settings, like the device name, IP address and, port 
of the target. It inherits the contents of the ```device_template``` section and, if a key is present on both, the more
specific wins (i.e.: the one coming from ```devices```).

### A Simple Config File Example
```
# These keys are application-wide.
global:
  instance_name: test_gte
  metric_prefix: gnmi
  listen_address: 0.0.0.0
  listen_port: 9456
  scrape_interval: 1m

# These keys are shared among all devices.
device_template:
  port: 6030
  user: admin
  password: admin

# These keys are device-specific and take precedence over the device_template section.
devices:
  - name: Router1
    address: r1.example.com
    plugins: [oc_interfaces,oc_lldp]
    port: 50051

  - name: Router2
    address: 192.0.2.10
    plugins: [oc_interfaces]
```

## Supported Schema Plugins
These are the currently available ```schema plugins```:
### ```oc_interfaces```
This plugin is based on the ```openconfig-interfaces``` data model.  
Subscribe to these schema paths:
1) ```/interfaces/interface/state/```
2) ```/interfaces/interface/aggregation/state/```
3) ```/interfaces/interface/subinterfaces/subinterface/state/```

Produces two Prometheus metrics:
1) ```<configured_metric_prefix>_oc_if_counters{}```.
2) ```<configured_metric_prefix>_oc_if_gauges{}```.

### ```oc_lldp```
This plugin is based on the ```openconfig-lldp``` data model.  
Subscribe to this schema path:
1) ```/lldp/interfaces/interface/neighbors/neighbor/state/```

Produces one Prometheus metrics:  
1) ```<configured_metric_prefix>_oc_lldp_if_nbr_gauges{}```.  
LLDP must be enabled on the target devices.

## Self-Monitoring Services
In addition to the ```schema plugins```, **GtExporter** emits several self-monitoring metrics to keep track of 
the app's health and operational state.  
These metrics are:
1) ```<configured_metric_prefix>_gnmi_client_counters{}```: These counters describe the state of the underlying gNMI
client instances.
2) ```<configured_metric_prefix>_gnmi_client_gauges{}```: These gauges describe the state of the underlying gNMI
client instances.
3) ```<configured_metric_prefix>_plugin_formatter_gauges{}```: These gauges describe the operational state of the 
running plugin's formatters.
4) ```<configured_metric_prefix>_plugin_parser_counters{}```: These counters describe the operational state of the 
running plugin's parsers.
5) The default Go Runtime Metrics exported by the Prometheus client library.

## Caveats
### The ```global:scrape_interval``` setting
This config key plays an important role. Together with the ```device:oversampling``` it is used to 
compute the actual sample interval of the gNMI subscription. **It must match the configured Prometheus 
scrape interval.**  
Prometheus scrapes and gNMI sample interval are two asynchronous loops that can't be synchronized. To keep
acquisition errors within acceptable levels, the gNMI subscription loop should run at least two times faster 
than Prometheus. This ensures that even if a scrape occurs in the middle of a gNMI batch delivery, the collected
samples always fall within the Prometheus scrape interval.  
The formula used to compute the gNMI sample interval is: ```sample_interval=scrape_interval/oversampling```.
The default ```device:oversampling``` value is 2.

### Cache mode and max_life
By default, **GtExporter** does not cache any data. The ```device:mode``` key can be used to force persistence of  
the yGot GoStruct over time. This setting implies that the monitored device must implement the 
gNMI delete messages mechanism, to avoid a continuously growing GoStruct.  
The ```device:max_life``` config sets a time limit on the gNMI subscription. When ```max_life``` expires, the
session is torn down and re-established, forcing a cache flush event. This setting can be useful in keeping
the GoStruct size under control.

### The ```device:desc_sanitize``` setting
Descriptions are user defined strings contained into the device configuration. Since descriptions are often used as 
Prometheus labels, not all characters are valid. The ```device:desc_sanitize``` config key is a regexp pattern
used to remove unsupported characters by Prometheus.

## License
Licensed under MIT license. See [LICENSE](LICENSE).

