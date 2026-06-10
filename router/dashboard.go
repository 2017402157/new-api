package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetDashboardRouter(router *gin.Engine) {
	apiRouter := router.Group("/")
	apiRouter.Use(middleware.RouteTag("old_api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	apiRouter.Use(middleware.CORS())
	apiRouter.Use(middleware.MaasJwtAuth())
	{
		apiRouter.GET("/dashboard/billing/subscription", controller.GetSubscription)
		apiRouter.GET("/v1/dashboard/billing/subscription", controller.GetSubscription)
		apiRouter.GET("/dashboard/billing/usage", controller.GetUsage)
		apiRouter.GET("/v1/dashboard/billing/usage", controller.GetUsage)
	}

	// MaaS 用量统计和配额管理路由
	maasUsageRouter := router.Group("/maas")
	maasUsageRouter.Use(middleware.CORS())
	maasUsageRouter.Use(middleware.MaasJwtAuth())
	{
		maasUsageRouter.GET("/usage/stats", controller.GetMaasUsageStats)
		maasUsageRouter.POST("/quota/set", controller.SetMaasTenantQuota)
	}
}
