# gNMI Telemetry Exporter for Prometheus
This project is currently under development. Features and database schema can change suddenly without notice.  
**PLEASE DO NOT USE IN PRODUCTION ENVIRONMENTS**  

**GtExporter** is a tool for subscribing and exporting gNMI streaming telemetries to Prometheus in a YANG 
"data-model-aware" way. The intended usage is to gather and store network devices' operational state metrics, 
but it should work with any [gNMI compliant](https://github.com/openconfig/reference/tree/master/rpc/gnmi) 
device that meets any [OpenConfig compliant](https://github.com/openconfig/public/blob/master/doc/openconfig_style_guide.md) YANG data model. 

## Overview
The application is built around these concepts:
- Several app instances can be spawned on different machines across the network. 
The produced metrics will seamlessly merge into one or more Prometheus server instances.
- A given instance can run several **gNMI clients** concurrently, one for each monitored device.
- Each gNMI client can run several **schema plugins**, one for each set of the YANG path on interest.
- When Prometheus server scrapes, the **exporter** collects all the received metrics from all the **schema plugins** 
and exposes them to Prometheus via its own [client](https://github.com/prometheus/client_golang) library.  

Hence, the central component of the app is the **schema plugin**:
- A **schema plugin** is responsible for subscribing and rendering a set of 
[schema paths](https://openconfig.net/projects/models/paths/) from a selected YANG data model.
- Internally, the schema plugin is further divided into two components:
  - The **Parser** decodes and loads the received gNMI notifications into the related **GoStruct** data structure.
  - The **Formatter** reads that **GoStruct** and builds up the metrics to be exported when Prometheus asks for them. 
- The **GoStruct** is a "data container" that represents the structure of the selected YANG data model. 
It is generated using the Openconfig [yGot](https://github.com/openconfig/ygot) project utilities and takes as 
input the actual ```.yang``` files published by the device vendor or the OpenConfig community.

The "awareness" of the underlying data model is a crucial enabler for exporting a stable and 
predictable metric set for Prometheus. This, in turn, allows horizontal scaling of the monitoring system with a 
robust vendor-neutral approach.

## Getting Started
### Build from sources
Requires ```go 1.21.6``` or higher and ```make```.
```
git clone https://github.com/automixer/gtexporter.git
cd gtexporter
make
```
The binary executable will be created into the ```build/``` project folder. The only mandatory argument is 
a valid config file: ```gtexporter -config <path/to/config/file>```.

### Use the provided Docker image
Requires ```Docker Engine```.
```
docker run -p 9456:9456/tcp --mount type=bind,source=.,target=/etc/gte/ automixer/gtexporter:latest -config /etc/gte/config.yaml
```
The only requirement is a valid config file into the current folder: ```./config.yaml```

### The Configuration File
This [configuration file template](config-keys.yaml) describes all the supported configuration keys.  
The config file is subdivided into three sections:
1) The ```global:``` section contains application-wide settings like the Prometheus client listen address and port.
2) The ```device_template:``` section contains the shared settings among all devices. This section can
help keep the configuration file small and readable. It can be empty, and any keys can be overridden by a more
specific key into the following ```devices:``` section.
3) The ```devices:``` section contains the device-specific settings, like the device name, IP address and, port.
It inherits the contents of the ```device_template:``` section and, if a key is present in both sections, the more
specific wins (i.e.: the one coming from the ```devices:``` section).

### A Simple Config File Example
```
# These keys are global.
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
This plugin is based on the ```openconfig-interfaces``` data model. It subscribes to these schema paths:
1) ```/interfaces/interface/state/```
2) ```/interfaces/interface/aggregation/state/```
3) ```/interfaces/interface/subinterfaces/subinterface/state/```
It produces two Prometheus metrics:
- ```<configured_metric_prefix>_oc_if_counters{}```.
- ```<configured_metric_prefix>_oc_if_gauges{}```.

### ```oc_lldp```
This plugin is based on the ```openconfig-lldp``` data model. It subscribes to this schema path:
1) ```/lldp/interfaces/interface/neighbors/neighbor/state/```
It produces one Prometheus metrics:
- ```<configured_metric_prefix>_oc_lldp_if_nbr_gauges{}```.
LLDP must be enabled on the target devices.

## Self-Monitoring Services
Other than the ```schema plugins``` metrics, **GtExporter** emits several self-monitoring metrics to monitor 
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


## License
Licensed under MIT license. See [LICENSE](LICENSE).

