package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetZLHubRouter(router *gin.Engine) {
	// ZLHub 原生 API — 需要 Token 认证
	// 视频创建走标准 /v1/videos 路由（含计费、任务追踪、轮询），此处仅提供查询和取消
	zlhubApiRouter := router.Group("/api/zlhub")
	zlhubApiRouter.Use(middleware.RouteTag("zlhub"), middleware.TokenAuth())
	{
		// 视频任务查询（原生透传）
		zlhubApiRouter.GET("/v1/task/get/:task_id", controller.ZLHubProxyVideoGet)
		// 视频任务取消（原生透传）
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