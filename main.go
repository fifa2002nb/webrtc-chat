// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"net/http"
	"text/template"
)

var addr = flag.String("addr", "0.0.0.0:8000", "http service address")
var homeTemplate = template.Must(template.ParseFiles("index.html"))

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	homeTemplate.Execute(w, r.Host)
}

func main() {
	flag.Parse()
	go hub.run()
	http.HandleFunc("/ws", serveWs)
	http.HandleFunc("/", serveHome)
	fsh := http.FileServer(http.Dir("./public"))
	http.Handle("/public/", http.StripPrefix("/public/", fsh))
	//err := http.ListenAndServe(*addr, nil)
	err := http.ListenAndServeTLS(*addr, "tls/server.crt", "tls/server.key", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
