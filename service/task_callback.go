package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const (
	taskCallbackTimeout     = 5 * time.Second
	taskCallbackMaxAttempts = 3
	taskCallbackRetryDelay  = time.Second
)

func NormalizeTaskCallbackURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u == nil || u.Host == "" {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return raw
	default:
		return ""
	}
}

func VideoTaskResultFromTask(task *model.Task) *dto.VideoTaskResultDto {
	payload := taskVideoPayload(task.Data)
	resolution := taskStringFromMap(payload, "resolution")
	upstreamID := firstNonEmpty(taskStringFromMap(payload, "id"), task.GetUpstreamTaskID())
	result := &dto.VideoTaskResultDto{
		ID:                    task.TaskID,
		TaskID:                task.TaskID,
		UpstreamID:            upstreamID,
		Model:                 firstNonEmpty(taskStringFromMap(payload, "model"), task.Properties.OriginModelName, task.Properties.UpstreamModelName),
		Status:                firstNonEmpty(taskStringFromMap(payload, "status"), mapTaskStatusToVideoResult(task.Status)),
		CreatedAt:             taskInt64FromMap(payload, "created_at"),
		UpdatedAt:             taskInt64FromMap(payload, "updated_at"),
		Seed:                  taskIntFromMap(payload, "seed"),
		Resolution:            resolution,
		Ratio:                 taskStringFromMap(payload, "ratio"),
		Duration:              taskIntFromMap(payload, "duration"),
		FramesPerSecond:       taskIntFromMap(payload, "framespersecond"),
		ServiceTier:           taskStringFromMap(payload, "service_tier"),
		ExecutionExpiresAfter: taskIntFromMap(payload, "execution_expires_after"),
		GenerateAudio:         taskBoolPtrFromMap(payload, "generate_audio"),
		Draft:                 taskBoolPtrFromMap(payload, "draft"),
	}
	if result.CreatedAt == 0 {
		result.CreatedAt = task.CreatedAt
	}
	if result.UpdatedAt == 0 {
		result.UpdatedAt = task.UpdatedAt
	}

	videoURL := taskContentVideoURL(payload)
	if videoURL == "" {
		videoURL = task.GetResultURL()
	}
	if videoURL != "" {
		result.Content = &dto.VideoTaskContentDto{VideoURL: videoURL}
	}

	usageMap, _ := payload["usage"].(map[string]any)
	result.Usage = taskUsageFromMap(usageMap)
	return result
}

func VideoTaskResultResponseBody(task *model.Task) ([]byte, error) {
	return common.Marshal(dto.TaskResponse[any]{
		Code:    "success",
		Message: "",
		Data:    VideoTaskResultFromTask(task),
	})
}

func TaskUsageAndCost(data []byte) (*dto.Usage, any) {
	if len(data) == 0 {
		return nil, nil
	}
	payload := taskVideoPayload(data)
	usageMap, _ := payload["usage"].(map[string]any)
	usage := taskUsageFromMap(usageMap)
	return usage, payload["cost"]
}

func ApplyVideoTaskResult(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, responseBody []byte, taskResult *relaycommon.TaskInfo) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	if taskResult == nil {
		return fmt.Errorf("task result is nil")
	}

	snap := task.Snapshot()
	if len(responseBody) > 0 {
		task.Data = redactVideoResponseBody(responseBody)
	}

	if taskResult.Status == "" {
		errorResult := &dto.GeneralErrorResponse{}
		if err := common.Unmarshal(responseBody, &errorResult); err == nil {
			openaiError := errorResult.TryToOpenAIError()
			if openaiError != nil {
				if openaiError.Code == "429" {
					return nil
				}
				taskResult = relaycommon.FailTaskInfo("upstream returned error")
			} else {
				logger.LogError(ctx, fmt.Sprintf("Task %s returned empty status with unrecognized error format, response: %s", task.TaskID, string(responseBody)))
				taskResult = relaycommon.FailTaskInfo("upstream returned unrecognized message")
			}
		}
	}

	shouldRefund := false
	shouldSettle := false
	quota := task.Quota
	now := time.Now().Unix()

	task.Status = model.TaskStatus(taskResult.Status)
	switch task.Status {
	case model.TaskStatusSubmitted:
		task.Progress = taskcommon.ProgressSubmitted
	case model.TaskStatusQueued:
		task.Progress = taskcommon.ProgressQueued
	case model.TaskStatusInProgress:
		task.Progress = taskcommon.ProgressInProgress
		if task.StartTime == 0 {
			task.StartTime = now
		}
	case model.TaskStatusSuccess:
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		if strings.HasPrefix(taskResult.Url, "data:") {
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		} else if taskResult.Url != "" {
			task.PrivateData.ResultURL = taskResult.Url
		} else {
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		}
		shouldSettle = true
	case model.TaskStatusFailure:
		task.Status = model.TaskStatusFailure
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		task.FailReason = taskResult.Reason
		logger.LogInfo(ctx, fmt.Sprintf("Task %s failed: %s", task.TaskID, task.FailReason))
		taskResult.Progress = taskcommon.ProgressComplete
		if quota != 0 {
			shouldRefund = true
		}
	default:
		return fmt.Errorf("unknown task status %s for task %s", taskResult.Status, task.TaskID)
	}
	if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}

	canNotify := false
	canBill := false
	isDone := task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure
	if isDone && snap.Status != task.Status {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("UpdateWithStatus failed for task %s: %s", task.TaskID, err.Error()))
			shouldRefund = false
			shouldSettle = false
		} else if !won {
			logger.LogWarn(ctx, fmt.Sprintf("Task %s already transitioned by another process, skip billing", task.TaskID))
			shouldRefund = false
			shouldSettle = false
		} else {
			canNotify = true
			canBill = true
		}
	} else if !snap.Equal(task.Snapshot()) {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("Failed to update task %s: %s", task.TaskID, err.Error()))
		} else if won {
			canNotify = true
		}
	} else {
		canNotify = true
		logger.LogDebug(ctx, fmt.Sprintf("No update needed for task %s", task.TaskID))
	}

	if canBill && shouldSettle {
		settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)
	}
	if canBill && shouldRefund {
		RefundTaskQuota(ctx, task, task.FailReason)
	}
	if canNotify {
		NotifyTaskCallbackIfNeeded(ctx, task)
	}
	return nil
}

func HandleVideoTaskCallback(ctx context.Context, platform constant.TaskPlatform, responseBody []byte) (*model.Task, error) {
	upstreamTaskID := ExtractCallbackUpstreamTaskID(responseBody)
	if upstreamTaskID == "" {
		return nil, fmt.Errorf("callback missing upstream task id")
	}

	task, exist, err := model.GetByUpstreamTaskId(upstreamTaskID)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, fmt.Errorf("task not found for upstream task id %s", upstreamTaskID)
	}

	adaptor := GetTaskAdaptorFunc(platform)
	if adaptor == nil {
		return nil, fmt.Errorf("video adaptor not found for platform %s", platform)
	}

	if ch, err := model.CacheGetChannel(task.ChannelId); err == nil {
		adaptor.Init(&relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ApiKey:         ch.Key,
				ChannelType:    ch.Type,
				ChannelBaseUrl: ch.GetBaseURL(),
			},
		})
	} else {
		logger.LogWarn(ctx, fmt.Sprintf("callback task %s failed to load channel %d: %s", task.TaskID, task.ChannelId, err.Error()))
		adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
	}

	taskResult, err := adaptor.ParseTaskResult(responseBody)
	if err != nil {
		return task, err
	}
	if taskResult.TaskID == "" {
		taskResult.TaskID = upstreamTaskID
	}
	return task, ApplyVideoTaskResult(ctx, adaptor, task, responseBody, taskResult)
}

func ExtractCallbackUpstreamTaskID(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	root := map[string]any{}
	if err := common.Unmarshal(body, &root); err != nil {
		return ""
	}
	if data, ok := root["data"].(map[string]any); ok {
		if id := taskStringFromMap(data, "id"); id != "" {
			return id
		}
		if taskID := taskStringFromMap(data, "task_id"); taskID != "" {
			return taskID
		}
	}
	if id := taskStringFromMap(root, "id"); id != "" {
		return id
	}
	return taskStringFromMap(root, "task_id")
}

func NotifyTaskCallbackIfNeeded(ctx context.Context, task *model.Task) {
	if task == nil {
		return
	}
	callbackURL := NormalizeTaskCallbackURL(task.PrivateData.CallbackURL)
	if callbackURL == "" {
		return
	}
	callbackStatus := mapTaskStatusToVideoResult(task.Status)
	if task.PrivateData.CallbackStatus == callbackStatus {
		return
	}
	body, err := VideoTaskResultResponseBody(task)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("build task callback body failed task %s: %s", task.TaskID, err.Error()))
		return
	}

	taskID := task.ID
	publicTaskID := task.TaskID
	status := task.Status
	go postTaskCallback(taskID, publicTaskID, status, callbackStatus, callbackURL, body)
}

func postTaskCallback(taskID int64, publicTaskID string, status model.TaskStatus, callbackStatus string, callbackURL string, body []byte) {
	for attempt := 1; attempt <= taskCallbackMaxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), taskCallbackTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(body))
		if err != nil {
			cancel()
			logger.LogWarn(context.Background(), fmt.Sprintf("build task callback request failed task %s: %s", publicTaskID, err.Error()))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-New-Api-Task-Id", publicTaskID)

		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			_ = resp.Body.Close()
			cancel()
			if err := model.MarkTaskCallbackNotified(taskID, status, callbackStatus, time.Now().Unix()); err != nil {
				logger.LogWarn(context.Background(), fmt.Sprintf("mark task callback notified failed task %s: %s", publicTaskID, err.Error()))
			}
			return
		}
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
			_ = resp.Body.Close()
		}
		cancel()
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("post task callback failed task %s attempt %d/%d: %s", publicTaskID, attempt, taskCallbackMaxAttempts, err.Error()))
		} else {
			logger.LogWarn(context.Background(), fmt.Sprintf("post task callback failed task %s attempt %d/%d: status %d", publicTaskID, attempt, taskCallbackMaxAttempts, statusCode))
		}
		if attempt < taskCallbackMaxAttempts {
			time.Sleep(taskCallbackRetryDelay)
		}
	}
}

func taskVideoPayload(data []byte) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	var root map[string]any
	if err := common.Unmarshal(data, &root); err != nil {
		return map[string]any{}
	}
	if nested, ok := root["data"].(map[string]any); ok {
		return nested
	}
	return root
}

func mapTaskStatusToVideoResult(status model.TaskStatus) string {
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

func taskContentVideoURL(payload map[string]any) string {
	content, _ := payload["content"].(map[string]any)
	return taskStringFromMap(content, "video_url")
}

func taskStringFromMap(m map[string]any, key string) string {
	if len(m) == 0 {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func taskIntFromMap(m map[string]any, key string) int {
	if len(m) == 0 {
		return 0
	}
	return taskIntFromAny(m[key])
}

func taskInt64FromMap(m map[string]any, key string) int64 {
	if len(m) == 0 {
		return 0
	}
	switch n := m[key].(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	default:
		return 0
	}
}

func taskBoolPtrFromMap(m map[string]any, key string) *bool {
	if len(m) == 0 {
		return nil
	}
	if v, ok := m[key].(bool); ok {
		return &v
	}
	return nil
}

func taskUsageFromMap(usageMap map[string]any) *dto.Usage {
	if len(usageMap) == 0 {
		return nil
	}
	completionTokens := taskIntFromAny(usageMap["completion_tokens"])
	totalTokens := taskIntFromAny(usageMap["total_tokens"])
	if totalTokens <= 0 {
		totalTokens = completionTokens
	}
	if completionTokens <= 0 && totalTokens <= 0 {
		return nil
	}
	return &dto.Usage{
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
	}
}

func taskIntFromAny(v any) int {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
