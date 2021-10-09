/**
 * rastermimimi reads calendar data from various sources and displays then as html.
 */

package main

import (
	"flag"
	"log"
	"os"
	"strconv"
	_ "time/tzdata"
)

func main() {
	// get config from flags and environment using flag.Parse and os.Getenv
	listenAddr := flag.String("listen-addr", getenv("MIMIMI_LISTEN_ADDR", ":8080"), "listen address")

	websiteURL := flag.String("url", getenv("MIMIMI_WEBSITE_URL", "https://rabe.ch"), "URL of RaBe Website")
	libretimeURL := flag.String("libretime", getenv("MIMIMI_LIBRETIME_URL", "https://airtime.service.int.rabe.ch"), "URL of Libretime Instance to compare")

	durationDefault, error := strconv.Atoi(getenv("MIMIMI_DURATION", "60"))
	if error != nil {
		log.Fatal("MIMIMI_DURATION must be an integer")
	}
	duration := flag.Int("duration", durationDefault, "How far ahead to check in days")

	flag.Parse()

	// configure and run app and server
	app := NewApp(
		AppConfig{
			duration: *duration,

			websiteURL:   *websiteURL,
			libretimeURL: *libretimeURL,

			listenAddr: *listenAddr,
		},
	)
	app.Load()

	server := NewServer(app)
	server.Setup()
	log.Fatal(server.ListenAndServe())
}

func getenv(key, fallback string) string {
	value := fallback
	if v := os.Getenv(key); v != "" {
		value = v
	}
	return value
}
