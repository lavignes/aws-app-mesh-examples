package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/aws/aws-xray-sdk-go/xray"

	"golang.org/x/net/http2"
)

func main() {
	colorHost := os.Getenv("COLOR_HOST")
	if colorHost == "" {
		log.Fatalf("no COLOR_HOST defined")
	}
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatalf("no PORT defined")
	}
	appName := os.Getenv("APP_NAME")
	if appName == "" {
		log.Fatalf("no APP_NAME defined")
	}
	log.Printf("COLOR_HOST is: %v", colorHost)
	log.Printf("PORT is: %v", port)
	log.Printf("APP_NAME is: %v", appName)
	xraySegmentNamer := xray.NewFixedSegmentNamer(appName)

	// Create an h2c client
	client := xray.Client(&http.Client{
		Transport: &http2.Transport{
			// Allow non-https urls
			AllowHTTP: true,
			// Make the transport *not-actually* use TLS
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	})

	http.Handle("/ping", xray.Handler(xraySegmentNamer, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {})))

	http.Handle("/color", xray.Handler(xraySegmentNamer, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		resp, err := client.Get("http://" + colorHost)
		if err != nil {
			http.Error(w, err.Error(), 500)
			log.Printf("Could not get color: %v", err)
			return
		}
		defer resp.Body.Close()
		color, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), 400)
			log.Printf("Could not read response body: %v", err)
			return
		}
		log.Printf("Got color response: %s", string(color))
		fmt.Fprint(w, string(color))
	})))

	http.Handle("/setFlake", xray.Handler(xraySegmentNamer, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		resp, err := client.Get("http://" + colorHost + req.URL.RequestURI())
		if err != nil {
			http.Error(w, err.Error(), 500)
			log.Printf("Could not set flakiness: %v", err)
			return
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), 400)
			log.Printf("Could not read response body: %v", err)
			return
		}
		log.Printf("Got setFlake response: %s", string(body))
		if resp.StatusCode != 200 {
			http.Error(w, string(body), resp.StatusCode)
			return
		}
		fmt.Fprint(w, string(body))
	})))

	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
