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
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/zlhub"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// ============================
// 视频任务查询/取消（原生透传，不含计费）
// 视频创建走标准 /v1/videos 路由，由 relay 系统处理计费和任务追踪
// ============================

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

// CancelZLHubTask 通过 new-api 本地 task_id 取消 ZLHub 视频任务
// POST /v1/task/cancel/:task_id
func CancelZLHubTask(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_request",
			"message": "task_id 不能为空",
		})
		return
	}

	task, exist, err := model.GetByTaskId(c.GetInt("id"), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "get_task_failed",
			"message": err.Error(),
		})
		return
	}
	if !exist {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "task_not_exist",
			"message": "任务不存在",
		})
		return
	}

	if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
		c.JSON(http.StatusOK, gin.H{
			"code":    "success",
			"message": "",
			"data": gin.H{
				"id":     task.TaskID,
				"status": mapTaskStatusForZLHubResponse(task.Status),
			},
		})
		return
	}

	ch, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "get_channel_failed",
			"message": err.Error(),
		})
		return
	}
	if ch.Type != constant.ChannelTypeZLHub {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_channel_type",
			"message": "该取消接口仅支持 ZLHub 任务",
		})
		return
	}

	upstreamTaskID := task.GetUpstreamTaskID()
	if upstreamTaskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "missing_upstream_task_id",
			"message": "上游任务 ID 为空",
		})
		return
	}

	adaptor := &zlhub.TaskAdaptor{}
	videoKey, assetToken := zlhub.ParseAssetCredentials(ch.Key)
	baseURL := ch.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeZLHub]
	}
	adaptor.SetCredentials(baseURL, videoKey, assetToken)
	cancelResp, err := adaptor.CancelTask(upstreamTaskID, ch.GetSetting().Proxy)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    "cancel_task_failed",
			"message": err.Error(),
		})
		return
	}
	if cancelResp.Code != "" && cancelResp.Code != "success" {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    cancelResp.Code,
			"message": cancelResp.Message,
			"data":    cancelResp.Data,
		})
		return
	}

	oldStatus := task.Status
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.FailReason = "task cancelled"
	won, err := task.UpdateWithStatus(oldStatus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "update_task_failed",
			"message": "上游任务已取消，但本地任务状态更新失败: " + err.Error(),
		})
		return
	}
	if won && task.Quota != 0 {
		service.RefundTaskQuota(c.Request.Context(), task, task.FailReason)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "success",
		"message": "",
		"data": gin.H{
			"id":     task.TaskID,
			"status": "cancelled",
		},
	})
}

func mapTaskStatusForZLHubResponse(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusSuccess:
		return "succeeded"
	case model.TaskStatusFailure:
		return "failed"
	case model.TaskStatusQueued, model.TaskStatusSubmitted:
		return "queued"
	default:
		return "running"
	}
}

// proxyVideoRequest 通用视频 API 透传方法
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

// ============================
// 素材审核接口
// ============================

// ZLHubAssetUploadRequest 素材审核提交请求
type ZLHubAssetUploadRequest struct {
	Images    []string `json:"images" binding:"required,max=50"`
	AssetType string   `json:"asset_type,omitempty"` // Image / Video / Audio, 默认 Image
	Async     bool     `json:"async,omitempty"`      // true=异步, false=同步(默认)
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
	submitAssetReview(c, nil)
}

// SubmitAssetReviewSync 同步提交素材审核
// POST /v1/asset/upload/sync
func SubmitAssetReviewSync(c *gin.Context) {
	async := false
	submitAssetReview(c, &async)
}

// SubmitAssetReviewAsync 异步提交素材审核
// POST /v1/asset/upload/async
func SubmitAssetReviewAsync(c *gin.Context) {
	async := true
	submitAssetReview(c, &async)
}

func submitAssetReview(c *gin.Context, asyncOverride *bool) {
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

	asyncMode := req.Async
	if asyncOverride != nil {
		asyncMode = *asyncOverride
	}

	common.SysLog(fmt.Sprintf("ZLHub asset upload: type=%s async=%v count=%d [channel:%d]", assetReq.AssetType, asyncMode, len(req.Images), ch.Id))

	if asyncMode {
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
