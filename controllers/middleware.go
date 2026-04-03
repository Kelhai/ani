package controllers

import (
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

var SessionMiddleware = middleware.KeyAuth(
	func(c *echo.Context, token string, source middleware.ExtractorSource) (bool, error) {
		tk, err := uuid.Parse(token)
		if err != nil {
			log.Println("Failed to parse UUID")
			return false, nil
		}

		session, err := authService.GetSessionByToken(tk)
		if err != nil {
			return false, nil
		}
		if time.Now().After(session.ExpiresAt) {
			return false, nil
		}

		c.Set("userId", session.UserId)
		return true, nil
	},
)

