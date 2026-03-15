//go:build dev

package main

import (
	"go-image-server/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// registerSwagger 在开发构建中注册 Swagger UI 路由。
func registerSwagger(r *gin.Engine, isDev bool) {
	if !isDev {
		return
	}

	docs.SwaggerInfo.BasePath = "/api/v1"
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

