package gnmiclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	log "github.com/golang/glog"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"math"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Constants
const (
	timeoutMultiplier = 3
	oversampling      = 2
	srBufferSize      = 128
)

type plugin interface {
	GetPlugName() string
	GetPathsToSubscribe() []string
	GetDataModel() string
	OnSync(status bool)
	Notification(nf *gnmi.Notification)
}

type Config struct {
	IPAddress             string
	Port                  string
	User                  string
	Password              string
	TLS                   bool
	TLSCert               string
	TLSKey                string
	TLSCa                 string
	TLSInsecureSkipVerify bool
	ForceEncoding         string
	DevName               string
	ScrapeInterval        time.Duration
	MaxLife               time.Duration
	GnmiSubscriptionMode  gnmi.SubscriptionMode
	GnmiUpdatesOnly       bool
	OverSampling          int64
	Vendor                string
}

// GnmiClient The gNMI client object
type GnmiClient struct {
	clientMon
	config    Config
	shutdown  func()
	encoding  gnmi.Encoding
	plugins   map[string]plugin   // Map key: plugin name
	xPathList map[string][]string // Map key: plugin name. Paths to be subscribed, including YANG keys filter
	xPaths    map[string]plugin   // Map key: subscribed xPath (schema path used for routing subResponses)

}

// New Creates a new GnmiClient instance
func New(cfg Config) (*GnmiClient, error) {
	gClient := &GnmiClient{config: cfg}
	gClient.xPathList = make(map[string][]string)
	if err := gClient.clientMon.configure(cfg.DevName); err != nil {
		return nil, err
	}
	return gClient, nil
}

// Close closes the GnmiClient instance. If the shutdown function is not nil,
// it is called to gracefully terminate the underlying client.
func (c *GnmiClient) Close() {
	if c.shutdown != nil {
		c.shutdown()
	}
}

// RegisterPlugin registers a plugin instance into the GnmiClient.
func (c *GnmiClient) RegisterPlugin(name string, plug plugin) error {
	if c.plugins == nil {
		c.plugins = make(map[string]plugin)
	}
	if _, ok := c.plugins[name]; ok {
		return fmt.Errorf("plugin %s is already registered", name)
	}
	if plug == nil {
		return fmt.Errorf("plugin cannot be nil")
	}

	if c.xPaths == nil {
		c.xPaths = make(map[string]plugin)
	}

	plugPaths := plug.GetPathsToSubscribe()
	re := regexp.MustCompile(`\[.*?]`)
	for _, reqPath := range plugPaths {
		c.xPathList[name] = append(c.xPathList[name], reqPath)
		// Remove keys from YANG path
		reqPath = re.ReplaceAllString(reqPath, "")
		c.xPaths[reqPath] = plug
	}

	c.plugins[name] = plug
	return nil
}

// Start starts the goRoutine that take care of GNMI channel
// It is non-blocking
func (c *GnmiClient) Start() error {
	gCtx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		c.run(gCtx)
		wg.Done()
	}()
	c.shutdown = func() {
		cancel()
		wg.Wait()
	}
	return nil
}

// newDialOptions returns a slice of grpc.DialOptions and an error. These options
// are used to configure the dialing process of the gNMI client.
// The options include:
// - Setting the maximum received message size for calls
// - Setting the backoff and minimum connect timeout values
// - Configuring TLS for secure connections
// - Setting device access credentials per RPC
func (c *GnmiClient) newDialOptions() ([]grpc.DialOption, error) {
	opts := make([]grpc.DialOption, 0)
	opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)))

	// Backoff settings
	opts = append(opts, grpc.WithConnectParams(grpc.ConnectParams{Backoff: backoff.DefaultConfig,
		MinConnectTimeout: time.Second * 20}))

	// TLS
	if c.config.TLS {
		// Secure connection
		var rootCAs *x509.CertPool
		if c.config.TLSCa != "" {
			// Load custom CA
			var err error
			rootCAs, err = x509.SystemCertPool()
			if err != nil {
				rootCAs = x509.NewCertPool()
			}

			var ca []byte
			ca, err = os.ReadFile(c.config.TLSCa)
			if err != nil {
				return nil, err
			}

			if ok := rootCAs.AppendCertsFromPEM(ca); !ok {
				return nil, fmt.Errorf("<%s>: cannot load CA certificate file", c.config.DevName)
			}
		}

		cert, err := tls.LoadX509KeyPair(c.config.TLSCert, c.config.TLSKey)
		if err != nil {
			return nil, err
		}
		tlsCfg := &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            rootCAs,
			InsecureSkipVerify: c.config.TLSInsecureSkipVerify,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		// Clear text
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Device access credentials (per RPC)
	if c.config.User != "" && c.config.Password != "" {
		var secure bool
		if c.config.TLS {
			secure = true
		} else {
			secure = false
		}
		devCreds := newPerRpcCreds(c.config.User, c.config.Password, secure)
		opts = append(opts, grpc.WithPerRPCCredentials(devCreds))
	}
	return opts, nil
}

// checkCapabilities verifies if the target device supports the required datamodels and encodings.
// The capabilities include:
// - Check if the required datamodel is supported
// - Check if the required encoding is supported
func (c *GnmiClient) checkCapabilities(ctx context.Context, stub gnmi.GNMIClient) error {
	caps, err := stub.Capabilities(ctx, &gnmi.CapabilityRequest{})
	if err != nil {
		return err
	}

	// Check for yang datamodels support
	supportedModels := make(map[string]*gnmi.ModelData, len(caps.SupportedModels))
	for _, model := range caps.SupportedModels {
		supportedModels[model.Name] = model
	}
	for _, plug := range c.plugins {
		reqModel := plug.GetDataModel()
		if _, ok := supportedModels[reqModel]; !ok {
			return fmt.Errorf("the yang model <%s> is not supported by %s", reqModel, c.config.DevName)
		}
	}

	// Pick protocol buffer by default
	c.encoding = gnmi.Encoding_PROTO

	// Override if required
	if c.config.ForceEncoding != "" {
		// Config enforces encoding
		c.config.ForceEncoding = strings.ToUpper(c.config.ForceEncoding)
		switch c.config.ForceEncoding {
		case "JSON":
			c.encoding = gnmi.Encoding_JSON
		case "BYTES":
			c.encoding = gnmi.Encoding_BYTES
		case "PROTO":
			c.encoding = gnmi.Encoding_PROTO
		case "ASCII":
			c.encoding = gnmi.Encoding_ASCII
		case "JSON_IETF":
			c.encoding = gnmi.Encoding_JSON_IETF
		default:
			return fmt.Errorf("the encoding %s is not supported by gNMI", c.config.ForceEncoding)
		}
	}
	return nil
}

// receive takes care of receiving the GNMI stream from the device
func (c *GnmiClient) receive(sub gnmi.GNMI_SubscribeClient) error {
	ch := make(chan *gnmi.SubscribeResponse, srBufferSize)
	done := make(chan struct{})
	var err error
	var sr *gnmi.SubscribeResponse

	go func() {
		for {
			sr, err = sub.Recv()
			if err != nil {
				close(done)
				close(ch)
				return
			}
			ch <- sr
			c.srBufSize(len(ch))
		}
	}()

	for {
		select {
		case <-done:
			<-ch
			for _, plug := range c.plugins {
				plug.OnSync(false)
			}
			return err
		case msg := <-ch:
			c.routeSr(msg)
		}
	}
}

// routeSr examines the subscribe response paths metadata and sends the SR object to the related plugin
func (c *GnmiClient) routeSr(sr *gnmi.SubscribeResponse) {
	// Sync response
	if sr.GetSyncResponse() {
		for _, plug := range c.plugins {
			plug.OnSync(true)
		}
		return
	}

	// Notification
	nf := sr.GetUpdate() // Beware! GetUpdate() actually returns a notification, not an Update :-(
	c.incNfCounters(uint64(len(nf.GetUpdate())), uint64(len(nf.GetDelete())))
	if nf.GetPrefix().GetTarget() != "" {
		// Huawei specific
		if c.config.Vendor == "huawei" {
			c.removeDmPfxFromPath(nf)
		}
		// Normal messages routing
		if _, ok := c.plugins[nf.Prefix.Target]; !ok {
			// Unknown destination
			c.incSrRoutingErrors()
			return
		}
		c.plugins[nf.Prefix.Target].Notification(nf)
	} else {
		// Huawei specific
		if c.config.Vendor == "huawei" {
			c.removeDmPfxFromPath(nf)
		}

		// The device does not support gnmi targeting, or the subscription does not include a target
		pfx, _ := ygot.PathToSchemaPath(nf.Prefix)
		if len(pfx) < 2 {
			// Empty prefix
			pfx = ""
		}

		// Search for Updates
		for _, upd := range nf.GetUpdate() {
			path, _ := ygot.PathToSchemaPath(upd.Path)
			fullPath := pfx + path
			for xPath, plug := range c.xPaths {
				if strings.HasPrefix(fullPath, xPath) {
					plug.Notification(nf)
					return
				}
			}
		}
		// Search for Deletes
		for _, delPath := range nf.GetDelete() {
			path, _ := ygot.PathToSchemaPath(delPath)
			fullDelPath := pfx + path
			for xPath, plug := range c.xPaths {
				if strings.HasPrefix(fullDelPath, xPath) {
					plug.Notification(nf)
					return
				}
			}
		}
		// Unknown destination
		// Sr response error field is deprecated and not handled
		c.incSrRoutingErrors()
	}
}

// removeDmPfxFromPath sanitizes the prefix, updates, and deletes paths in the given
// gnmi.Notification object. It removes any namespace prefix from the path names to
// ensure consistent handling of paths across plugins.
// NOTE: the deprecated "element" field is not supported
func (c *GnmiClient) removeDmPfxFromPath(nf *gnmi.Notification) {
	// Sanitize Prefix
	if nf.Prefix != nil && len(nf.Prefix.Elem) > 0 {
		splitted := strings.SplitAfter(nf.Prefix.Elem[0].Name, ":")
		if len(splitted) == 2 {
			nf.Prefix.Elem[0].Name = splitted[1]
		}
	}

	// Sanitize updates
	for i := 0; i < len(nf.Update); i++ {
		if nf.Update[i] != nil && nf.Update[i].Path != nil && len(nf.Update[i].Path.Elem) > 0 {
			splitted := strings.SplitAfter(nf.Update[i].Path.Elem[0].Name, ":")
			if len(splitted) == 2 {
				nf.Update[i].Path.Elem[0].Name = splitted[1]
			}
		}
	}

	// Sanitize deletes
	for i := 0; i < len(nf.Delete); i++ {
		if len(nf.Delete[i].Elem) > 0 {
			splitted := strings.SplitAfter(nf.Delete[i].Elem[0].Name, ":")
			if len(splitted) == 2 {
				nf.Delete[i].Elem[0].Name = splitted[1]
			}
		}
	}
}

// run is the main loop for gNMI worker thread. It establishes a connection to the target
// device using the specified dial options, checks the device capabilities, subscribes to
// gNMI telemetry, and continuously receives the gNMI stream. It runs until the context is
// canceled or an error occurs.
func (c *GnmiClient) run(ctx context.Context) {
	var conn *grpc.ClientConn
	var dialOpts []grpc.DialOption
	var err error
	var stub gnmi.GNMIClient
	var sub gnmi.GNMI_SubscribeClient
	var gCtx context.Context
	var gCtxCancelFunc func()
	var maxLifeExpired bool
	var sessionTimer *time.Timer

	// Setup dial options
	dialOpts, err = c.newDialOptions()
	if err != nil {
		log.Error(err)
		log.Errorf("Device %s has been disabled...", c.config.DevName)
		return
	}

	// Setup target ip address
	var targetDev string
	if net.ParseIP(c.config.IPAddress) != nil {
		targetDev = fmt.Sprintf("%s:%s", c.config.IPAddress, c.config.Port)
	} else {
		targetDev = fmt.Sprintf("dns:///%s:%s", c.config.IPAddress, c.config.Port)
	}

	// Setup session TTL timer
	if c.config.MaxLife != 0 {
		sessionTimer = time.AfterFunc(c.config.MaxLife, func() {
			if gCtx != nil && conn != nil {
				gCtxCancelFunc()
				gCtx = nil
				gCtxCancelFunc = nil
				maxLifeExpired = true
				_ = conn.Close()
			}
			sessionTimer.Reset(c.config.MaxLife)
		})
	}

	// This is the gNMI worker thread main loop
	for {
		// Reconnecting?
		if gCtxCancelFunc != nil {
			<-gCtx.Done() // Wait deadline before retrying
			gCtxCancelFunc()
			_ = conn.Close()
		}
		// Time to exit?
		if ctx.Err() != nil {
			if sessionTimer != nil {
				sessionTimer.Stop()
			}
			break
		}
		// Reconnecting after MaxLife expired?
		if maxLifeExpired {
			maxLifeExpired = false
			time.Sleep(100 * time.Millisecond)
		}

		// Dial
		log.Infof("Dialing %s...", c.config.DevName)
		conn, err = grpc.NewClient(targetDev, dialOpts...)
		if err != nil {
			log.Info(err)
			c.incDialErrors()
			continue
		}
		stub = gnmi.NewGNMIClient(conn)

		// Check capabilities
		timeout := c.config.ScrapeInterval * timeoutMultiplier
		if timeout > time.Minute*5 {
			timeout = time.Minute * 5
		}
		gCtx, gCtxCancelFunc = context.WithTimeout(ctx, timeout)
		log.Infof("Checking %s capabilities...", c.config.DevName)
		if err = c.checkCapabilities(gCtx, stub); err != nil {
			log.Info(err)
			c.incCheckCapsErrors()
			continue
		}

		// Subscribe
		log.Infof("Subscribing gNMI telemetries to %s...", c.config.DevName)
		sub, err = c.subscribe(ctx, stub)
		if err != nil {
			log.Info(err)
			c.incSubscribeErrors()
			continue
		}

		// Receive gNMI stream (blocking)
		log.Infof("Device %s is now online...", c.config.DevName)
		if err = c.receive(sub); err != nil {
			log.Error(err)
			c.incDisconnections()
		}
	}
}
