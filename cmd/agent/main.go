package main

import (
	"flag"
	"log"
	"runtime"
	"time"

	"sdwan/internal/agent"
)

func main() {
	var (
		controller = flag.String("controller", "http://127.0.0.1:8080", "controller base url")
		hostname   = flag.String("hostname", "node", "hostname")
		iface      = flag.String("iface", "wg0", "wireguard interface name")
		listenPort = flag.Int("listen", 51820, "wireguard listen port")
	)
	flag.Parse()

	a := agent.New(agent.Config{
		Controller: *controller,
		Hostname:   *hostname,
		OS:         runtime.GOOS,
		Version:    "dev",
		Endpoints:  nil,
		Iface:      *iface,
		ListenPort: *listenPort,
	})
	if err := a.Register(); err != nil {
		log.Fatalf("register: %v", err)
	}
	log.Printf("registered: id=%s ip=%s privKey(len)=%d", a.Self().ID, a.Self().TunnelIP, len(a.PrivateKey()))
	if err := a.ApplyWireGuard(nil); err != nil {
		log.Printf("apply wg (initial): %v", err)
	}

	for {
		peers, err := a.FetchPeers()
		if err != nil {
			log.Printf("peers: %v", err)
		} else {
			log.Printf("peers: %d", len(peers))
			if err := a.ApplyWireGuard(peers); err != nil {
				log.Printf("apply wg: %v", err)
			}
		}
		time.Sleep(15 * time.Second)
	}
}
