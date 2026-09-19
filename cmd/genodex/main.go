package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex"
	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/mcp"
	"github.com/amarin/genodex/internal/store"
	"github.com/amarin/genodex/web"
)

func main() {
	port := flag.Int("p", 9000, "HTTP server port")
	webMode := flag.String("web", web.ModeProd, "web assets mode: prod (embedded) or dev (from disk)")
	flag.Parse()

	st := store.New()
	mcpServer := mcp.NewServer(st)

	docsFS := genodex.DocsFS(*webMode)

	mux := http.NewServeMux()
	mux.Handle("/mcp", server.NewStreamableHTTPServer(mcpServer))
	mux.Handle("/api/", httpapi.NewHandler(st, docsFS))
	mux.Handle("/static/", http.StripPrefix("/static/", web.StaticHandler(*webMode)))
	mux.Handle("/", web.SPAHandler(*webMode))

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("Genealogy MCP server started on %s", addr)
	log.Printf("  MCP:      http://localhost:%d/mcp", *port)
	log.Printf("  API:      http://localhost:%d/api", *port)
	log.Printf("  Web:      http://localhost:%d/ (web mode: %s)", *port, *webMode)

	// Обработка сигналов: graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown error: %v", err)
		}
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
