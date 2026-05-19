package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetZLHubRouter(router *gin.Engine) {
	// ZLHub 原生 API 透传 — 需要 Token 认证
	// 视频生成：请求体和响应体原样透传，只加认证和追踪头
	zlhubApiRouter := router.Group("/api/zlhub")
	zlhubApiRouter.Use(middleware.RouteTag("zlhub"), middleware.TokenAuth())
	{
		// 视频生成（原生透传，与上游 API 格式完全一致）
		zlhubApiRouter.POST("/v1/task/create", controller.ZLHubProxyVideoCreate)
		zlhubApiRouter.GET("/v1/task/get/:task_id", controller.ZLHubProxyVideoGet)
		zlhubApiRouter.POST("/v1/task/cancel/:task_id", controller.ZLHubProxyVideoCancel)

		// 素材审核
		zlhubApiRouter.POST("/asset/upload", controller.SubmitAssetReview)
		zlhubApiRouter.GET("/asset/task/:task_id", controller.QueryAssetTask)
	}

	// ZLHub 回调端点 — 无需认证，ZLHub 服务器调用
	zlhubCallbackRouter := router.Group("/api/zlhub")
	zlhubCallbackRouter.Use(middleware.RouteTag("zlhub"))
	{
		zlhubCallbackRouter.POST("/asset/callback", controller.AssetReviewCallback)
		zlhubCallbackRouter.POST("/callback/video", controller.ZLHubVideoCallback)
	}
}