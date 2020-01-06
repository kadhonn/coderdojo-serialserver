package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST")
	answer := InvokeCommand(serial, r.URL.Path[1:], flat(r.URL.Query()))
	log.Println("called " + r.URL.Path[1:] + " answer is: " + answer)
	fmt.Fprintf(w, answer)
}

func flat(query url.Values) map[string]string {
	params := make(map[string]string)
	for key := range query {
		params[key] = query[key][0]
	}
	return params
}

var serial *Serial

func main() {
	var err error
	serial, err = Open()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func runTest() {
	InvokeCommand(serial, "reset", nil)

	time.Sleep(1500 * time.Millisecond)
	InvokeCommand(serial, "sound", nil)

	InvokeCommand(serial, "power_on", nil)
	time.Sleep(2000 * time.Millisecond)

	InvokeCommand(serial, "body_height", map[string]string{"height": "60"})
	time.Sleep(3000 * time.Millisecond)

	InvokeCommand(serial, "walk_forward", nil)
	time.Sleep(5000 * time.Millisecond)

	InvokeCommand(serial, "walk_stop", nil)
	time.Sleep(2000 * time.Millisecond)

	InvokeCommand(serial, "power_off", nil)
}
