package dto

import (
	"encoding/json"
)

type TaskError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Data       any    `json:"data"`
	StatusCode int    `json:"-"`
	LocalError bool   `json:"-"`
	Error      error  `json:"-"`
}

type TaskData interface {
	SunoDataResponse | []SunoDataResponse | string | any
}

const TaskSuccessCode = "success"

type TaskResponse[T TaskData] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func (t *TaskResponse[T]) IsSuccess() bool {
	return t.Code == TaskSuccessCode
}

type TaskDto struct {
	ID         int64           `json:"id"`
	CreatedAt  int64           `json:"created_at"`
	UpdatedAt  int64           `json:"updated_at"`
	TaskID     string          `json:"task_id"`
	Platform   string          `json:"platform"`
	UserId     int             `json:"user_id"`
	Group      string          `json:"group"`
	ChannelId  int             `json:"channel_id"`
	Quota      int             `json:"quota"`
	Action     string          `json:"action"`
	Status     string          `json:"status"`
	FailReason string          `json:"fail_reason"`
	ResultURL  string          `json:"result_url,omitempty"` // 任务结果 URL（视频地址等）
	SubmitTime int64           `json:"submit_time"`
	StartTime  int64           `json:"start_time"`
	FinishTime int64           `json:"finish_time"`
	Progress   string          `json:"progress"`
	Properties any             `json:"properties"`
	Username   string          `json:"username,omitempty"`
	Usage      *Usage          `json:"usage,omitempty"`
	Cost       any             `json:"cost,omitempty"`
	Data       json.RawMessage `json:"data"`
}

type VideoTaskContentDto struct {
	VideoURL string `json:"video_url,omitempty"`
}

type VideoTaskResultDto struct {
	ID                    string               `json:"id"`
	TaskID                string               `json:"task_id,omitempty"`
	UpstreamID            string               `json:"upstream_id,omitempty"`
	Model                 string               `json:"model,omitempty"`
	Status                string               `json:"status"`
	Content               *VideoTaskContentDto `json:"content,omitempty"`
	Usage                 *Usage               `json:"usage,omitempty"`
	CreatedAt             int64                `json:"created_at,omitempty"`
	UpdatedAt             int64                `json:"updated_at,omitempty"`
	Seed                  int                  `json:"seed,omitempty"`
	Resolution            string               `json:"resolution,omitempty"`
	Ratio                 string               `json:"ratio,omitempty"`
	Duration              int                  `json:"duration,omitempty"`
	FramesPerSecond       int                  `json:"framespersecond,omitempty"`
	ServiceTier           string               `json:"service_tier,omitempty"`
	ExecutionExpiresAfter int                  `json:"execution_expires_after,omitempty"`
	GenerateAudio         *bool                `json:"generate_audio,omitempty"`
	Draft                 *bool                `json:"draft,omitempty"`
}

type FetchReq struct {
	IDs []string `json:"ids"`
}
