//go:build !dev

package main

import "github.com/gin-gonic/gin"

// registerSwagger 在非 dev 构建中为空实现，避免引入 swagger 相关依赖。
func registerSwagger(r *gin.Engine, isDev bool) {
	// no-op
}
