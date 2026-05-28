package model

import (
	"database/sql"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UsageStatistics struct {
	Id                 int    `json:"id" gorm:"primaryKey"`
	Date               string `json:"date" gorm:"type:varchar(10);not null;index:idx_usage_statistics_date;uniqueIndex:uk_usage_statistics_date_token_model,priority:1"`
	TokenId            int    `json:"token_id" gorm:"not null;index:idx_usage_statistics_token_id;uniqueIndex:uk_usage_statistics_date_token_model,priority:2"`
	TokenName          string `json:"token_name" gorm:"type:varchar(255);not null;default:''"`
	ModelName          string `json:"model_name" gorm:"type:varchar(255);not null;index:idx_usage_statistics_model_name;uniqueIndex:uk_usage_statistics_date_token_model,priority:3"`
	TotalRequests      int    `json:"total_requests" gorm:"not null;default:0"`
	SuccessfulRequests int    `json:"successful_requests" gorm:"not null;default:0"`
	FailedRequests     int    `json:"failed_requests" gorm:"not null;default:0"`
	TotalTokens        int    `json:"total_tokens" gorm:"not null;default:0"`
	PromptTokens       int    `json:"prompt_tokens" gorm:"not null;default:0"`
	CompletionTokens   int    `json:"completion_tokens" gorm:"not null;default:0"`
	TotalQuota         int    `json:"total_quota" gorm:"not null;default:0"`
	CreatedTime        int64  `json:"created_time" gorm:"bigint;not null"`
	UpdatedTime        int64  `json:"updated_time" gorm:"bigint;not null"`
}

func (UsageStatistics) TableName() string {
	return "usage_statistics"
}

type UsageStatisticsSummary struct {
	TotalRequests      int     `json:"total_requests"`
	SuccessfulRequests int     `json:"successful_requests"`
	FailedRequests     int     `json:"failed_requests"`
	SuccessRate        float64 `json:"success_rate"`
	TotalTokens        int     `json:"total_tokens"`
	TotalQuota         int     `json:"total_quota"`
}

func RecordUsageStatistics(tokenId int, tokenName, modelName string, promptTokens, completionTokens int, quota int, isSuccess bool) error {
	if tokenId <= 0 || modelName == "" {
		return nil
	}

	totalRequests := 1
	successfulRequests := 0
	failedRequests := 0
	if isSuccess {
		successfulRequests = 1
	} else {
		failedRequests = 1
	}

	return UpsertUsageStatistics(
		time.Now().Format("2006-01-02"),
		tokenId,
		tokenName,
		modelName,
		totalRequests,
		successfulRequests,
		failedRequests,
		promptTokens+completionTokens,
		promptTokens,
		completionTokens,
		quota,
	)
}

func UpsertUsageStatistics(date string, tokenId int, tokenName, modelName string, totalRequests, successfulRequests, failedRequests int, totalTokens, promptTokens, completionTokens, totalQuota int) error {
	now := common.GetTimestamp()
	record := UsageStatistics{
		Date:               date,
		TokenId:            tokenId,
		TokenName:          tokenName,
		ModelName:          modelName,
		TotalRequests:      totalRequests,
		SuccessfulRequests: successfulRequests,
		FailedRequests:     failedRequests,
		TotalTokens:        totalTokens,
		PromptTokens:       promptTokens,
		CompletionTokens:   completionTokens,
		TotalQuota:         totalQuota,
		CreatedTime:        now,
		UpdatedTime:        now,
	}

	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "date"},
			{Name: "token_id"},
			{Name: "model_name"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"token_name":          tokenName,
			"total_requests":      gorm.Expr("total_requests + ?", totalRequests),
			"successful_requests": gorm.Expr("successful_requests + ?", successfulRequests),
			"failed_requests":     gorm.Expr("failed_requests + ?", failedRequests),
			"total_tokens":        gorm.Expr("total_tokens + ?", totalTokens),
			"prompt_tokens":       gorm.Expr("prompt_tokens + ?", promptTokens),
			"completion_tokens":   gorm.Expr("completion_tokens + ?", completionTokens),
			"total_quota":         gorm.Expr("total_quota + ?", totalQuota),
			"updated_time":        now,
		}),
	}).Create(&record).Error
}

func GetUsageStatistics(startDate, endDate string, tokenId int, modelName string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	var statistics []*UsageStatistics
	var total int64

	query := applyUsageStatisticsFilters(DB.Model(&UsageStatistics{}), startDate, endDate, tokenId, modelName, "")
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("date DESC, token_id ASC, model_name ASC").
		Offset(offset).
		Limit(pageSize).
		Find(&statistics).Error
	return statistics, total, err
}

func GetUserUsageStatistics(userId int, startDate, endDate string, tokenId int, modelName string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	var statistics []*UsageStatistics
	var total int64

	query := applyUsageStatisticsFilters(
		DB.Table("usage_statistics").
			Joins("JOIN tokens ON usage_statistics.token_id = tokens.id").
			Where("tokens.user_id = ?", userId),
		startDate,
		endDate,
		tokenId,
		modelName,
		"usage_statistics.",
	)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Select("usage_statistics.*").
		Order("usage_statistics.date DESC, usage_statistics.token_id ASC, usage_statistics.model_name ASC").
		Offset(offset).
		Limit(pageSize).
		Find(&statistics).Error
	return statistics, total, err
}

func GetMonthlyUsageStatistics(startDate, endDate string, tokenId int, modelName string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	return getMonthlyUsageStatistics(0, startDate, endDate, tokenId, modelName, page, pageSize)
}

func GetUserMonthlyUsageStatistics(userId int, startDate, endDate string, tokenId int, modelName string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	return getMonthlyUsageStatistics(userId, startDate, endDate, tokenId, modelName, page, pageSize)
}

func GetUsageStatisticsSummary(startDate, endDate string, tokenId int, modelName string) (*UsageStatisticsSummary, error) {
	query := applyUsageStatisticsFilters(DB.Model(&UsageStatistics{}), startDate, endDate, tokenId, modelName, "")
	return scanUsageStatisticsSummary(query)
}

func GetUserUsageStatisticsSummary(userId int, startDate, endDate string, tokenId int, modelName string) (*UsageStatisticsSummary, error) {
	query := applyUsageStatisticsFilters(
		DB.Table("usage_statistics").
			Joins("JOIN tokens ON usage_statistics.token_id = tokens.id").
			Where("tokens.user_id = ?", userId),
		startDate,
		endDate,
		tokenId,
		modelName,
		"usage_statistics.",
	)
	return scanUsageStatisticsSummary(query)
}

func GetMonthlyUsageStatisticsSummary(startDate, endDate string, tokenId int, modelName string) (*UsageStatisticsSummary, error) {
	return getMonthlyUsageStatisticsSummary(0, startDate, endDate, tokenId, modelName)
}

func GetUserMonthlyUsageStatisticsSummary(userId int, startDate, endDate string, tokenId int, modelName string) (*UsageStatisticsSummary, error) {
	return getMonthlyUsageStatisticsSummary(userId, startDate, endDate, tokenId, modelName)
}

func applyUsageStatisticsFilters(query *gorm.DB, startDate, endDate string, tokenId int, modelName string, prefix string) *gorm.DB {
	if startDate != "" {
		query = query.Where(prefix+"date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where(prefix+"date <= ?", endDate)
	}
	if tokenId > 0 {
		query = query.Where(prefix+"token_id = ?", tokenId)
	}
	if modelName != "" {
		query = query.Where(prefix+"model_name LIKE ?", "%"+modelName+"%")
	}
	return query
}

func scanUsageStatisticsSummary(query *gorm.DB) (*UsageStatisticsSummary, error) {
	var result struct {
		TotalRequests      sql.NullInt64 `json:"total_requests"`
		SuccessfulRequests sql.NullInt64 `json:"successful_requests"`
		FailedRequests     sql.NullInt64 `json:"failed_requests"`
		TotalTokens        sql.NullInt64 `json:"total_tokens"`
		TotalQuota         sql.NullInt64 `json:"total_quota"`
	}
	err := query.Select(
		"SUM(total_requests) as total_requests",
		"SUM(successful_requests) as successful_requests",
		"SUM(failed_requests) as failed_requests",
		"SUM(total_tokens) as total_tokens",
		"SUM(total_quota) as total_quota",
	).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return buildUsageStatisticsSummary(result.TotalRequests, result.SuccessfulRequests, result.FailedRequests, result.TotalTokens, result.TotalQuota), nil
}

func buildUsageStatisticsSummary(totalRequests, successfulRequests, failedRequests, totalTokens, totalQuota sql.NullInt64) *UsageStatisticsSummary {
	summary := &UsageStatisticsSummary{
		TotalRequests:      int(nullInt64Value(totalRequests)),
		SuccessfulRequests: int(nullInt64Value(successfulRequests)),
		FailedRequests:     int(nullInt64Value(failedRequests)),
		TotalTokens:        int(nullInt64Value(totalTokens)),
		TotalQuota:         int(nullInt64Value(totalQuota)),
	}
	if summary.TotalRequests > 0 {
		summary.SuccessRate = float64(summary.SuccessfulRequests) / float64(summary.TotalRequests) * 100
	}
	return summary
}

func nullInt64Value(value sql.NullInt64) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func getMonthlyUsageStatistics(userId int, startDate, endDate string, tokenId int, modelName string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	conditions, params := buildMonthlyUsageStatisticsConditions(userId, startDate, endDate, tokenId, modelName)
	var statistics []*UsageStatistics
	var countResult struct {
		Count int64 `json:"count"`
	}

	countSQL := `
		SELECT COUNT(*) as count FROM (
			SELECT 1
			FROM usage_statistics
			` + monthlyUsageStatisticsJoin(userId) + `
			WHERE 1=1` + conditions + `
			GROUP BY SUBSTR(usage_statistics.date, 1, 7), usage_statistics.token_id, usage_statistics.token_name, usage_statistics.model_name
		) as grouped_data
	`
	if err := DB.Raw(countSQL, params...).Scan(&countResult).Error; err != nil {
		return nil, 0, err
	}

	querySQL := `
		SELECT
			MAX(usage_statistics.id) as id,
			SUBSTR(usage_statistics.date, 1, 7) as date,
			usage_statistics.token_id,
			usage_statistics.token_name,
			usage_statistics.model_name,
			SUM(usage_statistics.total_requests) as total_requests,
			SUM(usage_statistics.successful_requests) as successful_requests,
			SUM(usage_statistics.failed_requests) as failed_requests,
			SUM(usage_statistics.total_tokens) as total_tokens,
			SUM(usage_statistics.prompt_tokens) as prompt_tokens,
			SUM(usage_statistics.completion_tokens) as completion_tokens,
			SUM(usage_statistics.total_quota) as total_quota,
			MAX(usage_statistics.created_time) as created_time,
			MAX(usage_statistics.updated_time) as updated_time
		FROM usage_statistics
		` + monthlyUsageStatisticsJoin(userId) + `
		WHERE 1=1` + conditions + `
		GROUP BY SUBSTR(usage_statistics.date, 1, 7), usage_statistics.token_id, usage_statistics.token_name, usage_statistics.model_name
		ORDER BY date DESC, token_id ASC, model_name ASC
		LIMIT ? OFFSET ?
	`
	offset := (page - 1) * pageSize
	queryParams := append(append([]interface{}{}, params...), pageSize, offset)
	err := DB.Raw(querySQL, queryParams...).Scan(&statistics).Error
	return statistics, countResult.Count, err
}

func getMonthlyUsageStatisticsSummary(userId int, startDate, endDate string, tokenId int, modelName string) (*UsageStatisticsSummary, error) {
	conditions, params := buildMonthlyUsageStatisticsConditions(userId, startDate, endDate, tokenId, modelName)
	var result struct {
		TotalRequests      sql.NullInt64 `json:"total_requests"`
		SuccessfulRequests sql.NullInt64 `json:"successful_requests"`
		FailedRequests     sql.NullInt64 `json:"failed_requests"`
		TotalTokens        sql.NullInt64 `json:"total_tokens"`
		TotalQuota         sql.NullInt64 `json:"total_quota"`
	}

	querySQL := `
		SELECT
			SUM(total_requests) as total_requests,
			SUM(successful_requests) as successful_requests,
			SUM(failed_requests) as failed_requests,
			SUM(total_tokens) as total_tokens,
			SUM(total_quota) as total_quota
		FROM (
			SELECT
				SUM(usage_statistics.total_requests) as total_requests,
				SUM(usage_statistics.successful_requests) as successful_requests,
				SUM(usage_statistics.failed_requests) as failed_requests,
				SUM(usage_statistics.total_tokens) as total_tokens,
				SUM(usage_statistics.total_quota) as total_quota
			FROM usage_statistics
			` + monthlyUsageStatisticsJoin(userId) + `
			WHERE 1=1` + conditions + `
			GROUP BY SUBSTR(usage_statistics.date, 1, 7), usage_statistics.token_id, usage_statistics.token_name, usage_statistics.model_name
		) as grouped_data
	`
	if err := DB.Raw(querySQL, params...).Scan(&result).Error; err != nil {
		return nil, err
	}
	return buildUsageStatisticsSummary(result.TotalRequests, result.SuccessfulRequests, result.FailedRequests, result.TotalTokens, result.TotalQuota), nil
}

func buildMonthlyUsageStatisticsConditions(userId int, startDate, endDate string, tokenId int, modelName string) (string, []interface{}) {
	conditions := ""
	params := make([]interface{}, 0, 5)
	if userId > 0 {
		conditions += " AND tokens.user_id = ?"
		params = append(params, userId)
	}
	if startDate != "" {
		conditions += " AND usage_statistics.date >= ?"
		params = append(params, monthStartDate(startDate))
	}
	if endDate != "" {
		conditions += " AND usage_statistics.date <= ?"
		params = append(params, monthEndDate(endDate))
	}
	if tokenId > 0 {
		conditions += " AND usage_statistics.token_id = ?"
		params = append(params, tokenId)
	}
	if modelName != "" {
		conditions += " AND usage_statistics.model_name LIKE ?"
		params = append(params, "%"+modelName+"%")
	}
	return conditions, params
}

func monthlyUsageStatisticsJoin(userId int) string {
	if userId <= 0 {
		return ""
	}
	return "JOIN tokens ON usage_statistics.token_id = tokens.id"
}

func monthStartDate(value string) string {
	if len(value) >= 7 {
		return value[:7] + "-01"
	}
	return value
}

func monthEndDate(value string) string {
	if len(value) < 7 {
		return value
	}
	month, err := time.ParseInLocation("2006-01", value[:7], time.Local)
	if err != nil {
		return value[:7] + "-31"
	}
	return month.AddDate(0, 1, -1).Format("2006-01-02")
}
