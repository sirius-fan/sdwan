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
	)
	flag.Parse()

	a := agent.New(agent.Config{
		Controller: *controller,
		Hostname:   *hostname,
		OS:         runtime.GOOS,
		Version:    "dev",
		Endpoints:  nil,
	})
	if err := a.Register(); err != nil {
		log.Fatalf("register: %v", err)
	}
	log.Printf("registered: id=%s ip=%s privKey(len)=%d", a.Self().ID, a.Self().TunnelIP, len(a.PrivateKey()))

	for {
		peers, err := a.FetchPeers()
		if err != nil {
			log.Printf("peers: %v", err)
		} else {
			log.Printf("peers: %d", len(peers))
		}
		time.Sleep(15 * time.Second)
	}
}
