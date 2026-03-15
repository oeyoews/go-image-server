package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIResponse 统一 RESTful 响应体
type APIResponse struct {
	Code    int         `json:"code" example:"200"`
	Message string      `json:"message" example:"success"`
	Data    interface{} `json:"data"`
}

// APIError 错误响应
type APIError struct {
	Code    int    `json:"code" example:"400"`
	Message string `json:"message" example:"missing path"`
	Data    any    `json:"data"`
}

func respOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "success",
		Data:    data,
	})
}

func respCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{
		Code:    http.StatusCreated,
		Message: "success",
		Data:    data,
	})
}

func respBadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, APIResponse{
		Code:    http.StatusBadRequest,
		Message: message,
		Data:    nil,
	})
}

func respNotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, APIResponse{
		Code:    http.StatusNotFound,
		Message: message,
		Data:    nil,
	})
}

func respServerError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, APIResponse{
		Code:    http.StatusInternalServerError,
		Message: message,
		Data:    nil,
	})
}
