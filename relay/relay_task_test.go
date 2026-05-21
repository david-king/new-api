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
