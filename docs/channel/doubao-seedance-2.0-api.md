# 视频生成与素材审核对外接入文档

本文档面向接入方，说明如何通过本平台调用 ZLHub 视频生成与素材审核能力。接入方只需要关注本平台暴露的接口、任务 ID、查询和回调格式，不需要直接对接上游服务。

## 请求和返回-官方详细接口文档
### 请详细参考下面的火山官方接口。

- 创建视频生成任务 `API` 接口文档
```text
https://www.volcengine.com/docs/82379/1520757?lang=zh
```

- 查询视频生成任务 `API` 接口文档
```text
https://www.volcengine.com/docs/82379/15213s09?lang=zh
```

- 取消或删除视频生成任务接口文档
```text
https://www.volcengine.com/docs/82379/1521720?lang=zh
```

## 1. 接入信息

### 1.1 Base URL

```text
https://your-domain.com
```

请将示例中的 `https://your-domain.com` 替换为实际服务地址。

### 1.2 鉴权方式

所有对外业务接口均使用 Bearer Token 鉴权：

```http
Authorization: Bearer <API_KEY>
Content-Type: application/json
X-Trace-ID: <TRACE_ID>
```

`X-Trace-ID` 请求跟踪ID，建议填写，用于出现问题时，反馈 `TraceID` 快速排查定位。每次请求必须不同随机32 位字符串，如  `f3ab5c98d7e6f1a2b3c4d5e6f7a8b9c0`。

### 1.3 接口总览

| 场景 | 方法 | 路径 | 说明 |
|---|---|---|---|
| 创建视频任务 | POST | `/v1/task/create` | 创建异步视频生成任务 |
| 查询视频任务 | GET | `/v1/task/get/{task_id}` | 查询任务状态和结果 |
| 取消视频任务 | POST | `/v1/task/cancel/{task_id}` | 取消未完成任务 |
| 同步素材审核 | POST | `/v1/asset/upload/sync` | 提交素材并同步等待结果 |
| 异步素材审核 | POST | `/v1/asset/upload/async` | 提交素材审核任务 |
| 查询素材审核 | GET | `/v1/asset/task/{task_id}` | 查询异步素材审核结果 |

## 2. 视频生成流程

说明：

- 创建接口返回的是本平台本地任务 ID，格式一般为 `task_xxx`。
- 查询、取消、回调中都应使用或保存本地 `task_id`。
- 即使配置了 `callback_url`，仍建议保留查询逻辑作为兜底。(可选，作为辅助，建议任务创建10分钟后还未收到状态回调可以主动查询，每个任务每分钟查询一次，严禁频繁查询)

## 3. 创建视频任务

```http
POST /v1/task/create
```

### 3.1 文生视频示例

```bash
curl -X POST "https://your-domain.com/v1/task/create" \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2.0",
    "content": [
      {"type": "text", "text": "一个女孩在海边奔跑，电影感镜头，自然光"}
    ],
    "duration": 6,
    "ratio": "16:9",
    "resolution": "720p",
    "generate_audio": true,
    "watermark": false,
    "callback_url": "https://client.example.com/callback/video"
  }'
```

### 3.2 图生视频首帧示例

```bash
curl -X POST "https://your-domain.com/v1/task/create" \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2.0",
    "content": [
      {"type": "text", "text": "让图片中的人物自然转身并微笑"},
      {
        "type": "image_url",
        "image_url": {"url": "https://client.example.com/assets/person.jpg"},
        "role": "first_frame"
      }
    ],
    "duration": 5,
    "ratio": "9:16",
    "resolution": "720p"
  }'
```

### 3.3 图生视频首尾帧示例

```bash
curl -X POST "https://your-domain.com/v1/task/create" \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2.0",
    "content": [
      {"type": "text", "text": "根据首帧和尾帧生成流畅过渡的视频"},
      {
        "type": "image_url",
        "image_url": {"url": "https://client.example.com/assets/first.jpg"},
        "role": "first_frame"
      },
      {
        "type": "image_url",
        "image_url": {"url": "https://client.example.com/assets/last.jpg"},
        "role": "last_frame"
      }
    ],
    "duration": 8,
    "ratio": "16:9",
    "resolution": "720p"
  }'
```

### 3.4 多模态参考示例

```bash
curl -X POST "https://your-domain.com/v1/task/create" \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2.0",
    "content": [
      {"type": "text", "text": "参考图片中的人物、视频动作和音频节奏生成一段自拍视频"},
      {
        "type": "image_url",
        "image_url": {"url": "https://client.example.com/assets/ref.jpg"},
        "role": "reference_image"
      },
      {
        "type": "video_url",
        "video_url": {"url": "https://client.example.com/assets/motion.mp4"},
        "role": "reference_video"
      },
      {
        "type": "audio_url",
        "audio_url": {"url": "https://client.example.com/assets/voice.mp3"},
        "role": "reference_audio"
      }
    ],
    "duration": 6,
    "ratio": "16:9",
    "resolution": "720p",
    "generate_audio": true,
    "callback_url": "https://client.example.com/callback/video"
  }'
```

### 3.5 使用审核素材示例

```bash
curl -X POST "https://your-domain.com/v1/task/create" \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2.0",
    "content": [
      {"type": "text", "text": "参考审核通过的图片生成视频"},
      {
        "type": "image_url",
        "image_url": {"url": "Asset://Asset-20260520120000-xxxxx"},
        "role": "reference_image"
      }
    ],
    "duration": 6,
    "ratio": "16:9",
    "resolution": "720p"
  }'
```

### 3.6 创建响应

创建成功后返回本地任务 ID：

```json
{
  "code": "success",
  "message": "",
  "data": {
    "id": "task_fywJTIsAR1DYIyEVRWkG35n3Xesyx1RE",
    "task_id": "task_fywJTIsAR1DYIyEVRWkG35n3Xesyx1RE",
    "model": "doubao-seedance-2.0",
    "status": "queued",
    "created_at": 1779273982
  }
}
```

字段说明：

| 字段 | 说明 |
|---|---|
| `data.id` | 本平台任务 ID |
| `data.task_id` | 同 `data.id`，查询、取消和回调幂等使用该字段 |
| `data.status` | 初始状态，一般为 `queued` |
| `data.created_at` | 创建时间，Unix 秒时间戳 |

## 4. 查询视频任务

```http
GET /v1/task/get/{task_id}
```

### 4.1 请求示例

```bash
curl -X GET "https://your-domain.com/v1/task/get/task_fywJTIsAR1DYIyEVRWkG35n3Xesyx1RE" \
  -H "Authorization: Bearer <API_KEY>"
```

### 4.2 查询响应

```json
{
    "code": "success",
    "message": "",
    "data": {
        "id": "task_IKHoit5LEAd0j3uOvUMZ5aQMUlv4m9cr",
        "task_id": "task_IKHoit5LEAd0j3uOvUMZ5aQMUlv4m9cr",
        "upstream_id": "cgt-20260521185344-tbm29",
        "model": "doubao-seedance-2.0",
        "status": "succeeded",
        "error": null,
        "content": {
            "video_url": ""
        },
        "usage": {
            "completion_tokens": 87300,
            "total_tokens": 87300
        },
        "created_at": 1779360825,
        "updated_at": 1779360941,
        "seed": 47108,
        "resolution": "720p",
        "ratio": "16:9",
        "duration": 4,
        "framespersecond": 24,
        "service_tier": "default",
        "execution_expires_after": 172800,
        "generate_audio": true,
        "draft": false
    }
}
```

### 4.3 任务状态

| 状态 | 说明 |
|---|---|
| `queued` | 排队中 |
| `running` | 生成中 |
| `succeeded` | 生成成功 |
| `failed` | 生成失败 |
| `cancelled` | 任务已取消 |
| `expired` | 任务超时 |

说明：

- 成功后从 `data.content.video_url` 获取视频地址。
- 如果创建任务时设置 `return_last_frame: true`，且上游返回尾帧，则可从 `data.content.last_frame_url` 获取尾帧图像。
- `data.duration` 和 `data.frames` 只会返回一个；创建时指定 `frames` 时优先返回 `frames`。
- `data.usage` 为本次任务实际 token 用量，最终计费以平台计费配置和实际用量为准。
- `data.error`、`data.tools`、`data.safety_identifier`、`data.draft_task_id` 等字段按上游查询结果保留。

## 5. 取消视频任务

```http
POST /v1/task/cancel/{task_id}
```

仅支持取消未完成的视频任务。

### 5.1 请求示例

```bash
curl -X POST "https://your-domain.com/v1/task/cancel/task_fywJTIsAR1DYIyEVRWkG35n3Xesyx1RE" \
  -H "Authorization: Bearer <API_KEY>"
```

### 5.2 响应示例

```json
{
  "code": "success",
  "message": "",
  "data": {
    "id": "task_fywJTIsAR1DYIyEVRWkG35n3Xesyx1RE",
    "status": "cancelled"
  }
}
```

## 6. 视频任务回调

创建任务时传入 `callback_url` 后，任务状态变化时平台会向该地址发送 POST 请求。

### 6.1 回调要求

| 项目 | 说明 |
|---|---|
| 请求方法 | POST |
| Content-Type | `application/json` |
| 超时时间 | 5 秒 |
| 重试次数 | 最多 3 次 |
| 成功条件 | 接入方返回 HTTP 2xx |

### 6.2 回调请求体

回调请求体与 `GET /v1/task/get/{task_id}` 的响应结构一致：

```json
{
  "code": "success",
  "message": "",
  "data": {
    "id": "task_fywJTIsAR1DYIyEVRWkG35n3Xesyx1RE",
    "task_id": "task_fywJTIsAR1DYIyEVRWkG35n3Xesyx1RE",
    "model": "doubao-seedance-2.0",
    "status": "succeeded",
    "error": null,
    "content": {
      "video_url": "https://example.com/video.mp4"
    },
    "usage": {
      "completion_tokens": 131137,
      "total_tokens": 131137,
      "tool_usage": {
        "web_search": 1
      }
    },
    "duration": 6,
    "resolution": "720p",
    "ratio": "21:9",
    "framespersecond": 24,
    "tools": [
      {"type": "web_search"}
    ],
    "safety_identifier": "user-hash",
    "generate_audio": true,
    "draft": false,
    "draft_task_id": "cgt-draft"
  }
}
```

### 6.3 回调处理建议

- 使用 `data.task_id` 做幂等键。
- 同一个任务可能收到多个状态回调，业务侧应允许重复通知。
- 最终状态为 `succeeded` 或 `failed`。
- 如果未收到回调，应使用查询接口兜底。

## 7. 素材审核

素材审核用于提前审核图片、视频或音频素材。审核通过后返回的 `Asset://` 地址可以用于视频生成请求中的素材 URL。

### 7.1 同步提交审核

```http
POST /v1/asset/upload/sync
```

同步接口会等待审核结果返回，适合少量素材。

```bash
curl -X POST "https://your-domain.com/v1/asset/upload/sync" \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "images": [
      "https://client.example.com/assets/photo1.jpg"
    ],
    "asset_type": "Image"
  }'
```

### 7.2 异步提交审核

```http
POST /v1/asset/upload/async
```

异步接口会立即返回素材审核任务 ID，后续通过查询接口获取结果。

```bash
curl -X POST "https://your-domain.com/v1/asset/upload/async" \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "images": [
      "https://client.example.com/assets/photo1.jpg",
      "https://client.example.com/assets/photo2.jpg"
    ],
    "asset_type": "Image"
  }'
```

### 7.3 查询素材审核

```http
GET /v1/asset/task/{task_id}
```

```bash
curl -X GET "https://your-domain.com/v1/asset/task/asset-task-id" \
  -H "Authorization: Bearer <API_KEY>"
```

### 7.4 素材审核请求字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `images` | string[] | 是 | 素材 URL 列表，最多 50 条 |
| `asset_type` | string | 否 | `Image`、`Video`、`Audio`，默认 `Image` |

注意：

- 素材 URL 必须是公网可访问的 HTTP(S) 地址。
- 不支持 base64。
- 同一批次素材类型必须一致。
- 当前素材审核接口不支持接入方自定义 `callback_url`，异步结果请通过查询接口获取。

### 7.5 审核结果关键字段

| 字段 | 说明 |
|---|---|
| `submit_review_status` | `1` 表示通过，`0` 表示失败 |
| `asset_url` | `Asset://` 地址，可用于视频生成 |
| `downstream_final_url` | 临时访问地址 |
| `error_code` / `error_message` | 审核失败原因 |
