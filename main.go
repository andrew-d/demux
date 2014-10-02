package main

import (
	"fmt"
	"net"
	"os"
	"runtime"

	flag "github.com/ogier/pflag"
	log "gopkg.in/inconshreveable/log15.v2"
)

var (
	rootLog = log.New("module", "demux")

	flagListProtocols  bool
	flagUseTransparent bool
	flagListenPort     uint16
	flagListenHost     string
)

func init() {
	flag.BoolVar(&flagListProtocols, "list-protocols", false,
		"list all registered protocols and then exit")
	flag.BoolVar(&flagUseTransparent, "transparent", true,
		"use transparent proxying (only available on Linux)")
	flag.Uint16VarP(&flagListenPort, "port", "p", 0,
		"port to listen on")
	flag.StringVarP(&flagListenHost, "host", "h", "0.0.0.0",
		"host to listen on")
}

func checkExitFlags() {
	if flagListProtocols {
		rootLog.Info(fmt.Sprintf("Currently support %d protocol(s)", len(protocols)))
		for name, _ := range protocols {
			rootLog.Debug("Protocol: " + name)
		}
		os.Exit(0)
	}
}

func validateFlags() {
	if flagListenPort == 0 {
		rootLog.Crit("You must provide a listen port")
		return
	}
	if runtime.GOOS != "linux" && flagUseTransparent {
		rootLog.Warn("Transparent proxying is only supported on Linux")
	}
}

func main() {
	rootLog.Info("Started")

	// For each registered protocol, we add the corresponding flags for its
	// destination.
	protoDestinations := make(map[string]*string)
	for name, _ := range protocols {
		protoDestinations[name] = flag.String(name+"-destination", "",
			"destination to forward "+name+" traffic to")
	}

	// Parse all flags.
	flag.Parse()

	// First off, handle flags that cause us to exit instead of actually listening
	checkExitFlags()
	validateFlags()

	// Find out what we've got enabled
	enabledProtocols := []Protocol{}
	descString := ""
	for name, flag := range protoDestinations {
		if len(*flag) > 0 {
			enabledProtocols = append(enabledProtocols, protocols[name])
			descString += name + ","
		}
	}

	if len(enabledProtocols) == 0 {
		rootLog.Crit("No protocols were enabled")
		return
	}

	log.Debug("Enabled protocols: " + descString[0:len(descString)-1])

	// Start listening
	addr := fmt.Sprintf("%s:%d", flagListenHost, flagListenPort)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		rootLog.Crit("Could not open listener", "err", err)
		return
	}
	defer l.Close()

	rootLog.Info("Started listening", "addr", addr)
	for {
		conn, err := l.Accept()
		if err != nil {
			rootLog.Error("Error accepting connection", "err", err)
			continue
		}

		p := NewProxy(conn, rootLog)
		p.EnabledProtocols = enabledProtocols
		p.ProtoDestinations = protoDestinations
		go p.Start()
	}
}
