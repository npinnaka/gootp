package main

import (
	context "context"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"io/fs"

	"gootp/handler"

	redis "github.com/redis/go-redis/v9"
)

// Embed Swagger UI assets (served at /swagger/)
//
//go:embed swagger/*
var swaggerAssets embed.FS

const (
	defaultRedisAddr = "localhost:6379"
)

func main() {
	addr := getenv("ADDR", ":8080")
	redisAddr := getenv("REDIS_ADDR", defaultRedisAddr)
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := getenvInt("REDIS_DB", 0)

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis at %s: %v", redisAddr, err)
	}

	s := handler.New(rdb)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	http.HandleFunc("/generate", s.HandleGenerate)
	http.HandleFunc("/validate", s.HandleValidate)

	// Swagger UI and spec
	if sub, err := fs.Sub(swaggerAssets, "swagger"); err == nil {
		http.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(sub))))
	} else {
		log.Printf("warning: failed to init swagger assets: %v", err)
	}
	http.HandleFunc("/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := swaggerAssets.ReadFile("swagger/swagger.json")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	base := publicBaseURL(addr)
	log.Printf("Swagger UI available at: %s/swagger/", base)
	log.Printf("Swagger spec available at: %s/swagger.json", base)
	log.Printf("server listening on %s (redis=%s)", addr, redisAddr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func publicBaseURL(addr string) string {
	// Expect forms like ":8080", "0.0.0.0:8080", "127.0.0.1:8080", "[::]:8080"
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// Fallback: if addr lacks port, default to it as is
		if strings.HasPrefix(addr, ":") {
			port = strings.TrimPrefix(addr, ":")
			host = ""
		} else {
			return fmt.Sprintf("http://%s", addr)
		}
	}
	// Map empty, wildcard, or unspecified hosts to localhost for user-friendly URL
	switch strings.TrimSpace(strings.ToLower(host)) {
	case "", "0.0.0.0", "::", "[::]":
		host = "localhost"
	}
	return fmt.Sprintf("http://%s", net.JoinHostPort(host, port))
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
