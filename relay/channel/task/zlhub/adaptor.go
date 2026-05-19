package zlhub

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ============================
// Video generation structures
// ============================

type ContentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type requestPayload struct {
	Model         string        `json:"model"`
	Content       []ContentItem `json:"content,omitempty"`
	CallbackURL   string        `json:"callback_url,omitempty"`
	GenerateAudio *dto.BoolValue `json:"generate_audio,omitempty"`
	Ratio         string        `json:"ratio,omitempty"`
	Duration      *dto.IntValue  `json:"duration,omitempty"`
	Watermark     *dto.BoolValue `json:"watermark,omitempty"`
	Seed          *dto.IntValue  `json:"seed,omitempty"`
}

type createResponse struct {
	ID string `json:"id"`
}

type queryResponse struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Data    *taskData `json:"data,omitempty"`
}

type taskData struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL     string `json:"video_url"`
		LastFrameURL string `json:"last_frame_url"`
	} `json:"content"`
	Duration        int    `json:"duration"`
	Ratio           string `json:"ratio"`
	Resolution      string `json:"resolution"`
	Seed            int    `json:"seed"`
	GenerateAudio   bool   `json:"generate_audio"`
	Watermark       bool   `json:"watermark"`
	FramesPerSecond int    `json:"framespersecond"`
	Usage           struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Cost struct {
		Currency   string `json:"currency"`
		InputCost  string `json:"input_cost"`
		OutputCost string `json:"output_cost"`
		TotalCost  string `json:"total_cost"`
	} `json:"cost"`
	Error     interface{} `json:"error"`
	CreatedAt int64       `json:"created_at"`
	UpdatedAt int64       `json:"updated_at"`
}

// ============================
// Asset review structures
// ============================

type AssetUploadRequest struct {
	Images       []string `json:"images"`
	AssetType    string   `json:"asset_type,omitempty"`
	CallbackURL  string   `json:"callback_url,omitempty"`
}

type AssetUploadSyncResponse struct {
	Code    int               `json:"code"`
	TaskID  string            `json:"task_id"`
	Status  string            `json:"status"`
	Message string            `json:"message,omitempty"`
	Result  *AssetReviewResult `json:"result,omitempty"`
}

type AssetUploadAsyncResponse struct {
	Code    int    `json:"code"`
	TaskID  string `json:"task_id"`
	Message string `json:"message,omitempty"`
}

type AssetReviewResult struct {
	ReviewBatchID string           `json:"review_batch_id"`
	Items         []AssetReviewItem `json:"items"`
}

type AssetReviewItem struct {
	AssetID              string `json:"asset_id"`
	SourceURL             string `json:"source_url"`
	AssetURL              string `json:"asset_url"`
	DownstreamAssetID     string `json:"downstream_asset_id"`
	DownstreamFinalURL    string `json:"downstream_final_url"`
	DownstreamURLExpireAt string `json:"downstream_url_expire_at"`
	SubmitReviewStatus    int    `json:"submit_review_status"`
	AssetType             string `json:"asset_type"`
	ErrorCode             string `json:"error_code"`
	ErrorMessage          string `json:"error_message"`
	CreateTime            string `json:"createtime"`
}

type AssetTaskQueryResponse struct {
	Code         int               `json:"code"`
	TaskID       string            `json:"task_id"`
	TrackID      string            `json:"track_id"`
	Status       string            `json:"status"`
	ErrorMessage string            `json:"error_message,omitempty"`
	TotalCount   int               `json:"total_count,omitempty"`
	DoneCount    int               `json:"done_count,omitempty"`
	Result       *AssetReviewResult `json:"result,omitempty"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
	assetToken  string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey, a.assetToken = ParseAssetCredentials(info.ApiKey)
}

// SetCredentials 直接设置凭证，用于非 Relay 场景（素材审核 API）
func (a *TaskAdaptor) SetCredentials(baseURL, videoKey, assetToken string) {
	a.baseURL = baseURL
	a.apiKey = videoKey
	a.assetToken = assetToken
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/task/create", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	// X-Trace-ID: 优先从用户请求透传，否则自动生成
	traceID := c.GetHeader("X-Trace-ID")
	if traceID == "" {
		traceID, _ = common.GenerateRandomCharsKey(32)
	}
	req.Header.Set("X-Trace-ID", traceID)

	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	duration := req.Duration
	if duration <= 0 {
		if sec, err := strconv.Atoi(req.Seconds); err == nil && sec > 0 {
			duration = sec
		}
	}
	if duration <= 0 {
		duration = 5
	}
	return map[string]float64{"seconds": float64(duration)}
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	body := a.convertToRequestPayload(&req)
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}

	// 设置 callback_url
	if serverAddr := system_setting.ServerAddress; serverAddr != "" {
		body.CallbackURL = serverAddr + "/api/zlhub/callback/video"
	}

	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var createResp createResponse
	if err := common.Unmarshal(responseBody, &createResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if createResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty, body: %s", responseBody), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return createResp.ID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/v1/task/get/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	// FetchTask 由轮询系统调用，key 可能是双凭证格式
	videoKey, _ := ParseAssetCredentials(key)

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+videoKey)
	traceID, _ := common.GenerateRandomCharsKey(32)
	req.Header.Set("X-Trace-ID", traceID)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) *requestPayload {
	r := requestPayload{
		Model:   req.Model,
		Content: []ContentItem{},
	}

	if req.HasImage() {
		// 从 metadata 提取 image roles
		imageRoles := extractImageRoles(req.Metadata)
		for i, imgURL := range req.Images {
			role := "reference_image" // 默认
			if i < len(imageRoles) && imageRoles[i] != "" {
				role = imageRoles[i]
			}
			r.Content = append(r.Content, ContentItem{
				Type: "image_url",
				ImageURL: &MediaURL{
					URL: imgURL,
				},
				Role: role,
			})
		}
	}

	// 从 metadata 解析其他字段（video_url, audio_url, ratio 等）
	_ = taskcommon.UnmarshalMetadata(req.Metadata, &r)

	if req.Duration > 0 {
		r.Duration = lo.ToPtr(dto.IntValue(req.Duration))
	} else if sec, err := strconv.Atoi(req.Seconds); err == nil && sec > 0 {
		r.Duration = lo.ToPtr(dto.IntValue(sec))
	}

	// 替换 text 项为 prompt
	r.Content = lo.Reject(r.Content, func(c ContentItem, _ int) bool { return c.Type == "text" })
	r.Content = append(r.Content, ContentItem{
		Type: "text",
		Text: req.Prompt,
	})

	return &r
}

// extractImageRoles 从 metadata 中提取图片角色映射
// metadata 格式: {"image_roles": [{"index": 0, "role": "first_frame"}, ...]}
// 或者 "first_frame"/"last_frame" URL 在 metadata 中指定
func extractImageRoles(metadata map[string]interface{}) []string {
	if metadata == nil {
		return nil
	}
	rolesRaw, ok := metadata["image_roles"]
	if !ok {
		return nil
	}
	rolesSlice, ok := rolesRaw.([]interface{})
	if !ok {
		return nil
	}
	var roles []string
	for _, item := range rolesSlice {
		if m, ok := item.(map[string]interface{}); ok {
			if role, ok := m["role"].(string); ok {
				roles = append(roles, role)
			}
		}
	}
	return roles
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var qResp queryResponse
	if err := common.Unmarshal(respBody, &qResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	if qResp.Data == nil {
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
		return &taskResult, nil
	}

	data := qResp.Data

	switch data.Status {
	case "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = data.Content.VideoURL
		taskResult.CompletionTokens = data.Usage.CompletionTokens
		taskResult.TotalTokens = data.Usage.TotalTokens
		taskResult.Duration = data.Duration
	case "failed":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		if data.Error != nil {
			if errMap, ok := data.Error.(map[string]interface{}); ok {
				if msg, ok := errMap["message"].(string); ok {
					taskResult.Reason = msg
				}
			}
		}
	case "cancelled", "expired":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = "task " + data.Status
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

// AdjustBillingOnComplete 基于实际视频时长结算计费
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	actualDuration := taskResult.Duration
	if actualDuration <= 0 {
		return 0
	}

	bc := task.PrivateData.BillingContext
	if bc == nil {
		return 0
	}

	modelRatio := bc.ModelRatio
	groupRatio := bc.GroupRatio
	if modelRatio <= 0 {
		return 0
	}
	if groupRatio <= 0 {
		groupRatio = 1.0
	}

	actualQuota := int(modelRatio * float64(common.QuotaPerUnit) * groupRatio * float64(actualDuration))
	return actualQuota
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	var qResp queryResponse
	if err := common.Unmarshal(originTask.Data, &qResp); err == nil && qResp.Data != nil {
		openAIVideo.SetMetadata("url", qResp.Data.Content.VideoURL)
		if qResp.Data.Duration > 0 {
			openAIVideo.SetMetadata("duration", fmt.Sprintf("%d", qResp.Data.Duration))
		}
		if qResp.Data.Status == "failed" {
			if errMap, ok := qResp.Data.Error.(map[string]interface{}); ok {
				msg, _ := errMap["message"].(string)
				code, _ := errMap["code"].(string)
				openAIVideo.Error = &dto.OpenAIVideoError{
					Message: msg,
					Code:    code,
				}
			}
		}
	} else {
		openAIVideo.SetMetadata("url", originTask.GetResultURL())
	}

	return common.Marshal(openAIVideo)
}

// ============================
// Asset review HTTP methods
// ============================

// SubmitAssetReview 提交素材审核（同步）
func (a *TaskAdaptor) SubmitAssetReview(req *AssetUploadRequest, proxy string) (*AssetUploadSyncResponse, error) {
	body, err := common.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, AssetBaseURL+"/api/asset/upload/sync", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Access-Token", a.assetToken)
	traceID, _ := common.GenerateRandomCharsKey(32)
	httpReq.Header.Set("X-Track-Id", traceID)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result AssetUploadSyncResponse
	if err := common.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal asset upload sync response failed")
	}
	return &result, nil
}

// SubmitAssetReviewAsync 提交素材审核（异步）
func (a *TaskAdaptor) SubmitAssetReviewAsync(req *AssetUploadRequest, proxy string) (*AssetUploadAsyncResponse, error) {
	body, err := common.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, AssetBaseURL+"/api/asset/upload/async", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Access-Token", a.assetToken)
	traceID, _ := common.GenerateRandomCharsKey(32)
	httpReq.Header.Set("X-Track-Id", traceID)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result AssetUploadAsyncResponse
	if err := common.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal asset async upload response failed")
	}
	return &result, nil
}

// QueryAssetTask 查询素材审核任务结果
func (a *TaskAdaptor) QueryAssetTask(taskID string, proxy string) (*AssetTaskQueryResponse, error) {
	url := fmt.Sprintf("%s/api/task/%s", AssetBaseURL, taskID)
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Access-Token", a.assetToken)
	traceID, _ := common.GenerateRandomCharsKey(32)
	httpReq.Header.Set("X-Track-Id", traceID)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result AssetTaskQueryResponse
	if err := common.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal asset task query response failed")
	}
	return &result, nil
}

// ============================
// Video task cancel
// ============================

// CancelTask 取消或删除视频任务
// 调用 ZLHub POST /v1/task/cancel/{id}
func (a *TaskAdaptor) CancelTask(taskID string, proxy string) (*CancelTaskResponse, error) {
	uri := fmt.Sprintf("%s/v1/task/cancel/%s", a.baseURL, taskID)

	req, err := http.NewRequest(http.MethodPost, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	traceID, _ := common.GenerateRandomCharsKey(32)
	req.Header.Set("X-Trace-ID", traceID)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result CancelTaskResponse
	if err := common.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Wrap(err, "unmarshal cancel task response failed")
	}
	return &result, nil
}

// CancelTaskResponse ZLHub 取消任务响应
type CancelTaskResponse struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Data    struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"data,omitempty"`
}

// ============================
// Key parsing helpers
// ============================

// ParseAssetCredentials 从 channel key 中解析双凭证
// Key 格式: "video_api_key|asset_access_token"
// 如果没有 | 分隔符，则视频和素材审核使用同一个 key
func ParseAssetCredentials(key string) (videoKey string, assetToken string) {
	parts := strings.SplitN(key, "|", 2)
	videoKey = strings.TrimSpace(parts[0])
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		assetToken = strings.TrimSpace(parts[1])
	} else {
		assetToken = videoKey
	}
	return
}