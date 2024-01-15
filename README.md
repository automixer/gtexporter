# gNMI Telemetry Exporter

## Introduction
**GtExporter** is a tool for exporting gNMI streaming telemetries to Prometheus in a data-model-aware way.  
The application is designed around these three objects:
- The ```GnmiClient``` manages the gNMI session to the monitored device and subscribes the 
required xPath by the ```Schema Plugins```. 
A single ```GnmiClient``` instance can run several ```Schema plugins```. 
An application instance can run several ```GnmiClients```.
- The ```Schema Plugin``` is responsible for:
  - Parse the incoming gNMI telemetry streams and load them on a data structure based on the selected YANG datamodel.
  - Format the content of that data structure in a Prometheus easy-to-query metrics collection.
- The ```Exporter``` waits for scrapes from Prometheus server and exposes the collected ```Schema Plugins``` 
metrics to it.  

This project is currently under development. Features and database schema can change suddenly without notice.  
**PLEASE DO NOT USE IN PRODUCTION ENVIRONMENTS**  

## Getting Started
This [configuration template file](config-keys.yaml) describes the supported configuration keys.

## License
Licensed under MIT license. See [LICENSE](LICENSE).