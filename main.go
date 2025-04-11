package traefik_wopisrc_plugin


import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
)

// Config the plugin configuration.
type Config struct {
	CacheSize  int    `json:"cacheSize,omitempty"`
	CookieName string `json:"cookieName,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		CacheSize:  1000,
		CookieName: "traefik_collabora_sticky",
	}
}

type WOPISrcSticky struct {
	next   http.Handler
	Config *Config
	name   string
	cache  map[string]string
	mu     sync.RWMutex
}

func (c *WOPISrcSticky) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	log.Printf("Incoming request: %s %s\n", req.Method, req.URL)

	// Get the WOPISrc parameter from the query string
	wopiSrc := req.URL.Query().Get("WOPISrc")

	// TODO gérer le cas si vide. Il faudrait que ca loadBalance 
	if wopiSrc == "" {
		log.Println("Cookies envoyés au backend sans wopi:")
		for _, ck := range req.Cookies() {
			log.Printf(" - %s = %s", ck.Name, ck.Value)
		}
		c.next.ServeHTTP(rw, req)
		return
	}

	if wopiSrc != "" {
		log.Printf("CHECK HEADER COOKIE")
		cookieHeader := req.Header.Get("Cookie")
		newCookies := ""
		for _, part := range strings.Split(cookieHeader, ";") {
			part = strings.TrimSpace(part)
			if !strings.HasPrefix(part, c.Config.CookieName+"=") {
				if newCookies != "" {
					newCookies += "; "
				}
				newCookies += part
			}
		}
		if newCookies != "" {
			req.Header.Set("Cookie", newCookies)
		} else {
			req.Header.Del("Cookie")
		}
		log.Printf("Removed existing sticky cookie for custom WOPISrc: %s", wopiSrc)
	}
	

	hash := md5.Sum([]byte(wopiSrc))
	hashStr := hex.EncodeToString(hash[:])

	c.mu.RLock()
	cookieVal, found := c.cache[hashStr]
	c.mu.RUnlock()

	fmt.Printf("cache found: %v, wopisrc: %v, cookie: %v \n", found, wopiSrc, cookieVal)
	if found {
		req.AddCookie(&http.Cookie{
			Name:     c.Config.CookieName,
			Value:    cookieVal,
			Path:     "/",
			HttpOnly: true,
		})
	}

	rec := &responseRecorder{
		ResponseWriter: rw,
		header:         http.Header{},
		body:           &bytes.Buffer{},
	}

	log.Println("Cookies envoyés au backend :")
	for _, ck := range req.Cookies() {
		log.Printf(" - %s = %s", ck.Name, ck.Value)
	}

	c.next.ServeHTTP(rec, req)
	rw.WriteHeader(rec.statusCode)

	fmt.Printf("resp cookie: %v, len: %v \n", rec.cookies, len(rec.cookies))
	for _, cookie := range rec.cookies {
		if cookie.Name == c.Config.CookieName {
			fmt.Printf("update cookie, wopi: %s, cookie: %s\n", hashStr, cookie.Value)
			c.mu.Lock()
			c.cache[hashStr] = cookie.Value
			c.mu.Unlock()
		}
		http.SetCookie(rw, cookie)
	}

	rw.Write(rec.body.Bytes())
}

type responseRecorder struct {
	http.ResponseWriter
	header     http.Header
	cookies    []*http.Cookie
	body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode

	// Set-Cookie 
	if cookies, ok := r.header["Set-Cookie"]; ok {
		for _, c := range cookies {
			cookie, err := ParseSetCookie(c)
			if err != nil {
				fmt.Printf("failed to parse Set-Cookie, cookie: %v, err: %v", cookie, err)
				continue
			}
			r.cookies = append(r.cookies, cookie)
		}
	}

	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.body == nil {
		r.body = &bytes.Buffer{}
	}
	r.body.Write(b)
	return len(b), nil
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	goVersion := runtime.Version()
	fmt.Printf("set up StickyHeader plugin, go version: %v, config: %v", goVersion, config)

	// cache, err := lru.New(config.CacheSize)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	// }

	return &WOPISrcSticky{
		Config: config,
		next:   next,
		name:   name,
		cache:  make(map[string]string),
	}, nil
}