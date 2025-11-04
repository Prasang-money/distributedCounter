package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Prasang-money/distributedCounter/internal/counter"
	"github.com/gin-gonic/gin"
)

type Server struct {
	counter    *counter.Counter
	httpServer *http.Server
}

func NewServer(port string, counter *counter.Counter) *Server {
	server := Server{
		counter: counter,
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: SetupRoutes(),
	}
	server.httpServer = httpServer
	return &server
}

// Start starts the server.
func (s *Server) Start() error {

	return s.httpServer.ListenAndServe()
}

// Stop stops the server.
func (s *Server) Stop() error {

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctxShutDown)
}

func SetupRoutes() *gin.Engine {
	r := gin.Default()
	// Define your routes here
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
	return r
}
