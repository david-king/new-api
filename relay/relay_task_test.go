package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				"content": {"video_url": "https://example.com/video.mp4"},
				"usage": {"completion_tokens": 108900, "total_tokens": 108900},
				"created_at": 1743414619,
				"updated_at": 1743414673,
				"seed": 10,
				"resolution": "720p",
				"ratio": "16:9",
				"duration": 5,
				"framespersecond": 24,
				"service_tier": "default",
				"execution_expires_after": 172800,
				"generate_audio": true,
				"draft": false,
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
	assert.Equal(t, 108900, result.Usage.CompletionTokens)
	assert.Equal(t, "720p", result.Resolution)
	assert.Equal(t, "16:9", result.Ratio)
	assert.Equal(t, 5, result.Duration)
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
