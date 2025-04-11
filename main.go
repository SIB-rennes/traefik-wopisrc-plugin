package traefik_wopisrc_plugin

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
)

type Config struct {
	CookieName string `json:"cookieName,omitempty"`
}

func CreateConfig() *Config {
	log.Println("Creating plugin config...")
	return &Config{
		CookieName: "traefik_collabora_sticky",
	}
}

type WOPISrcSticky struct {
	next       http.Handler
	cookieName string
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	log.Printf("Initializing plugin '%s'...\n", name)
	return &WOPISrcSticky{
		next:       next,
		cookieName: config.CookieName,
	}, nil
}

func (m *WOPISrcSticky) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Log the incoming request URL and method
	log.Printf("Incoming request: %s %s\n", req.Method, req.URL)

	// Get the WOPISrc parameter from the query string
	wopiSrc := req.URL.Query().Get("WOPISrc")
	if wopiSrc != "" {
		// Hash the WOPISrc value
		hash := md5.Sum([]byte(wopiSrc))
		hashStr := hex.EncodeToString(hash[:])

		// Log the hash of the WOPISrc
		log.Printf("WOPISrc: %s, Hash: %s\n", wopiSrc, hashStr)

		// Set the sticky cookie with the hash value
		http.SetCookie(rw, &http.Cookie{
			Name:  m.cookieName,
			Value: hashStr,
			Path:  "/",
		})

		// Log the cookie being set
		log.Printf("Set cookie: %s=%s\n", m.cookieName, hashStr)
	} else {
		log.Println("No WOPISrc parameter found in request.")
	}

	// Forward the request to the next handler
	m.next.ServeHTTP(rw, req)
}
