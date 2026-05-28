package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestVideoTaskResultFromTaskSanitizesStoredData(t *testing.T) {
	task := &model.Task{
		TaskID:    "task_public",
		Platform:  constant.TaskPlatform("58"),
		Status:    model.TaskStatusSuccess,
		CreatedAt: 1743414619,
		UpdatedAt: 1743414673,
		Properties: model.Properties{
			OriginModelName: "doubao-seedance-2.0",
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "cgt-upstream",
		},
		Data: []byte(`{
			"code": "success",
			"data": {
				"id": "cgt-2025-test",
				"model": "doubao-seedance-2.0",
				"status": "succeeded",
				"error": null,
				"content": {
					"video_url": "https://example.com/video.mp4",
					"last_frame_url": "https://example.com/last-frame.png"
				},
				"usage": {
					"completion_tokens": 108900,
					"total_tokens": 108900,
					"tool_usage": {"web_search": 2}
				},
				"created_at": 1743414619,
				"updated_at": 1743414673,
				"seed": 10,
				"resolution": "720p",
				"ratio": "16:9",
				"duration": 5,
				"framespersecond": 24,
				"tools": [{"type": "web_search"}],
				"safety_identifier": "user-hash",
				"service_tier": "default",
				"execution_expires_after": 172800,
				"generate_audio": true,
				"draft": false,
				"draft_task_id": "cgt-draft",
				"cost": {"total_cost": "1.23"},
				"extra": "not exposed"
			}
		}`),
	}

	result := service.VideoTaskResultFromTask(task)

	require.NotNil(t, result.Content)
	require.NotNil(t, result.Usage)
	require.NotNil(t, result.GenerateAudio)
	require.NotNil(t, result.Draft)
	assert.Equal(t, "task_public", result.ID)
	assert.Equal(t, "task_public", result.TaskID)
	assert.Equal(t, "cgt-2025-test", result.UpstreamID)
	assert.Equal(t, "succeeded", result.Status)
	assert.Equal(t, "https://example.com/video.mp4", result.Content.VideoURL)
	assert.Equal(t, "https://example.com/last-frame.png", result.Content.LastFrameURL)
	assert.Equal(t, 108900, result.Usage.CompletionTokens)
	assert.Equal(t, 108900, result.Usage.TotalTokens)
	assert.Equal(t, 2, result.Usage.ToolUsage["web_search"])
	assert.Equal(t, "720p", result.Resolution)
	assert.Equal(t, "16:9", result.Ratio)
	assert.Equal(t, 5, result.Duration)
	assert.Equal(t, 24, result.FramesPerSecond)
	assert.Equal(t, "user-hash", result.SafetyIdentifier)
	assert.Equal(t, "default", result.ServiceTier)
	assert.Equal(t, 172800, result.ExecutionExpiresAfter)
	assert.Equal(t, "cgt-draft", result.DraftTaskID)
	assert.True(t, *result.GenerateAudio)
	assert.False(t, *result.Draft)

	body, err := common.Marshal(result)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, common.Unmarshal(body, &out))
	assert.NotContains(t, out, "data")
	assert.NotContains(t, out, "quota")
	assert.NotContains(t, out, "cost")
	assert.NotContains(t, out, "extra")

	wrapped, err := common.Marshal(dto.TaskResponse[any]{
		Code:    "success",
		Message: "",
		Data:    result,
	})
	require.NoError(t, err)
	var wrappedOut map[string]any
	require.NoError(t, common.Unmarshal(wrapped, &wrappedOut))
	data, ok := wrappedOut["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "task_public", data["id"])
	assert.Equal(t, "task_public", data["task_id"])
	assert.Equal(t, "cgt-2025-test", data["upstream_id"])
	assert.NotContains(t, data, "data")
	assert.NotContains(t, data, "cost")
}

func setupRelayTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.RedisEnabled = oldRedisEnabled
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.Channel{}))
	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	service.InitHttpClient()
	return db
}

func TestTryRealtimeFetchZLHubTaskAPIRefreshesFromUpstream(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	var upstreamCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		assert.Equal(t, "/v1/task/get/cgt-test", r.URL.Path)
		assert.Equal(t, "Bearer video-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": "success",
			"message": "",
			"data": {
				"id": "cgt-test",
				"model": "doubao-seedance-2.0",
				"status": "running",
				"content": {},
				"created_at": 1779358566,
				"updated_at": 1779359051
			}
		}`))
	}))
	defer server.Close()

	baseURL := server.URL
	channel := &model.Channel{
		Id:      1,
		Type:    constant.ChannelTypeZLHub,
		Key:     "video-key|asset-token",
		BaseURL: &baseURL,
		Status:  common.ChannelStatusEnabled,
	}
	require.NoError(t, db.Create(channel).Error)

	task := &model.Task{
		TaskID:    "task_public",
		Platform:  constant.TaskPlatform("58"),
		UserId:    1,
		ChannelId: 1,
		Status:    model.TaskStatusNotStart,
		Progress:  "0%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "cgt-test",
		},
		Properties: model.Properties{
			OriginModelName: "doubao-seedance-2.0",
		},
	}
	require.NoError(t, db.Create(task).Error)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/task/get/task_public", nil)

	respBody := tryRealtimeFetch(c, task, false, true)
	require.NotEmpty(t, respBody)
	assert.True(t, upstreamCalled)

	var resp dto.TaskResponse[map[string]any]
	require.NoError(t, common.Unmarshal(respBody, &resp))
	require.True(t, resp.IsSuccess())
	assert.Equal(t, "task_public", resp.Data["task_id"])
	assert.Equal(t, "cgt-test", resp.Data["upstream_id"])
	assert.Equal(t, "running", resp.Data["status"])

	var reloaded model.Task
	require.NoError(t, db.First(&reloaded, task.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusInProgress), reloaded.Status)
	assert.Equal(t, "50%", reloaded.Progress)
}

func TestRelayTaskFetchZLHubTaskAPIRefreshesFromUpstream(t *testing.T) {
	db := setupRelayTaskTestDB(t)
	var upstreamCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		assert.Equal(t, "/v1/task/get/cgt-entry", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": "success",
			"message": "",
			"data": {
				"id": "cgt-entry",
				"model": "doubao-seedance-2.0",
				"status": "succeeded",
				"content": {
					"video_url": "https://example.com/video.mp4"
				},
				"usage": {
					"completion_tokens": 108900,
					"total_tokens": 108900
				},
				"created_at": 1779358566,
				"updated_at": 1779359051
			}
		}`))
	}))
	defer server.Close()

	baseURL := server.URL
	require.NoError(t, db.Create(&model.Channel{
		Id:      1,
		Type:    constant.ChannelTypeZLHub,
		Key:     "video-key|asset-token",
		BaseURL: &baseURL,
		Status:  common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_entry",
		Platform:  constant.TaskPlatform("58"),
		UserId:    7,
		ChannelId: 1,
		Status:    model.TaskStatusQueued,
		Progress:  "10%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "cgt-entry",
		},
		Properties: model.Properties{
			OriginModelName: "doubao-seedance-2.0",
		},
	}).Error)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 7)
	c.Params = gin.Params{{Key: "task_id", Value: "task_entry"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/task/get/task_entry", nil)

	taskErr := RelayTaskFetch(c, relayconstant.RelayModeVideoFetchByID)
	require.Nil(t, taskErr)
	assert.True(t, upstreamCalled)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp dto.TaskResponse[map[string]any]
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.IsSuccess())
	assert.Equal(t, "task_entry", resp.Data["task_id"])
	assert.Equal(t, "cgt-entry", resp.Data["upstream_id"])
	assert.Equal(t, "succeeded", resp.Data["status"])
	content, ok := resp.Data["content"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://example.com/video.mp4", content["video_url"])

	var reloaded model.Task
	require.NoError(t, db.Where("task_id = ?", "task_entry").First(&reloaded).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), reloaded.Status)
	assert.Equal(t, "100%", reloaded.Progress)
}
