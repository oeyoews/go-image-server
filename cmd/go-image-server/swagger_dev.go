//go:build dev

package main

import (
	"go-image-server/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// registerSwagger 在开发构建中注册 Swagger UI 路由
func registerSwagger(r *gin.Engine, isDev bool) {
	if !isDev {
		return
	}

	// 基本信息：标题/描述/版本/前缀
	// docs.SwaggerInfo.Title = "Go Image Server API"
	// docs.SwaggerInfo.Description = "本地图片上传与管理服务的开发文档。"
	// docs.SwaggerInfo.Version = Version
	docs.SwaggerInfo.BasePath = "/api/v1"
	docs.SwaggerInfo.Schemes = []string{"http"}

	// /swagger/index.html
	r.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		// 默认折叠所有分组，只展开选中的接口
		ginSwagger.DocExpansion("none"),
		// 隐藏 Models 面板，界面更简洁
		ginSwagger.DefaultModelsExpandDepth(-1),
	))
}

