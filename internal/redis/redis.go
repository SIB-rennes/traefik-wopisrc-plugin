package redis

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// minimum redis connection pool size
const MAX_ACTIVE = 5

type Client interface {
	Close()
	Ping() error
	Set(key, value string, expiration time.Duration) error
	GetKey(key string) (string, error)
}

type ClientImpl struct {
	mu                sync.Mutex
	conns             chan net.Conn
	addr              string
	maxActive         int
	dialTimeout       time.Duration
	auth              string
	db                int
	connectionTimeout time.Duration
}

// NewClient initializes a new redis cleint with connection pool
func NewClient(addr string, db uint, authpassword string, connectionTimeout time.Duration) (Client, error) {
	maxActive := MAX_ACTIVE

	if maxActive <= 0 {
		return nil, errors.New("maxActive must be greater than 0")
	}

	r := &ClientImpl{
		conns:             make(chan net.Conn, maxActive),
		addr:              addr,
		maxActive:         maxActive,
		dialTimeout:       connectionTimeout * 2,
		auth:              authpassword,
		connectionTimeout: connectionTimeout,
	}

	// Prepopulate the pool with connections
	for i := 0; i < maxActive; i++ {
		conn, err := r.newConn()
		if err == nil {
			r.conns <- conn
		}
	}

	return r, nil
}

func (r *ClientImpl) newConn() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", r.addr, r.dialTimeout)
	if err != nil {
		return nil, err
	}
	if r.auth != "" {
		if _, err := sendCommand(conn, r.dialTimeout, "AUTH", r.auth); err != nil {
			conn.Close()
			return nil, err
		}
	}
	if _, err := sendCommand(conn, r.dialTimeout, "SELECT", strconv.Itoa(r.db)); err != nil {
		conn.Close()
		return nil, err
	}
	
	return conn, nil
}

// Get retrieves a connection from the pool
func (r *ClientImpl) get() (net.Conn, error) {
	select {
	case conn := <-r.conns:
		return conn, nil
	default:
		conn, err := r.newConn()
		return conn, err
	}
}

// Put returns a connection back to the pool
func (r *ClientImpl) put(conn net.Conn) error {
	if conn == nil {
		return errors.New("nil connection cannot be added to the pool")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// If the pool is full, just close the connection
	if len(r.conns) >= r.maxActive {
		conn.Close()
		return nil
	}

	r.conns <- conn
	return nil
}

// Close closes all the connections in the pool
func (r *ClientImpl) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	close(r.conns)
	for conn := range r.conns {
		conn.Close()
	}
}


func sendCommand(conn net.Conn, timeout time.Duration, args ...string) (interface{}, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}

	conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte(sb.String())); err != nil {
		return nil, err
	}

	return readRESP(bufio.NewReader(conn))
}

func readRESP(r *bufio.Reader) (interface{}, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(line, "\r\n")

	switch line[0] {
	case '+': // simple string
		return line[1:], nil
	case '-': // error
		return nil, errors.New(line[1:])
	case ':': // integer
		return strconv.ParseInt(line[1:], 10, 64)
	case '$': // bulk string
		length, _ := strconv.Atoi(line[1:])
		if length == -1 {
			return nil, nil // nil bulk string
		}
		buf := make([]byte, length+2)
		if _, err := r.Read(buf); err != nil {
			return nil, err
		}
		return string(buf[:length]), nil
	case '*': // array (not used here but could be handled)
		return nil, errors.New("array response not supported")
	default:
		return nil, errors.New("unknown RESP type")
	}
}

func (r *ClientImpl) Ping() error {
	conn, err := r.get()
	if err != nil {
		return err
	}
	defer r.put(conn)

	res, err := sendCommand(conn, r.connectionTimeout, "PING")
	if err != nil {
		// let's reset the conn
		conn.Close()
		conn = nil
		return err
	}

	if res != "PONG" {
		return fmt.Errorf("unexpected PING response: %v", res)
	}
	return nil
}

func (r *ClientImpl) Set(key, value string, expiration time.Duration) error {
	conn, err := r.get()
	if err != nil {
		return err
	}
	defer r.put(conn)

	args := []string{"SET", key, value}
	if expiration > 0 {
		args = append(args, "PX", fmt.Sprintf("%d", expiration.Milliseconds()))
	}

	res, err := sendCommand(conn, r.connectionTimeout, args...)
	if err != nil {
		conn.Close()
		return err
	}
	if res != "OK" {
		return fmt.Errorf("unexpected SET response: %v", res)
	}
	return nil
}

func (r *ClientImpl) GetKey(key string) (string, error) {
	conn, err := r.get()
	log.Printf("GET KEY  REDIS %s", key)
	if err != nil {
		log.Printf("GET KEY  NIL ")
		return "", err
	}
	defer r.put(conn)

	res, err := sendCommand(conn, r.connectionTimeout, "GET", key)
	log.Printf("GET KEY RESPONSE %s ", res)

	if err != nil {
		log.Printf("GET KEY  NIL 2")

		// reset la connexion si besoin
		conn.Close()
		return "", err
	}

	if res == nil {
		return "", nil // Key not found
	}

	val, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type for GET result: %T", res)
	}
	log.Printf("GET KEY RETURN %s ", val)
	return val, nil
}

