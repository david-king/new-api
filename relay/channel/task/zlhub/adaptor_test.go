package zlhub

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

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
		Size:   "16:9",
		Metadata: map[string]interface{}{
			"ratio": "9:16",
		},
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
