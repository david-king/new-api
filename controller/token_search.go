package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

type TokenSearchRequest struct {
	Token string `json:"token" binding:"required"`
}

type TokenInfoResponse struct {
	TokenName      string  `json:"token_name"`
	RemainQuota    int     `json:"remain_quota"`
	UsedQuota      int     `json:"used_quota"`
	UnlimitedQuota bool    `json:"unlimited_quota"`
	ExpiredTime    int64   `json:"expired_time"`
	Status         int     `json:"status"`
	ModelRatio     float64 `json:"model_ratio"`
}

func normalizeTokenSearchKey(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	token = strings.TrimPrefix(token, "sk-")
	if parts := strings.Split(token, "-"); len(parts) > 0 {
		token = parts[0]
	}
	return token
}

func SearchTokenByToken(c *gin.Context) {
	var req TokenSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	token, err := model.GetTokenByKey(normalizeTokenSearchKey(req.Token), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	modelRatio, _, _ := ratio_setting.GetModelRatio("gpt-3.5-turbo")
	response := TokenInfoResponse{
		TokenName:      token.Name,
		RemainQuota:    token.RemainQuota,
		UsedQuota:      token.UsedQuota,
		UnlimitedQuota: token.UnlimitedQuota,
		ExpiredTime:    token.ExpiredTime,
		Status:         token.Status,
		ModelRatio:     modelRatio,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    response,
	})
}
