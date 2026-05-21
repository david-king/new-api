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
	Type      string     `json:"type,omitempty"`
	Text      string     `json:"text,omitempty"`
	ImageURL  *MediaURL  `json:"image_url,omitempty"`
	VideoURL  *MediaURL  `json:"video_url,omitempty"`
	AudioURL  *MediaURL  `json:"audio_url,omitempty"`
	DraftTask *DraftTask `json:"draft_task,omitempty"`
	Role      string     `json:"role,omitempty"`
}

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type DraftTask struct {
	ID string `json:"id,omitempty"`
}

type requestPayload struct {
	Model                 string         `json:"model"`
	Content               []ContentItem  `json:"content,omitempty"`
	CallbackURL           string         `json:"callback_url,omitempty"`
	ReturnLastFrame       *dto.BoolValue `json:"return_last_frame,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *dto.IntValue  `json:"execution_expires_after,omitempty"`
	GenerateAudio         *dto.BoolValue `json:"generate_audio,omitempty"`
	Draft                 *dto.BoolValue `json:"draft,omitempty"`
	Tools                 any            `json:"tools,omitempty"`
	SafetyIdentifier      string         `json:"safety_identifier,omitempty"`
	Ratio                 string         `json:"ratio,omitempty"`
	Resolution            string         `json:"resolution,omitempty"`
	Duration              *dto.IntValue  `json:"duration,omitempty"`
	Frames                *dto.IntValue  `json:"frames,omitempty"`
	Watermark             *dto.BoolValue `json:"watermark,omitempty"`
	Seed                  *dto.IntValue  `json:"seed,omitempty"`
	CameraFixed           *dto.BoolValue `json:"camera_fixed,omitempty"`
}

type createResponse struct {
	ID string `json:"id"`
}

type taskCreateResponse struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Data    taskCreateData `json:"data"`
}

type taskCreateData struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Model     string `json:"model"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
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
	Duration              int    `json:"duration"`
	Frames                int    `json:"frames"`
	Ratio                 string `json:"ratio"`
	Resolution            string `json:"resolution"`
	Seed                  int    `json:"seed"`
	GenerateAudio         bool   `json:"generate_audio"`
	Watermark             bool   `json:"watermark"`
	FramesPerSecond       int    `json:"framespersecond"`
	Tools                 any    `json:"tools"`
	SafetyIdentifier      string `json:"safety_identifier"`
	Draft                 bool   `json:"draft"`
	DraftTaskID           string `json:"draft_task_id"`
	ServiceTier           string `json:"service_tier"`
	ExecutionExpiresAfter int    `json:"execution_expires_after"`
	Usage                 struct {
		CompletionTokens int            `json:"completion_tokens"`
		TotalTokens      int            `json:"total_tokens"`
		ToolUsage        map[string]int `json:"tool_usage"`
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
	Images      []string `json:"images"`
	AssetType   string   `json:"asset_type,omitempty"`
	CallbackURL string   `json:"callback_url,omitempty"`
}

type AssetUploadSyncResponse struct {
	Code    int                `json:"code"`
	TaskID  string             `json:"task_id"`
	Status  string             `json:"status"`
	Message string             `json:"message,omitempty"`
	Result  *AssetReviewResult `json:"result,omitempty"`
}

type AssetUploadAsyncResponse struct {
	Code    int    `json:"code"`
	TaskID  string `json:"task_id"`
	Message string `json:"message,omitempty"`
}

type AssetReviewResult struct {
	ReviewBatchID string            `json:"review_batch_id"`
	Items         []AssetReviewItem `json:"items"`
}

type AssetReviewItem struct {
	AssetID               string `json:"asset_id"`
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
	Code         int                `json:"code"`
	TaskID       string             `json:"task_id"`
	TrackID      string             `json:"track_id"`
	Status       string             `json:"status"`
	ErrorMessage string             `json:"error_message,omitempty"`
	TotalCount   int                `json:"total_count,omitempty"`
	DoneCount    int                `json:"done_count,omitempty"`
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

// nativeRequestBodyKey 存储在 gin.Context 中的原始 ZLHub 原生请求体
const nativeRequestBodyKey = "zlhub_native_request_body"

// nativeRequestModelKey 存储从原生请求体中提取的 model 名称
const nativeRequestModelKey = "zlhub_native_request_model"

const videoCallbackPath = "/api/task/callback/zlhub/video"

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	// 先尝试解析为 ZLHub 原生格式（包含 content 数组）
	var rawBody []byte
	if storage, err := common.GetBodyStorage(c); err == nil {
		rawBody, _ = storage.Bytes()
		storage.Seek(0, io.SeekStart)
		c.Request.Body = io.NopCloser(storage)
	}

	if len(rawBody) > 0 {
		var nativeReq map[string]interface{}
		if err := common.Unmarshal(rawBody, &nativeReq); err == nil {
			if _, hasContent := nativeReq["content"]; hasContent {
				// ZLHub 原生格式：从 content 数组提取 prompt，从顶层字段提取 model/duration
				if err := a.parseNativeRequest(c, info, nativeReq, rawBody); err != nil {
					return err
				}
				return nil
			}
		}
	}

	// 标准格式：走原有 ValidateBasicTaskRequest 逻辑
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

// parseNativeRequest 从 ZLHub 原生格式请求中提取计费信息，构造 TaskSubmitReq
func (a *TaskAdaptor) parseNativeRequest(c *gin.Context, info *relaycommon.RelayInfo, nativeReq map[string]interface{}, rawBody []byte) *dto.TaskError {
	req := relaycommon.TaskSubmitReq{}

	// 提取 model
	if model, ok := nativeReq["model"].(string); ok && model != "" {
		req.Model = model
	} else {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest)
	}

	// 从 content 数组提取 prompt（找 type=text 的条目）
	hasNonTextContent := false
	if contentArr, ok := nativeReq["content"].([]interface{}); ok {
		for _, item := range contentArr {
			if m, ok := item.(map[string]interface{}); ok {
				if m["type"] == "text" {
					if text, ok := m["text"].(string); ok {
						req.Prompt = text
						break
					}
				} else {
					hasNonTextContent = true
				}
			}
		}
	}
	if strings.TrimSpace(req.Prompt) == "" && !hasNonTextContent {
		return service.TaskErrorWrapperLocal(fmt.Errorf("content must include text or media item"), "missing_content", http.StatusBadRequest)
	}

	if callbackURL, ok := nativeReq["callback_url"].(string); ok {
		req.CallbackURL = strings.TrimSpace(callbackURL)
	}

	// 提取 duration
	if dur, ok := nativeReq["duration"]; ok {
		switch v := dur.(type) {
		case float64:
			req.Duration = int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				req.Duration = i
			}
		}
	}

	// 提取 images（从 content 数组中找 type=image_url 的条目）
	if contentArr, ok := nativeReq["content"].([]interface{}); ok {
		for _, item := range contentArr {
			if m, ok := item.(map[string]interface{}); ok {
				if m["type"] == "image_url" {
					if imgObj, ok := m["image_url"].(map[string]interface{}); ok {
						if url, ok := imgObj["url"].(string); ok {
							req.Images = append(req.Images, url)
						}
					}
				}
			}
		}
	}

	// 构建 metadata，把原生请求中的扩展字段放进去
	metadata := make(map[string]interface{})
	for k, v := range nativeReq {
		switch k {
		case "model", "content", "duration", "prompt", "callback_url":
			// 已提取，跳过
		default:
			metadata[k] = v
		}
	}
	if len(metadata) > 0 {
		req.Metadata = metadata
	}

	// 存储原生请求体和提取的 model，供 BuildRequestBody 使用
	c.Set(nativeRequestBodyKey, rawBody)
	c.Set(nativeRequestModelKey, req.Model)

	relaycommon.StoreTaskRequest(c, info, constant.TaskActionGenerate, req)
	return nil
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

	ratios := map[string]float64{}

	resolution, ratio, hasVideoInput := zlhubBillingInputs(c, req)
	if !info.PriceData.UsePrice {
		if estimatedTokens := estimateSeedanceVideoTokens(req, resolution, ratio, hasVideoInput); estimatedTokens > 0 {
			ratios["estimated_tokens"] = 2 * estimatedTokens / common.QuotaPerUnit
		}
	} else {
		duration := zlhubEstimateDurationSeconds(req)
		ratios["seconds"] = duration
	}

	if priceRatio, ok := taskcommon.Seedance2PriceRatio(info.OriginModelName, resolution, hasVideoInput); ok && priceRatio != 1 {
		ratios["seedance_price"] = priceRatio
	}

	return ratios
}

func zlhubBillingInputs(c *gin.Context, req relaycommon.TaskSubmitReq) (resolution string, ratio string, hasVideoInput bool) {
	resolution = taskcommon.MetadataString(req.Metadata, "resolution")
	ratio = taskcommon.MetadataString(req.Metadata, "ratio")
	hasVideoInput = taskcommon.HasVideoInMetadata(req.Metadata)

	if rawBody, exists := c.Get(nativeRequestBodyKey); exists {
		bodyBytes, ok := rawBody.([]byte)
		if ok && len(bodyBytes) > 0 {
			var nativeReq map[string]any
			if err := common.Unmarshal(bodyBytes, &nativeReq); err == nil {
				if v, ok := nativeReq["resolution"].(string); ok && strings.TrimSpace(v) != "" {
					resolution = strings.TrimSpace(v)
				}
				if v, ok := nativeReq["ratio"].(string); ok && strings.TrimSpace(v) != "" {
					ratio = strings.TrimSpace(v)
				}
				hasVideoInput = hasVideoInput || taskcommon.HasVideoInContent(nativeReq["content"])
			}
		}
	}

	if resolution == "" {
		resolution = strings.TrimSpace(req.Resolution)
	}
	if resolution == "" {
		resolution = strings.TrimSpace(req.Size)
	}
	if ratio == "" {
		ratio = strings.TrimSpace(req.Ratio)
	}
	if ratio == "" && looksLikeAspectRatio(req.Size) {
		ratio = strings.TrimSpace(req.Size)
	}
	return resolution, ratio, hasVideoInput
}

func estimateSeedanceVideoTokens(req relaycommon.TaskSubmitReq, resolution, ratio string, hasVideoInput bool) float64 {
	width, height := seedanceVideoDimensions(resolution, ratio)
	if width <= 0 || height <= 0 {
		return 0
	}
	frames := zlhubEstimateFrames(req)
	if hasVideoInput {
		frames += seedanceEstimateInputVideoFrames()
	}
	return float64(width*height*frames) / 1024
}

func zlhubEstimateFrames(req relaycommon.TaskSubmitReq) int {
	if req.Frames != nil && int(*req.Frames) > 0 {
		return int(*req.Frames)
	}
	if frames := zlhubIntFromAny(req.Metadata["frames"]); frames > 0 {
		return frames
	}
	return int(zlhubEstimateDurationSeconds(req) * 24)
}

func zlhubEstimateDurationSeconds(req relaycommon.TaskSubmitReq) float64 {
	duration := req.Duration
	if duration <= 0 {
		if d := zlhubIntFromAny(req.Metadata["duration"]); d > 0 {
			duration = d
		}
	}
	if duration <= 0 {
		if sec, err := strconv.Atoi(req.Seconds); err == nil && sec > 0 {
			duration = sec
		}
	}
	if duration <= 0 {
		duration = 5
	}
	return float64(duration)
}

func zlhubIntFromAny(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

func seedanceEstimateInputVideoFrames() int {
	const minInputVideoSeconds = 4
	return minInputVideoSeconds * 24
}

func seedanceVideoDimensions(resolution, ratio string) (int, int) {
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	ratio = strings.ToLower(strings.TrimSpace(ratio))
	if resolution == "" {
		resolution = "720p"
	}
	if ratio == "" || ratio == "adaptive" {
		ratio = "16:9"
	}

	dimensions := map[string]map[string][2]int{
		"480p": {
			"16:9": {864, 496},
			"4:3":  {752, 560},
			"1:1":  {640, 640},
			"3:4":  {560, 752},
			"9:16": {496, 864},
			"21:9": {992, 432},
		},
		"720p": {
			"16:9": {1280, 720},
			"4:3":  {1112, 834},
			"1:1":  {960, 960},
			"3:4":  {834, 1112},
			"9:16": {720, 1280},
			"21:9": {1470, 630},
		},
		"1080p": {
			"16:9": {1920, 1080},
			"4:3":  {1664, 1248},
			"1:1":  {1440, 1440},
			"3:4":  {1248, 1664},
			"9:16": {1080, 1920},
			"21:9": {2206, 946},
		},
	}
	if resolution == "480" {
		resolution = "480p"
	}
	if resolution == "720" {
		resolution = "720p"
	}
	if resolution == "1080" {
		resolution = "1080p"
	}
	byRatio, ok := dimensions[resolution]
	if !ok {
		byRatio = dimensions["720p"]
	}
	d, ok := byRatio[ratio]
	if !ok {
		d = byRatio["16:9"]
	}
	return d[0], d[1]
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	// 如果有原生请求体，直接使用（加 callback_url）
	if rawBody, exists := c.Get(nativeRequestBodyKey); exists {
		bodyBytes, ok := rawBody.([]byte)
		if ok && len(bodyBytes) > 0 {
			var nativeReq map[string]interface{}
			if err := common.Unmarshal(bodyBytes, &nativeReq); err != nil {
				return nil, errors.Wrap(err, "unmarshal native request body failed")
			}

			// 模型映射
			if info.IsModelMapped {
				nativeReq["model"] = info.UpstreamModelName
			} else if model, ok := c.Get(nativeRequestModelKey); ok {
				info.UpstreamModelName = model.(string)
			}

			// 上游只接收平台内部回调地址；用户 callback_url 存在本地任务 private_data 中。
			delete(nativeReq, "callback_url")
			if serverAddr := system_setting.ServerAddress; serverAddr != "" {
				nativeReq["callback_url"] = serverAddr + videoCallbackPath
			}

			data, err := common.Marshal(nativeReq)
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(data), nil
		}
	}

	// 标准格式：从 TaskSubmitReq 转换为 ZLHub 原生格式
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

	// 上游只接收平台内部回调地址；用户 callback_url 存在本地任务 private_data 中。
	if serverAddr := system_setting.ServerAddress; serverAddr != "" {
		body.CallbackURL = serverAddr + videoCallbackPath
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

	createdAt := time.Now().Unix()
	if isTaskCreateEndpoint(c) {
		c.JSON(http.StatusOK, taskCreateResponse{
			Code:    "success",
			Message: "",
			Data: taskCreateData{
				ID:        info.PublicTaskID,
				TaskID:    info.PublicTaskID,
				Model:     info.OriginModelName,
				Status:    dto.VideoStatusQueued,
				CreatedAt: createdAt,
			},
		})
	} else {
		ov := dto.NewOpenAIVideo()
		ov.ID = info.PublicTaskID
		ov.TaskID = info.PublicTaskID
		ov.CreatedAt = createdAt
		ov.Model = info.OriginModelName

		c.JSON(http.StatusOK, ov)
	}
	return createResp.ID, responseBody, nil
}

func isTaskCreateEndpoint(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	return strings.HasPrefix(c.Request.URL.Path, "/v1/task/create")
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
	if r.Resolution == "" {
		r.Resolution = strings.TrimSpace(req.Resolution)
	}
	if r.Ratio == "" && strings.TrimSpace(req.Ratio) != "" {
		r.Ratio = strings.TrimSpace(req.Ratio)
	}
	if r.Ratio == "" && looksLikeAspectRatio(req.Size) {
		r.Ratio = strings.TrimSpace(req.Size)
	}
	if r.ReturnLastFrame == nil {
		r.ReturnLastFrame = req.ReturnLastFrame
	}
	if r.ServiceTier == "" {
		r.ServiceTier = strings.TrimSpace(req.ServiceTier)
	}
	if r.ExecutionExpiresAfter == nil {
		r.ExecutionExpiresAfter = req.ExecutionExpiresAfter
	}
	if r.GenerateAudio == nil {
		r.GenerateAudio = req.GenerateAudio
	}
	if r.Draft == nil {
		r.Draft = req.Draft
	}
	if r.Tools == nil {
		r.Tools = req.Tools
	}
	if r.SafetyIdentifier == "" {
		r.SafetyIdentifier = strings.TrimSpace(req.SafetyIdentifier)
	}
	if r.Frames == nil {
		r.Frames = req.Frames
	}
	if r.Watermark == nil {
		r.Watermark = req.Watermark
	}
	if r.Seed == nil {
		r.Seed = req.Seed
	}
	if r.CameraFixed == nil {
		r.CameraFixed = req.CameraFixed
	}

	if r.Frames != nil {
		r.Duration = nil
	}
	if r.Duration != nil && int(*r.Duration) <= 0 {
		r.Duration = nil
	}
	if r.Frames != nil && int(*r.Frames) <= 0 {
		r.Frames = nil
	}

	if r.Duration == nil && r.Frames == nil {
		if req.Duration > 0 {
			r.Duration = lo.ToPtr(dto.IntValue(req.Duration))
		} else if sec, err := strconv.Atoi(req.Seconds); err == nil && sec > 0 {
			r.Duration = lo.ToPtr(dto.IntValue(sec))
		}
	}

	// 替换 text 项为 prompt
	r.Content = lo.Reject(r.Content, func(c ContentItem, _ int) bool { return c.Type == "text" })
	r.Content = append(r.Content, ContentItem{
		Type: "text",
		Text: req.Prompt,
	})

	return &r
}

func looksLikeAspectRatio(size string) bool {
	parts := strings.SplitN(strings.TrimSpace(size), ":", 2)
	if len(parts) != 2 {
		return false
	}
	width, errWidth := strconv.Atoi(parts[0])
	height, errHeight := strconv.Atoi(parts[1])
	return errWidth == nil && errHeight == nil && width > 0 && height > 0
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
				if idx, ok := imageRoleIndex(m["index"]); ok {
					for len(roles) <= idx {
						roles = append(roles, "")
					}
					roles[idx] = role
					continue
				}
				roles = append(roles, role)
			}
		}
	}
	return roles
}

func imageRoleIndex(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, n >= 0
	case int64:
		return int(n), n >= 0
	case float64:
		idx := int(n)
		return idx, n >= 0 && n == float64(idx)
	case string:
		idx, err := strconv.Atoi(n)
		return idx, err == nil && idx >= 0
	default:
		return 0, false
	}
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var qResp queryResponse
	if err := common.Unmarshal(respBody, &qResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	if qResp.Code != "" && qResp.Code != "success" {
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = qResp.Message
		if taskResult.Reason == "" {
			taskResult.Reason = "zlhub task query failed: " + qResp.Code
		}
		return &taskResult, nil
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

// AdjustBillingOnComplete 在上游没有返回 token usage 时，基于实际视频时长兜底结算（按倍率模式）。
// 按次计费（PerCallBilling）的任务不会走到这里，在 settleTaskBillingOnComplete 的第 0 步就跳过了。
// 返回了 token usage 的任务优先走通用 token 重算，避免再乘一次 duration。
// 返回 0 表示不做 adaptor 调整，由上层 decide 走 token 重算或保持预扣。
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if taskResult.TotalTokens > 0 || taskResult.CompletionTokens > 0 {
		return 0
	}

	bc := task.PrivateData.BillingContext
	if bc == nil {
		return 0
	}

	// 按次计费：直接使用预扣额度，不做调整
	if bc.PerCallBilling {
		return 0
	}

	actualDuration := taskResult.Duration
	if actualDuration <= 0 {
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
	if actualQuota <= 0 {
		return 0
	}
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
		applyZLHubVideoResult(openAIVideo, qResp.Data)
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

func applyZLHubVideoResult(openAIVideo *dto.OpenAIVideo, data *taskData) {
	if data == nil {
		return
	}
	openAIVideo.SetMetadata("upstream_task_id", data.ID)
	openAIVideo.SetMetadata("url", data.Content.VideoURL)
	if data.Duration > 0 {
		duration := strconv.Itoa(data.Duration)
		openAIVideo.Seconds = duration
		openAIVideo.SetMetadata("duration", duration)
	}
	if data.Ratio != "" {
		openAIVideo.Size = data.Ratio
		openAIVideo.SetMetadata("ratio", data.Ratio)
	}
	if data.Resolution != "" {
		openAIVideo.SetMetadata("resolution", data.Resolution)
	}
	if data.FramesPerSecond > 0 {
		openAIVideo.SetMetadata("framespersecond", data.FramesPerSecond)
	}
	if data.Seed != 0 {
		openAIVideo.SetMetadata("seed", data.Seed)
	}
	openAIVideo.SetMetadata("generate_audio", data.GenerateAudio)
	if data.Cost.TotalCost != "" || data.Cost.OutputCost != "" || data.Cost.InputCost != "" {
		openAIVideo.SetMetadata("cost", data.Cost)
	}
	if data.Usage.CompletionTokens > 0 || data.Usage.TotalTokens > 0 {
		totalTokens := data.Usage.TotalTokens
		if totalTokens <= 0 {
			totalTokens = data.Usage.CompletionTokens
		}
		openAIVideo.Usage = &dto.Usage{
			CompletionTokens: data.Usage.CompletionTokens,
			TotalTokens:      totalTokens,
		}
	}
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
	httpReq.Header.Set("X-Track-Id", newAssetTrackID())

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
	httpReq.Header.Set("X-Track-Id", newAssetTrackID())

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
	httpReq.Header.Set("X-Track-Id", newAssetTrackID())

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

func newAssetTrackID() string {
	return common.GetUUID()
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
