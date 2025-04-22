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
	"time"
	"strings"
	"github.com/SIB-rennes/traefik-wopisrc-plugin/internal/redis"
)

// Config the plugin configuration.
type Config struct {
	CookieName string `json:"cookieName,omitempty"`
	RedisAddr  string `json:"redisAddr,omitempty"`
	RedisDB uint `json:"redisDb,omitempty" yaml:"redisDb,omitempty"`
	// RedisPassword holds the password used to AUTH against a redis server, if it
	// is protected by a AUTH
	// if you dont want to put the password in clear text in the config definition
	RedisPassword string `json:"redisPassword,omitempty" yaml:"redisPassword,omitempty"`
	// ConnectionTimeout is the read and write connection timeout to redis.
	// By default it is 2 seconds
	RedisConnectionTimeout int64 `json:"redisConnectionTimeout,omitempty" yaml:"redisConnectionTimeout,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		CookieName: "traefik_collabora_sticky",
		RedisAddr: "redis:6379",
	}
}

type WOPISrcSticky struct {
	next   http.Handler
	Config *Config
	name   string
	redisClient redis.Client
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
			log.Printf("Set cookie For woisrc %s : %s", wopiSrc, newCookies)
			req.Header.Set("Cookie", newCookies)
		} else {
			req.Header.Del("Cookie")
		}
		log.Printf("Removed existing sticky cookie for custom WOPISrc: %s", wopiSrc)
	}
	

	hash := md5.Sum([]byte(wopiSrc))
	hashStr := hex.EncodeToString(hash[:])

	cookieValue, err := c.redisClient.GetKey(hashStr)
	log.Printf("[Redis] Search in redis for %s return %s\n", hashStr, cookieValue)
	if err == nil && cookieValue != "" {
		log.Printf("[Redis] Found cookie for %s: %s\n", hashStr, cookieValue)
		req.AddCookie(&http.Cookie{
			Name:     c.Config.CookieName,
			Value:    cookieValue,
			Path:     "/",
			HttpOnly: true,
		})
	}

	rec := &responseRecorder{
		ResponseWriter: rw,
		header:         http.Header{},
		body:           &bytes.Buffer{},
	}

	for _, ck := range req.Cookies() {
		log.Printf("Cookies envoyés au backend - %s = %s", ck.Name, ck.Value)
	}

	c.next.ServeHTTP(rec, req)
	rw.WriteHeader(rec.statusCode)

	fmt.Printf("resp cookie: %v, len: %v \n", rec.cookies, len(rec.cookies))
	for _, cookie := range rec.cookies {
		if cookie.Name == c.Config.CookieName {
			fmt.Printf("[Redis] update cookie, wopi: %s, cookie: %s\n", hashStr, cookie.Value)
			_ = c.redisClient.Set(hashStr, cookie.Value, 30*time.Minute)
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

	if config.RedisAddr == "" {
		config.RedisAddr = "redis:6379"
	}
	
	if config.RedisConnectionTimeout < 1 {
		config.RedisConnectionTimeout = 2
	}

	client, err := redis.NewClient(
		config.RedisAddr,
		config.RedisDB,
		config.RedisPassword,
		time.Duration(config.RedisConnectionTimeout)*time.Second,
	)

	if err != nil {
		return nil, fmt.Errorf("unable to create redis client: %v", err)
	}

	return &WOPISrcSticky{
		Config: config,
		next:   next,
		name:   name,
		redisClient: client,
	}, nil
}