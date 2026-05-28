package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type usageStatisticsPeriod string

const (
	usageStatisticsDaily   usageStatisticsPeriod = "daily"
	usageStatisticsMonthly usageStatisticsPeriod = "monthly"
)

func GetUsageStatistics(c *gin.Context) {
	getUsageStatistics(c, usageStatisticsDaily, false)
}

func GetUserUsageStatistics(c *gin.Context) {
	getUsageStatistics(c, usageStatisticsDaily, true)
}

func GetMonthlyUsageStatistics(c *gin.Context) {
	getUsageStatistics(c, usageStatisticsMonthly, false)
}

func GetUserMonthlyUsageStatistics(c *gin.Context) {
	getUsageStatistics(c, usageStatisticsMonthly, true)
}

func GetUsageStatisticsSummary(c *gin.Context) {
	getUsageStatisticsSummary(c, usageStatisticsDaily, false)
}

func GetUserUsageStatisticsSummary(c *gin.Context) {
	getUsageStatisticsSummary(c, usageStatisticsDaily, true)
}

func GetMonthlyUsageStatisticsSummary(c *gin.Context) {
	getUsageStatisticsSummary(c, usageStatisticsMonthly, false)
}

func GetUserMonthlyUsageStatisticsSummary(c *gin.Context) {
	getUsageStatisticsSummary(c, usageStatisticsMonthly, true)
}

func getUsageStatistics(c *gin.Context, period usageStatisticsPeriod, selfOnly bool) {
	userId := c.GetInt("id")
	if selfOnly && userId == 0 {
		common.ApiErrorMsg(c, "用户ID不能为空")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", c.DefaultQuery("page_size", "20")))
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	modelName := c.Query("model_name")

	startDate, endDate = defaultUsageStatisticsDates(period, startDate, endDate)
	page, pageSize = normalizeUsageStatisticsPagination(page, pageSize)

	if selfOnly && tokenId > 0 {
		if _, err := model.GetTokenByIds(tokenId, userId); err != nil {
			common.ApiErrorMsg(c, "Token不存在或无权访问")
			return
		}
	}

	var (
		items []*model.UsageStatistics
		total int64
		err   error
	)
	if period == usageStatisticsMonthly {
		if selfOnly {
			items, total, err = model.GetUserMonthlyUsageStatistics(userId, startDate, endDate, tokenId, modelName, page, pageSize)
		} else {
			items, total, err = model.GetMonthlyUsageStatistics(startDate, endDate, tokenId, modelName, page, pageSize)
		}
	} else {
		if selfOnly {
			items, total, err = model.GetUserUsageStatistics(userId, startDate, endDate, tokenId, modelName, page, pageSize)
		} else {
			items, total, err = model.GetUsageStatistics(startDate, endDate, tokenId, modelName, page, pageSize)
		}
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	summary, err := loadUsageStatisticsSummary(period, selfOnly, userId, startDate, endDate, tokenId, modelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":      items,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
			"summary":    summary,
			"start_date": startDate,
			"end_date":   endDate,
		},
	})
}

func getUsageStatisticsSummary(c *gin.Context, period usageStatisticsPeriod, selfOnly bool) {
	userId := c.GetInt("id")
	if selfOnly && userId == 0 {
		common.ApiErrorMsg(c, "用户ID不能为空")
		return
	}

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	modelName := c.Query("model_name")
	startDate, endDate = defaultUsageStatisticsDates(period, startDate, endDate)

	if selfOnly && tokenId > 0 {
		if _, err := model.GetTokenByIds(tokenId, userId); err != nil {
			common.ApiErrorMsg(c, "Token不存在或无权访问")
			return
		}
	}

	summary, err := loadUsageStatisticsSummary(period, selfOnly, userId, startDate, endDate, tokenId, modelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func loadUsageStatisticsSummary(period usageStatisticsPeriod, selfOnly bool, userId int, startDate, endDate string, tokenId int, modelName string) (*model.UsageStatisticsSummary, error) {
	if period == usageStatisticsMonthly {
		if selfOnly {
			return model.GetUserMonthlyUsageStatisticsSummary(userId, startDate, endDate, tokenId, modelName)
		}
		return model.GetMonthlyUsageStatisticsSummary(startDate, endDate, tokenId, modelName)
	}
	if selfOnly {
		return model.GetUserUsageStatisticsSummary(userId, startDate, endDate, tokenId, modelName)
	}
	return model.GetUsageStatisticsSummary(startDate, endDate, tokenId, modelName)
}

func defaultUsageStatisticsDates(period usageStatisticsPeriod, startDate, endDate string) (string, string) {
	if startDate != "" || endDate != "" {
		return startDate, endDate
	}
	now := time.Now()
	if period == usageStatisticsMonthly {
		return now.AddDate(0, -6, 0).Format("2006-01"), now.Format("2006-01")
	}
	return now.AddDate(0, 0, -7).Format("2006-01-02"), now.Format("2006-01-02")
}

func normalizeUsageStatisticsPagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
