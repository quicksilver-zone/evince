package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/labstack/echo-contrib/prometheus"
	echov4 "github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"gopkg.in/yaml.v2"
)

var GitCommit string

func main() {
	fmt.Printf("Quicksilver (evince): %s\n", GitCommit)

	// handle flags
	var filename string
	flag.StringVar(&filename, "f", "", "YAML file to parse.")
	flag.Parse()

	if filename == "" {
		fmt.Println("Please specify a config file using the -f option")
		return
	}

	e := echov4.New()
	e.Logger.SetLevel(log.DEBUG)

	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Skipper: func(c echov4.Context) bool {
			return strings.Contains(c.Path(), "metrics")
		},
	}))

	// YAML configuration
	yamlfile, err := os.ReadFile(filename)
	if err != nil {
		e.Logger.Fatalf("%v: %v", ErrReadConfigFile, err)
		return
	}

	var cfg Config
	err = yaml.Unmarshal(yamlfile, &cfg)
	if err != nil {
		e.Logger.Fatalf("%v: %v", ErrParseConfigFile, err)
		return
	}

	// risteretto cache
	ristrettoCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // Num keys to track frequency of (10M).
		MaxCost:     1 << 30, // Maximum cost of cache (1GB).
		BufferItems: 64,      // Number of keys per Get buffer.
	})
	if err != nil {
		e.Logger.Fatalf("unable to start risteretto cache: %v", err)
	}

	// quick cache service
	service := NewCacheService(e, ristrettoCache, cfg)

	// routing (see routes.go)
	service.ConfigureRoutes()

	// start server
	go func() {
		// Enable metrics middleware
		p := prometheus.NewPrometheus("echo", nil)
		p.Use(e)

		if err := e.Start(":1323"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatalf("%v: %v", ErrEchoFatal, err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server. Use a timeout
	// of 30 seconds. Use a buffered channel to avoid missing signals as
	// recommended for signal.Notify.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}

	fmt.Println("...server shutdown.")
}
