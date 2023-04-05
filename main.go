package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"time"
)

var addr = flag.String("addr", ":8080", "http service address")

func main() {
	//parse commadn line arguments
	flag.Parse()
	//create run hub
	hub := newHub()
	go hub.run()

	//disable certificate verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{
		Timeout:   time.Second * 10,
		Transport: tr,
	}

	//server all static files
	http.Handle("/", http.FileServer(http.Dir("./public")))

	//bulk gate endpoint
	http.HandleFunc("/bulk_server", func(w http.ResponseWriter, r *http.Request) {
		serveBulkServer(hub, httpClient, w, r)
	})

	//test dlr handler - tests client side
	http.HandleFunc("/dlr_test", func(w http.ResponseWriter, r *http.Request) {
		serveTestDlrHandler(w, r)
	})

	//websocket handler
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	log.Printf("Mock bulk server running @ [%s]\n", *addr)

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
