package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/zlhub"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// ============================
// 视频生成原生透传接口（带任务追踪和计费）
// ============================

// ZLHubProxyVideoCreate 创建视频任务（原生透传 + 任务追踪 + 计费）
// POST /api/zlhub/v1/task/create
func ZLHubProxyVideoCreate(c *gin.Context) {
	ch, err := getZLHubChannel(c)
	if err != nil || ch == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "error",
			"message": fmt.Sprintf("未找到可用的 ZLHub 渠道: %v", err),
		})
		return
	}

	// 读取请求体
	var reqBody []byte
	if c.Request.Body != nil {
		reqBody, _ = io.ReadAll(c.Request.Body)
	}

	// 从请求体解析 model 和 duration 用于计费
	modelName := "doubao-seedance-2.0"
	duration := 5
	var reqMap map[string]interface{}
	if len(reqBody) > 0 {
		if err := common.Unmarshal(reqBody, &reqMap); err == nil {
			if m, ok := reqMap["model"].(string); ok && m != "" {
				modelName = m
			}
			if d, ok := reqMap["duration"].(float64); ok && d > 0 {
				duration = int(d)
			}
		}
	}

	// 获取用户信息（由 TokenAuth 中间件设置）
	userId := c.GetInt("id")
	tokenId := c.GetInt("token_id")
	userGroup := c.GetString("group")

	// 计算预扣费额度
	modelRatio, _, _ := ratio_setting.GetModelRatio(modelName)
	groupRatio := ratio_setting.GetGroupRatio(userGroup)
	preConsumedQuota := int(modelRatio * float64(duration) * common.QuotaPerUnit * groupRatio)
	if preConsumedQuota <= 0 {
		preConsumedQuota = int(modelRatio * 5 * common.QuotaPerUnit * groupRatio)
	}

	// 预扣费
	userQuota, err := model.GetUserQuota(userId, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "error",
			"message": "查询用户额度失败",
		})
		return
	}
	if userQuota < preConsumedQuota {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "error",
			"message": fmt.Sprintf("用户额度不足，剩余: %s，需要: %s", logger.FormatQuota(userQuota), logger.FormatQuota(preConsumedQuota)),
		})
		return
	}
	if err := model.DecreaseUserQuota(userId, preConsumedQuota, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "error",
			"message": "预扣费失败: " + err.Error(),
		})
		return
	}

	// 扣除令牌额度（如果有）
	tokenUnlimited := c.GetBool("token_unlimited_quota")
	if !tokenUnlimited && tokenId > 0 {
		_ = model.DecreaseTokenQuota(tokenId, c.GetString("token_key"), preConsumedQuota)
	}

	// 生成公开 task ID
	publicTaskID := model.GenerateTaskID()

	// 转发到上游
	videoKey, _ := zlhub.ParseAssetCredentials(ch.Key)
	baseURL := ch.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeZLHub]
	}
	upstreamURL := baseURL + "/v1/task/create"

	traceID := c.GetHeader("X-Trace-ID")
	if traceID == "" {
		traceID, _ = common.GenerateRandomCharsKey(32)
	}

	common.SysLog(fmt.Sprintf("ZLHub video create: [trace:%s] [channel:%d] [user:%d] model=%s duration=%d preConsumed=%d",
		traceID, ch.Id, userId, modelName, duration, preConsumedQuota))

	httpReq, _ := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+videoKey)
	httpReq.Header.Set("X-Trace-ID", traceID)

	proxy := ch.GetSetting().Proxy
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		model.IncreaseUserQuota(userId, preConsumedQuota, false)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "error",
			"message": "创建 HTTP 客户端失败: " + err.Error(),
		})
		return
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		model.IncreaseUserQuota(userId, preConsumedQuota, false)
		common.SysLog(fmt.Sprintf("ZLHub video create error: [trace:%s] %s", traceID, err.Error()))
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    "error",
			"message": "请求 ZLHub 上游失败: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		model.IncreaseUserQuota(userId, preConsumedQuota, false)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "error",
			"message": "读取上游响应失败: " + err.Error(),
		})
		return
	}

	// 上游返回非 200，退还预扣费
	if resp.StatusCode != http.StatusOK {
		model.IncreaseUserQuota(userId, preConsumedQuota, false)
		common.SysLog(fmt.Sprintf("ZLHub video create upstream error: [trace:%s] status=%d body=%s", traceID, resp.StatusCode, string(respBody)))

		for key, values := range resp.Header {
			switch key {
			case "Content-Type", "X-Trace-Id", "X-Request-Id":
				for _, v := range values {
					c.Header(key, v)
				}
			}
		}
		c.Status(resp.StatusCode)
		c.Writer.Write(respBody)
		return
	}

	// 解析上游响应获取 task ID
	var createResp struct {
		ID string `json:"id"`
	}
	if err := common.Unmarshal(respBody, &createResp); err != nil || createResp.ID == "" {
		model.IncreaseUserQuota(userId, preConsumedQuota, false)
		common.SysLog(fmt.Sprintf("ZLHub video create parse error: [trace:%s] body=%s", traceID, string(respBody)))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "error",
			"message": "解析上游响应失败: 无法获取任务ID",
		})
		return
	}

	common.SysLog(fmt.Sprintf("ZLHub video create success: [trace:%s] upstream_id=%s public_id=%s", traceID, createResp.ID, publicTaskID))

	// 创建任务记录
	platform := constant.TaskPlatform(strconv.Itoa(ch.Type))
	task := &model.Task{
		TaskID:     publicTaskID,
		UserId:     userId,
		Group:      userGroup,
		ChannelId:  ch.Id,
		Quota:      preConsumedQuota,
		Action:     constant.TaskActionGenerate,
		Status:     model.TaskStatusNotStart,
		Progress:   "0%",
		SubmitTime: time.Now().Unix(),
		Platform:   platform,
		Properties: model.Properties{
			UpstreamModelName: modelName,
			OriginModelName:   modelName,
			Input:             extractPrompt(reqMap),
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: createResp.ID,
			Key:            videoKey,
			BillingSource:  "wallet",
			TokenId:        tokenId,
			BillingContext: &model.TaskBillingContext{
				ModelRatio:      modelRatio,
				GroupRatio:      groupRatio,
				OriginModelName: modelName,
				OtherRatios:     map[string]float64{"seconds": float64(duration)},
			},
		},
	}
	task.SetData(respBody)

	if insertErr := task.Insert(); insertErr != nil {
		common.SysError("ZLHub insert task error: " + insertErr.Error())
	}

	// 返回上游响应 + public task_id
	var respWithTaskID map[string]interface{}
	if err := common.Unmarshal(respBody, &respWithTaskID); err == nil {
		respWithTaskID["task_id"] = publicTaskID
		for key, values := range resp.Header {
			switch key {
			case "Content-Type", "X-Trace-Id", "X-Request-Id":
				for _, v := range values {
					c.Header(key, v)
				}
			}
		}
		c.Status(resp.StatusCode)
		respBytes, _ := common.Marshal(respWithTaskID)
		c.Writer.Write(respBytes)
	} else {
		for key, values := range resp.Header {
			switch key {
			case "Content-Type", "X-Trace-Id", "X-Request-Id":
				for _, v := range values {
					c.Header(key, v)
				}
			}
		}
		c.Status(resp.StatusCode)
		c.Writer.Write(respBody)
	}
}

// ZLHubProxyVideoGet 查询视频任务（原生透传）
// GET /api/zlhub/v1/task/get/:task_id
func ZLHubProxyVideoGet(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "error",
			"message": "task_id 不能为空",
		})
		return
	}
	proxyVideoRequest(c, "/v1/task/get/"+taskID)
}

// ZLHubProxyVideoCancel 取消视频任务（原生透传）
// POST /api/zlhub/v1/task/cancel/:task_id
func ZLHubProxyVideoCancel(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "error",
			"message": "task_id 不能为空",
		})
		return
	}
	proxyVideoRequest(c, "/v1/task/cancel/"+taskID)
}

// proxyVideoRequest 通用视频 API 透传方法（不含任务追踪）
func proxyVideoRequest(c *gin.Context, upstreamPath string) {
	ch, err := getZLHubChannel(c)
	if err != nil || ch == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "error",
			"message": fmt.Sprintf("未找到可用的 ZLHub 渠道: %v", err),
		})
		return
	}

	videoKey, _ := zlhub.ParseAssetCredentials(ch.Key)
	baseURL := ch.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeZLHub]
	}
	upstreamURL := baseURL + upstreamPath

	var reqBody []byte
	if c.Request.Body != nil {
		reqBody, _ = io.ReadAll(c.Request.Body)
	}

	var httpReq *http.Request
	if len(reqBody) > 0 {
		httpReq, _ = http.NewRequest(c.Request.Method, upstreamURL, bytes.NewReader(reqBody))
		httpReq.ContentLength = int64(len(reqBody))
	} else {
		httpReq, _ = http.NewRequest(c.Request.Method, upstreamURL, nil)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+videoKey)

	traceID := c.GetHeader("X-Trace-ID")
	if traceID == "" {
		traceID, _ = common.GenerateRandomCharsKey(32)
	}
	httpReq.Header.Set("X-Trace-ID", traceID)

	common.SysLog(fmt.Sprintf("ZLHub video proxy: %s %s [trace:%s] [channel:%d]", c.Request.Method, upstreamPath, traceID, ch.Id))

	proxy := ch.GetSetting().Proxy
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "error",
			"message": "创建 HTTP 客户端失败: " + err.Error(),
		})
		return
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		common.SysLog(fmt.Sprintf("ZLHub video proxy error: %s %s [trace:%s] error: %s", c.Request.Method, upstreamPath, traceID, err.Error()))
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    "error",
			"message": "请求 ZLHub 上游失败: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "error",
			"message": "读取上游响应失败: " + err.Error(),
		})
		return
	}

	common.SysLog(fmt.Sprintf("ZLHub video proxy response: %s %s [trace:%s] status=%d body_len=%d", c.Request.Method, upstreamPath, traceID, resp.StatusCode, len(respBody)))

	for key, values := range resp.Header {
		switch key {
		case "Content-Type", "X-Trace-Id", "X-Request-Id":
			for _, v := range values {
				c.Header(key, v)
			}
		}
	}
	c.Status(resp.StatusCode)
	c.Writer.Write(respBody)
}

// extractPrompt 从原始请求体提取 prompt
func extractPrompt(reqMap map[string]interface{}) string {
	if reqMap == nil {
		return ""
	}
	if prompt, ok := reqMap["prompt"].(string); ok {
		return prompt
	}
	if content, ok := reqMap["content"].([]interface{}); ok {
		for _, item := range content {
			if m, ok := item.(map[string]interface{}); ok {
				if m["type"] == "text" {
					if text, ok := m["text"].(string); ok {
						return text
					}
				}
			}
		}
	}
	return ""
}

// ============================
// 素材审核接口
// ============================

// ZLHubAssetUploadRequest 素材审核提交请求
type ZLHubAssetUploadRequest struct {
	Images    []string `json:"images" binding:"required,max=50"`
	AssetType string   `json:"asset_type,omitempty"` // Image / Video / Audio, 默认 Image
	Async     bool     `json:"async,omitempty"`       // true=异步, false=同步(默认)
}

// ZLHubAssetUploadResponse 素材审核提交响应
type ZLHubAssetUploadResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// getZLHubChannel 获取 ZLHub 渠道，支持指定渠道 ID 或自动查找
func getZLHubChannel(c *gin.Context) (*model.Channel, error) {
	if channelIDStr := c.Query("channel_id"); channelIDStr != "" {
		channelID, err := strconv.Atoi(channelIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid channel_id: %s", channelIDStr)
		}
		return model.CacheGetChannel(channelID)
	}
	return model.GetChannelByType(constant.ChannelTypeZLHub)
}

// initZLHubAdaptor 初始化 ZLHub adaptor 并返回渠道信息
func initZLHubAdaptor(c *gin.Context) (*zlhub.TaskAdaptor, *model.Channel, error) {
	ch, err := getZLHubChannel(c)
	if err != nil || ch == nil {
		return nil, nil, fmt.Errorf("未找到可用的 ZLHub 渠道: %v", err)
	}

	adaptor := &zlhub.TaskAdaptor{}
	adaptor.ChannelType = constant.ChannelTypeZLHub
	videoKey, assetToken := zlhub.ParseAssetCredentials(ch.Key)
	adaptor.SetCredentials(ch.GetBaseURL(), videoKey, assetToken)
	return adaptor, ch, nil
}

// SubmitAssetReview 提交素材审核
// POST /api/zlhub/asset/upload
func SubmitAssetReview(c *gin.Context) {
	var req ZLHubAssetUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	adaptor, ch, err := initZLHubAdaptor(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	assetReq := &zlhub.AssetUploadRequest{
		Images:      req.Images,
		AssetType:   req.AssetType,
		CallbackURL: system_setting.ServerAddress + "/api/zlhub/asset/callback",
	}
	if assetReq.AssetType == "" {
		assetReq.AssetType = "Image"
	}

	proxy := ch.GetSetting().Proxy

	common.SysLog(fmt.Sprintf("ZLHub asset upload: type=%s async=%v count=%d [channel:%d]", assetReq.AssetType, req.Async, len(req.Images), ch.Id))

	if req.Async {
		result, err := adaptor.SubmitAssetReviewAsync(assetReq, proxy)
		if err != nil {
			common.SysLog(fmt.Sprintf("ZLHub asset upload error (async): %s", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "提交素材审核失败: " + err.Error(),
			})
			return
		}
		c.JSON(http.StatusAccepted, ZLHubAssetUploadResponse{
			Success: true,
			Message: "任务已受理",
			Data:    result,
		})
		return
	}

	result, err := adaptor.SubmitAssetReview(assetReq, proxy)
	if err != nil {
		common.SysLog(fmt.Sprintf("ZLHub asset upload error (sync): %s", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "提交素材审核失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ZLHubAssetUploadResponse{
		Success: true,
		Data:    result,
	})
}

// QueryAssetTask 查询素材审核任务
// GET /api/zlhub/asset/task/:task_id
func QueryAssetTask(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "task_id 不能为空",
		})
		return
	}

	adaptor, ch, err := initZLHubAdaptor(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	proxy := ch.GetSetting().Proxy
	result, err := adaptor.QueryAssetTask(taskID, proxy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询素材审核任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ZLHubAssetUploadResponse{
		Success: true,
		Data:    result,
	})
}

// ============================
// 回调端点
// ============================

// AssetReviewCallback 接收 ZLHub 素材审核回调
// POST /api/zlhub/asset/callback
func AssetReviewCallback(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
		return
	}
	common.SysLog(fmt.Sprintf("ZLHub asset review callback: %s", string(body)))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// ZLHubVideoCallback 接收 ZLHub 视频生成回调
// POST /api/zlhub/callback/video
func ZLHubVideoCallback(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": "success"})
		return
	}
	common.SysLog(fmt.Sprintf("ZLHub video callback: %s", string(body)))
	c.JSON(http.StatusOK, gin.H{"code": "success"})
}