package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/weather"
)

// weatherRequest defines the expected query parameter for GET /api/weather
type weatherRequest struct {
	City string `form:"city" binding:"required"`
}

// weatherResponse mirrors the Swagger schema for a successful weather lookup
type weatherResponse struct {
	Temperature float64 `json:"temperature"`
	Humidity    int     `json:"humidity"`
	Description string  `json:"description"`
}

// WeatherHandler returns a Gin handler for GET /api/weather
func WeatherHandler(fetcher weather.Fetcher) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1) Bind and validate the 'city' query parameter
		var req weatherRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			// 400 Invalid request
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 2) Fetch current weather
		w, err := fetcher.FetchCurrent(c.Request.Context(), req.City)
		if err != nil {
			// 404 City not found (or any fetch error)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		// 3) 200 Successful operation
		c.JSON(http.StatusOK, weatherResponse{
			Temperature: w.Temp,
			Humidity:    w.Humidity,
			Description: w.Description,
		})
	}
}
