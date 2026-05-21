package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractCallbackUpstreamTaskID(t *testing.T) {
	assert.Equal(t, "cgt-2025-test", ExtractCallbackUpstreamTaskID([]byte(`{
		"code": "success",
		"data": {
			"id": "cgt-2025-test",
			"status": "succeeded"
		}
	}`)))
	assert.Equal(t, "bare-task", ExtractCallbackUpstreamTaskID([]byte(`{"id":"bare-task"}`)))
	assert.Equal(t, "", ExtractCallbackUpstreamTaskID([]byte(`{"code":"success"}`)))
}

func TestVideoTaskResultResponseBodyIsSanitized(t *testing.T) {
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
					"tool_usage": {"web_search": 1}
				},
				"frames": 120,
				"tools": [{"type": "web_search"}],
				"safety_identifier": "user-hash",
				"draft_task_id": "cgt-draft",
				"cost": {"total_cost": "1.23"},
				"extra": "not exposed"
			}
		}`),
	}

	body, err := VideoTaskResultResponseBody(task)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, common.Unmarshal(body, &out))
	data, ok := out["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "task_public", data["id"])
	assert.Equal(t, "task_public", data["task_id"])
	assert.Equal(t, "cgt-2025-test", data["upstream_id"])
	assert.Equal(t, "succeeded", data["status"])
	assert.Contains(t, data, "error")
	assert.Equal(t, float64(120), data["frames"])
	assert.Equal(t, "user-hash", data["safety_identifier"])
	assert.Equal(t, "cgt-draft", data["draft_task_id"])
	usage, ok := data["usage"].(map[string]any)
	require.True(t, ok)
	toolUsage, ok := usage["tool_usage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), toolUsage["web_search"])
	assert.NotContains(t, data, "data")
	assert.NotContains(t, data, "cost")
	assert.NotContains(t, data, "extra")
}

func TestNormalizeTaskCallbackURL(t *testing.T) {
	assert.Equal(t, "https://example.com/callback", NormalizeTaskCallbackURL(" https://example.com/callback "))
	assert.Equal(t, "http://example.com/callback", NormalizeTaskCallbackURL("http://example.com/callback"))
	assert.Equal(t, "", NormalizeTaskCallbackURL("ftp://example.com/callback"))
	assert.Equal(t, "", NormalizeTaskCallbackURL("not-a-url"))
}
