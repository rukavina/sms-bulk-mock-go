package main

import (
	"flag"
	"log"
	"net/http"
)

var addr = flag.String("addr", ":8080", "http service address")


func main() {
	flag.Parse()
	
	hub := newHub()
	go hub.run()

	http.Handle("/", http.FileServer(http.Dir("./public")))

	http.HandleFunc("/bulk_server", func(w http.ResponseWriter, r *http.Request) {
		serveBulkServer(hub, w, r)
	})	
	
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	log.Printf("Mock bulk server running @ [%s]\n", *addr)

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}	
}
