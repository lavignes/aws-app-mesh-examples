package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-xray-sdk-go/xray"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// This is a simple HTTP server that supports cleartext HTTP1.1 and HTTP2
// requests as well as upgrading to HTTP2 via h2c
// Example:
// $ COLOR=red PORT=8080 go run main.go
// $ curl --http2-prior-knowledge -i localhost:8080
func main() {
	color := os.Getenv("COLOR")
	if color == "" {
		log.Fatalf("no COLOR defined")
	}
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatalf("no PORT defined")
	}
	appName := os.Getenv("APP_NAME")
	if appName == "" {
		log.Fatalf("no APP_NAME defined")
	}
	log.Printf("COLOR is: %v", color)
	log.Printf("PORT is: %v", port)
	log.Printf("APP_NAME is: %v", appName)

	flakeRate := float32(0.0)
	flakeCode := 200
        xraySegmentNamer := xray.NewFixedSegmentNamer(appName)

	mux := http.NewServeMux()
	mux.Handle("/ping", xray.Handler(xraySegmentNamer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))

	mux.Handle("/", xray.Handler(xraySegmentNamer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %v", r)
		if rand.Float32() < flakeRate {
			http.Error(w, "flaky server", flakeCode)
			return
		}
		fmt.Fprintf(w, "%s", color)
	})))

	mux.Handle("/setFlake", xray.Handler(xraySegmentNamer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %v", r)
		query := r.URL.Query()
		rates, ok := query["rate"]
		if !ok {
			http.Error(w, "rate must be specified", 400)
			log.Printf("Could not read rate parameter")
			return
		}
		rate, err := strconv.ParseFloat(rates[0], 32)
		if err != nil {
			http.Error(w, err.Error(), 400)
			log.Printf("Could not parse rate parameter: %v", err)
			return
		}
		if rate < 0.0 || rate > 1.0 {
			http.Error(w, "rate must be between 0.0 and 1.0", 400)
			log.Printf("Invalid rate parameter: %v", rate)
			return
		}

		codes, ok := query["code"]
		if !ok {
			http.Error(w, "code must be specified", 400)
			log.Printf("Could not read code parameter: %v", err)
			return
		}

		code, err := strconv.ParseInt(codes[0], 10, 32)
		if err != nil {
			http.Error(w, err.Error(), 400)
			log.Printf("Could not parse code parameter: %v", err)
			return
		}

		flakeRate = float32(rate)
		flakeCode = int(code)
		fmt.Fprintf(w, "rate: %g, code: %d", flakeRate, flakeCode)
	})))

	h2s := &http2.Server{}
	h1s := &http.Server{
		Addr:    "0.0.0.0:" + port,
		Handler: h2c.NewHandler(mux, h2s),
	}

	log.Fatal(h1s.ListenAndServe())
}
