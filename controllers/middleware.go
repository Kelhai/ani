package controllers

import (
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

var SessionMiddleware = middleware.KeyAuth(
	func(c *echo.Context, token string, source middleware.ExtractorSource) (bool, error) {
		session, err := authService.GetSessionByToken(token)
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
