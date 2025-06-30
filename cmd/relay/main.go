package main

import (
	"flag"
	"log"

	"sdwan/internal/relay"
)

func main() {
	var listen = flag.String("listen", ":3478", "udp listen addr")
	flag.Parse()
	r := relay.NewUDPRelay(*listen)
	if err := r.Run(); err != nil {
		log.Fatal(err)
	}
}
