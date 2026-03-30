package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Kelhai/ani/controllers"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	e := echo.New()

	// middleware
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			fmt.Printf("%s %s -> %d \n", v.Method, v.URI, v.Status)
			return nil
		},
	}))

	controllers.SetupAllRoutes(e)

	// start server
	fmt.Println(`             _ 
            (_)
 _____ ____  _ 
(____ |  _ \| |
/ ___ | | | | |
\_____|_| |_|_|
               `)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	startConfig := echo.StartConfig{
		Address:         ":52971",
		GracefulTimeout: 10 * time.Second,
		HideBanner:      true,
	}

	if err := startConfig.Start(ctx, e); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}

