package main

import (
	"flag"
	"log"
	"net/http"

	"sdwan/internal/controller"
)

func main() {
	var (
		listen = flag.String("listen", ":8080", "http listen addr")
		cidr   = flag.String("cidr", "100.64.0.0/16", "tunnel network cidr")
	)
	flag.Parse()

	store, err := controller.NewStore(*cidr)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	api := controller.NewHTTPServer(store)
	mux := http.NewServeMux()
	api.RegisterHandlers(mux)
	log.Printf("controller listening on %s (cidr=%s)", *listen, *cidr)
	log.Fatal(http.ListenAndServe(*listen, mux))
}
