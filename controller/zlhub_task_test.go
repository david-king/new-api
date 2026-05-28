package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestApplyTaskSubmitInitialStatusForZLHub(t *testing.T) {
	task := &model.Task{
		Status:   model.TaskStatusNotStart,
		Progress: "0%",
	}

	applyTaskSubmitInitialStatus(task, constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeZLHub)))

	assert.Equal(t, model.TaskStatus(model.TaskStatusQueued), task.Status)
	assert.Equal(t, "10%", task.Progress)
}

func TestApplyTaskSubmitInitialStatusKeepsOtherPlatforms(t *testing.T) {
	task := &model.Task{
		Status:   model.TaskStatusNotStart,
		Progress: "0%",
	}

	applyTaskSubmitInitialStatus(task, constant.TaskPlatformSuno)

	assert.Equal(t, model.TaskStatus(model.TaskStatusNotStart), task.Status)
	assert.Equal(t, "0%", task.Progress)
}

func setupZLHubControllerTestDB(t *testing.T) *gorm.DB {
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

func TestCancelZLHubTaskUsesLocalTaskAndUpstreamID(t *testing.T) {
	db := setupZLHubControllerTestDB(t)

	var upstreamCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/task/cancel/cgt-cancel", r.URL.Path)
		assert.Equal(t, "Bearer video-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"success","data":{"id":"cgt-cancel","status":"cancelled"}}`))
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
		TaskID:    "task_cancel",
		UserId:    7,
		ChannelId: 1,
		Status:    model.TaskStatusQueued,
		Progress:  "10%",
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "cgt-cancel",
		},
	}).Error)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 7)
	c.Params = gin.Params{{Key: "task_id", Value: "task_cancel"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/task/cancel/task_cancel", nil)

	CancelZLHubTask(c)

	require.True(t, upstreamCalled)
	assert.Equal(t, http.StatusOK, w.Code)
	var reloaded model.Task
	require.NoError(t, db.Where("task_id = ?", "task_cancel").First(&reloaded).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), reloaded.Status)
	assert.Equal(t, "100%", reloaded.Progress)
	assert.Equal(t, "task cancelled", reloaded.FailReason)
}

func TestZLHubProxyVideoGetUsesVideoKeyAndChannelBaseURL(t *testing.T) {
	db := setupZLHubControllerTestDB(t)

	var upstreamCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/task/get/cgt-proxy", r.URL.Path)
		assert.Equal(t, "Bearer video-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"success","data":{"id":"cgt-proxy","status":"queued"}}`))
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

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "task_id", Value: "cgt-proxy"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/zlhub/v1/task/get/cgt-proxy?channel_id=1", nil)

	ZLHubProxyVideoGet(c)

	require.True(t, upstreamCalled)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"code":"success","data":{"id":"cgt-proxy","status":"queued"}}`, w.Body.String())
}
