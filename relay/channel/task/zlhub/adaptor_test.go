package zlhub

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToRequestPayloadMapsAspectSizeToRatio(t *testing.T) {
	adaptor := &TaskAdaptor{}

	body := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:    "doubao-seedance-2.0",
		Prompt:   "test prompt",
		Size:     "16:9",
		Duration: 8,
	})

	assert.Equal(t, "16:9", body.Ratio)
	require.NotNil(t, body.Duration)
	assert.Equal(t, 8, int(*body.Duration))
}

func TestConvertToRequestPayloadKeepsMetadataRatioPriority(t *testing.T) {
	adaptor := &TaskAdaptor{}

	body := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2.0",
		Prompt: "test prompt",
		Ratio:  "1:1",
		Size:   "16:9",
		Metadata: map[string]interface{}{
			"ratio": "9:16",
		},
	})

	assert.Equal(t, "9:16", body.Ratio)
}

func TestConvertToRequestPayloadUsesTopLevelRatio(t *testing.T) {
	adaptor := &TaskAdaptor{}

	body := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2.0",
		Prompt: "test prompt",
		Ratio:  "9:16",
		Size:   "16:9",
	})

	assert.Equal(t, "9:16", body.Ratio)
}

func TestConvertToRequestPayloadIncludesResolution(t *testing.T) {
	adaptor := &TaskAdaptor{}

	body := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2.0",
		Prompt: "test prompt",
		Metadata: map[string]interface{}{
			"resolution": "1080p",
		},
	})

	assert.Equal(t, "1080p", body.Resolution)
}

func TestZLHubBillingInputsUsesTopLevelResolution(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	resolution, ratio, hasVideoInput := zlhubBillingInputs(c, relaycommon.TaskSubmitReq{
		Resolution: "1080p",
		Ratio:      "21:9",
		Size:       "720p",
	})

	assert.Equal(t, "1080p", resolution)
	assert.Equal(t, "21:9", ratio)
	assert.False(t, hasVideoInput)
}

func TestBuildRequestHeaderPassesTraceID(t *testing.T) {
	adaptor := &TaskAdaptor{apiKey: "video-key"}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/task/create", nil)
	c.Request.Header.Set("X-Trace-ID", "trace-123")
	req := httptest.NewRequest(http.MethodPost, "https://api.zlhub.cn/v1/task/create", nil)

	require.NoError(t, adaptor.BuildRequestHeader(c, req, &relaycommon.RelayInfo{}))

	assert.Equal(t, "Bearer video-key", req.Header.Get("Authorization"))
	assert.Equal(t, "trace-123", req.Header.Get("X-Trace-ID"))
}

func TestFetchTaskReturnsErrorForNon2xx(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/task/get/cgt-error", r.URL.Path)
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{}
	resp, err := adaptor.FetchTask(server.URL, "video-key|asset-token", map[string]any{"task_id": "cgt-error"}, "")

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "status 401")
}

func TestParseTaskResultRejectsUnrecognizedEmptyData(t *testing.T) {
	adaptor := &TaskAdaptor{}

	result, err := adaptor.ParseTaskResult([]byte(`{"error":{"message":"unauthorized"}}`))

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestEstimateBillingUsesOfficialVideoTokenEstimate(t *testing.T) {
	adaptor := &TaskAdaptor{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := &relaycommon.RelayInfo{
		OriginModelName: "doubao-seedance-2.0",
		PriceData: types.PriceData{
			ModelRatio: 3.6665,
			UsePrice:   false,
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	relaycommon.StoreTaskRequest(c, info, constant.TaskActionGenerate, relaycommon.TaskSubmitReq{
		Model:      "doubao-seedance-2.0",
		Prompt:     "test prompt",
		Duration:   6,
		Ratio:      "21:9",
		Resolution: "720p",
	})

	ratios := adaptor.EstimateBilling(c, info)

	require.Contains(t, ratios, "estimated_tokens")
	expectedTokens := float64(1470*630*6*24) / 1024
	assert.InDelta(t, 2*expectedTokens/common.QuotaPerUnit, ratios["estimated_tokens"], 0.000001)
	assert.NotContains(t, ratios, "seconds")
}

func TestConvertToRequestPayloadPassesOfficialOptionalParams(t *testing.T) {
	adaptor := &TaskAdaptor{}
	returnLastFrame := dto.BoolValue(true)
	generateAudio := dto.BoolValue(true)
	draft := dto.BoolValue(false)
	watermark := dto.BoolValue(false)
	cameraFixed := dto.BoolValue(false)
	frames := dto.IntValue(57)
	expiresAfter := dto.IntValue(3600)
	seed := dto.IntValue(123)

	body := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:                 "doubao-seedance-2.0",
		Prompt:                "test prompt",
		Resolution:            "720p",
		Ratio:                 "21:9",
		Duration:              6,
		Frames:                &frames,
		ReturnLastFrame:       &returnLastFrame,
		ServiceTier:           "default",
		ExecutionExpiresAfter: &expiresAfter,
		GenerateAudio:         &generateAudio,
		Draft:                 &draft,
		Watermark:             &watermark,
		Seed:                  &seed,
		CameraFixed:           &cameraFixed,
		Tools: []any{
			map[string]any{"type": "web_search"},
		},
		SafetyIdentifier: "user-hash",
		Metadata: map[string]any{
			"content": []any{
				map[string]any{
					"type":       "draft_task",
					"draft_task": map[string]any{"id": "cgt-draft"},
				},
			},
		},
	})

	assert.Equal(t, "720p", body.Resolution)
	assert.Equal(t, "21:9", body.Ratio)
	assert.Nil(t, body.Duration)
	require.NotNil(t, body.Frames)
	assert.Equal(t, 57, int(*body.Frames))
	require.NotNil(t, body.ReturnLastFrame)
	assert.True(t, bool(*body.ReturnLastFrame))
	assert.Equal(t, "default", body.ServiceTier)
	require.NotNil(t, body.ExecutionExpiresAfter)
	assert.Equal(t, 3600, int(*body.ExecutionExpiresAfter))
	require.NotNil(t, body.GenerateAudio)
	assert.True(t, bool(*body.GenerateAudio))
	require.NotNil(t, body.Draft)
	assert.False(t, bool(*body.Draft))
	require.NotNil(t, body.Watermark)
	assert.False(t, bool(*body.Watermark))
	require.NotNil(t, body.Seed)
	assert.Equal(t, 123, int(*body.Seed))
	require.NotNil(t, body.CameraFixed)
	assert.False(t, bool(*body.CameraFixed))
	assert.Equal(t, []any{map[string]any{"type": "web_search"}}, body.Tools)
	assert.Equal(t, "user-hash", body.SafetyIdentifier)
	require.Len(t, body.Content, 2)
	require.NotNil(t, body.Content[0].DraftTask)
	assert.Equal(t, "cgt-draft", body.Content[0].DraftTask.ID)
}

func TestBuildRequestBodyUsesInternalCallbackURL(t *testing.T) {
	oldServerAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://platform.example"
	defer func() { system_setting.ServerAddress = oldServerAddress }()

	adaptor := &TaskAdaptor{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/task/create", nil)
	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	relaycommon.StoreTaskRequest(c, info, constant.TaskActionGenerate, relaycommon.TaskSubmitReq{
		Model:       "doubao-seedance-2.0",
		Prompt:      "test prompt",
		CallbackURL: "https://user.example/callback",
	})

	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	bodyBytes, err := io.ReadAll(body)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, common.Unmarshal(bodyBytes, &out))
	assert.Equal(t, "https://platform.example/api/task/callback/zlhub/video", out["callback_url"])
	assert.NotEqual(t, "https://user.example/callback", out["callback_url"])
}

func TestDoResponseUsesTaskCreateEnvelope(t *testing.T) {
	adaptor := &TaskAdaptor{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/task/create", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName: "doubao-seedance-2.0",
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public",
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"cgt-upstream"}`)),
	}

	taskID, body, taskErr := adaptor.DoResponse(c, resp, info)

	require.Nil(t, taskErr)
	assert.Equal(t, "cgt-upstream", taskID)
	assert.JSONEq(t, `{"id":"cgt-upstream"}`, string(body))

	var out map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &out))
	assert.Equal(t, "success", out["code"])
	assert.Empty(t, out["message"])
	assert.NotContains(t, out, "object")
	data, ok := out["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "task_public", data["id"])
	assert.Equal(t, "task_public", data["task_id"])
	assert.Equal(t, "doubao-seedance-2.0", data["model"])
	assert.Equal(t, dto.VideoStatusQueued, data["status"])
}

func TestParseNativeRequestAllowsMediaOnlyContent(t *testing.T) {
	adaptor := &TaskAdaptor{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	rawBody := []byte(`{
		"model": "doubao-seedance-2.0",
		"content": [
			{"type": "image_url", "image_url": {"url": "https://example.com/ref.jpg"}, "role": "first_frame"}
		],
		"duration": 5
	}`)
	var nativeReq map[string]any
	require.NoError(t, common.Unmarshal(rawBody, &nativeReq))

	taskErr := adaptor.parseNativeRequest(c, info, nativeReq, rawBody)

	require.Nil(t, taskErr)
	req, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	assert.Empty(t, req.Prompt)
	assert.Equal(t, []string{"https://example.com/ref.jpg"}, req.Images)
	assert.Equal(t, constant.TaskActionGenerate, info.Action)
}

func TestParseNativeRequestStoresTextGenerateForTextOnlyContent(t *testing.T) {
	adaptor := &TaskAdaptor{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	rawBody := []byte(`{
		"model": "doubao-seedance-2.0",
		"content": [
			{"type": "text", "text": "生成一段城市夜景视频"}
		],
		"duration": 5
	}`)
	var nativeReq map[string]any
	require.NoError(t, common.Unmarshal(rawBody, &nativeReq))

	taskErr := adaptor.parseNativeRequest(c, info, nativeReq, rawBody)

	require.Nil(t, taskErr)
	req, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	assert.Equal(t, "生成一段城市夜景视频", req.Prompt)
	assert.Empty(t, req.Images)
	assert.Equal(t, constant.TaskActionTextGenerate, info.Action)
}

func TestValidateRequestAndSetActionStoresTextGenerateForPromptOnly(t *testing.T) {
	adaptor := &TaskAdaptor{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/task/create", bytes.NewReader([]byte(`{
		"model": "doubao-seedance-2.0",
		"prompt": "生成一段城市夜景视频",
		"duration": 5
	}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	taskErr := adaptor.ValidateRequestAndSetAction(c, info)

	require.Nil(t, taskErr)
	req, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	assert.Equal(t, "生成一段城市夜景视频", req.Prompt)
	assert.Equal(t, constant.TaskActionTextGenerate, info.Action)
}

func TestValidateRequestAndSetActionStoresGenerateForImageInput(t *testing.T) {
	adaptor := &TaskAdaptor{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/task/create", bytes.NewReader([]byte(`{
		"model": "doubao-seedance-2.0",
		"prompt": "让图片动起来",
		"image": "https://example.com/ref.jpg",
		"duration": 5
	}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	taskErr := adaptor.ValidateRequestAndSetAction(c, info)

	require.Nil(t, taskErr)
	req, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.com/ref.jpg"}, req.Images)
	assert.Equal(t, constant.TaskActionGenerate, info.Action)
}

func TestExtractImageRolesUsesIndex(t *testing.T) {
	roles := extractImageRoles(map[string]interface{}{
		"image_roles": []interface{}{
			map[string]interface{}{"index": float64(1), "role": "last_frame"},
			map[string]interface{}{"index": float64(0), "role": "first_frame"},
		},
	})

	assert.Equal(t, []string{"first_frame", "last_frame"}, roles)
}

func TestParseTaskResultTopLevelError(t *testing.T) {
	adaptor := &TaskAdaptor{}

	info, err := adaptor.ParseTaskResult([]byte(`{"code":"failed","message":"invalid task id"}`))

	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), info.Status)
	assert.Equal(t, "100%", info.Progress)
	assert.Equal(t, "invalid task id", info.Reason)
}

func TestAdjustBillingOnCompleteUsesActualOverEstimatedDuration(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{
		Quota: 500,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				ModelRatio: 1,
				GroupRatio: 1,
				OtherRatios: map[string]float64{
					"seconds": 5,
				},
			},
		},
	}

	actualDuration := 10
	actualQuota := adaptor.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{Duration: actualDuration})

	assert.Equal(t, int(common.QuotaPerUnit*float64(actualDuration)), actualQuota)
}

func TestAdjustBillingOnCompleteDefersToTokenBilling(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				ModelRatio: 1,
				GroupRatio: 1,
			},
		},
	}

	actualQuota := adaptor.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{
		Duration:         10,
		CompletionTokens: 100,
		TotalTokens:      100,
	})

	assert.Equal(t, 0, actualQuota)
}

func TestApplyZLHubVideoResultIncludesUsageAndCost(t *testing.T) {
	video := dto.NewOpenAIVideo()
	data := &taskData{
		ID:              "cgt-test",
		Duration:        6,
		Ratio:           "21:9",
		Resolution:      "720p",
		FramesPerSecond: 24,
		GenerateAudio:   true,
	}
	data.Content.VideoURL = "https://example.com/video.mp4"
	data.Usage.CompletionTokens = 131137
	data.Usage.TotalTokens = 131137
	data.Cost.Currency = "CNY"
	data.Cost.OutputCost = "6.0322966000"
	data.Cost.TotalCost = "6.0322966000"

	applyZLHubVideoResult(video, data)

	require.NotNil(t, video.Usage)
	assert.Equal(t, 131137, video.Usage.CompletionTokens)
	assert.Equal(t, 131137, video.Usage.TotalTokens)
	assert.Equal(t, "6", video.Seconds)
	assert.Equal(t, "21:9", video.Size)
	assert.Equal(t, "720p", video.Metadata["resolution"])
	assert.NotNil(t, video.Metadata["cost"])
}

func TestNewAssetTrackIDIsLowerHex32(t *testing.T) {
	trackID := newAssetTrackID()

	assert.Len(t, trackID, 32)
	for _, ch := range trackID {
		assert.Truef(t, (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f'), "invalid hex character: %q", ch)
	}
}
