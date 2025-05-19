package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/services"
)

// subscribeRequest matches both JSON and x-www-form-urlencoded payloads
type subscribeRequest struct {
	Email     string `form:"email"     json:"email"     binding:"required,email"`
	City      string `form:"city"      json:"city"      binding:"required"`
	Frequency string `form:"frequency" json:"frequency" binding:"required,oneof=hourly daily"`
}

// SubscribeHandler handles POST /api/subscribe
func SubscribeHandler(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req subscribeRequest
		if err := c.ShouldBind(&req); err != nil {
			// 400 Invalid input
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := svc.Subscribe(c.Request.Context(), req.Email, req.City, req.Frequency); err != nil {
			// 409 Conflict when email already subscribed
			if errors.Is(err, services.ErrAlreadySubscribed) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			// 400 Other validation or business errors (including services.ErrInvalidCity)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 200 Subscription successful
		c.JSON(http.StatusOK, gin.H{"message": "Subscription successful. Confirmation email sent."})
	}
}

// ConfirmHandler handles GET /api/confirm/:token
func ConfirmHandler(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		if token == "" {
			// 400 Invalid token
			c.JSON(http.StatusBadRequest, gin.H{"error": services.ErrInvalidToken.Error()})
			return
		}

		err := svc.Confirm(c.Request.Context(), token)
		switch {
		case err == nil:
			// 200 OK
			c.JSON(http.StatusOK, gin.H{"message": "Subscription confirmed successfully"})
		case errors.Is(err, services.ErrInvalidToken):
			// 400 Invalid token
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrTokenNotFound):
			// 404 Token not found
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		}
	}
}

// UnsubscribeHandler handles GET /api/unsubscribe/:token
func UnsubscribeHandler(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		if token == "" {
			// 400 Invalid token
			c.JSON(http.StatusBadRequest, gin.H{"error": services.ErrInvalidToken.Error()})
			return
		}

		err := svc.Unsubscribe(c.Request.Context(), token)
		switch {
		case err == nil:
			// 200 OK
			c.JSON(http.StatusOK, gin.H{"message": "Unsubscribed successfully"})
		case errors.Is(err, services.ErrInvalidToken):
			// 400 Invalid token
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrTokenNotFound):
			// 404 Token not found
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		}
	}
}
