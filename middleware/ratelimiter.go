package middleware

import (
	"time"

	"github.com/labstack/echo"
)

func RateLimiter(qps int) echo.MiddlewareFunc {
	d := time.Second / time.Duration(qps)
	ticker := time.Tick(d)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			<-ticker
			return next(c)
		}
	}
}
