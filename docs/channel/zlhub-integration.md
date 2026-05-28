# ZLHub 渠道内部集成指南

本文档面向维护者，说明 ZLHub 渠道在 new-api 内部的路由、请求转换、任务生命周期、回调、轮询、计费、素材审核和排查方式。对外接入文档见 [zlhub-external-api.md](./zlhub-external-api.md)，上游原始字段说明见 [zlhub-api-docs.md](../../zlhub-api-docs.md)。

## 1. 渠道定位

| 项目 | 值 |
|------|------|
| 渠道类型 | ZLHub |
| 渠道编号 | `58` |
| 主要能力 | 视频生成、视频任务查询、视频任务取消、素材审核 |
| 默认视频 Base URL | `https://api.zlhub.cn` |
| 固定素材 Base URL | `https://asset.zlhub.cn` |
| 当前重点模型 | `doubao-seedance-2.0`、`doubao-seedance-2.0-fast` |

ZLHub 视频生成走标准 relay 任务链路，会创建本地 `Task`，参与预扣、完成结算、失败退款、消费日志和后台轮询。素材审核是独立代理接口，不创建 `Task`，不进入视频任务计费链路。

## 2. 关键文件

| 模块 | 文件 | 职责 |
|------|------|------|
| 路由 | `router/video-router.go` | `/v1/task/*` 和 `/v1/asset/*` 对外路由 |
| ZLHub 路由 | `router/zlhub_router.go` | `/api/zlhub/*` 透传和回调路由 |
| 视频 adaptor | `relay/channel/task/zlhub/adaptor.go` | 请求校验、请求体转换、上游调用、任务结果解析、取消任务、素材审核客户端 |
| 任务 DTO | `dto/task.go` | 统一视频任务查询/回调响应 DTO |
| 任务查询 | `relay/relay_task.go` | `/v1/task/get/{task_id}` 使用本地任务定位上游 ID，实时刷新后返回统一结果 |
| 视频回调 | `service/task_callback.go` | 处理上游回调、生成下游查询/回调响应、推送用户 callback_url |
| 任务轮询 | `service/task_polling.go` | 后台轮询、ZLHub 60 秒节流、完成结算入口 |
| 任务计费 | `service/task_billing.go` | token 重算、差额结算、失败退款 |
| 素材接口 | `controller/zlhub_asset.go` | 素材审核、ZLHub 透传、取消本地任务、视频回调入口 |
| Seedance 定价 | `relay/channel/task/taskcommon/helpers.go` | Seedance 2.0 分辨率/输入视频价格修正 |

## 3. 凭证配置

ZLHub 使用两套凭证，渠道 Key 支持用 `|` 分隔：

| 场景 | Key 格式 | 说明 |
|------|----------|------|
| 仅视频生成 | `video_api_key` | 视频接口使用该 key |
| 视频 + 素材审核 | `video_api_key\|asset_access_token` | 前半段给视频，后半段给素材审核 |
| 两者相同 | `key` | `ParseAssetCredentials` 自动复用 |

视频生成请求头：

```http
Authorization: Bearer <video_api_key>
Content-Type: application/json
Accept: application/json
X-Trace-ID: <client trace or generated trace>
```

素材审核请求头：

```http
X-Access-Token: <asset_access_token>
X-Track-Id: <generated trace>
Content-Type: application/json
```

## 4. 路由矩阵

### 4.1 对外契约

| 场景 | 方法 | 路径 | Controller | 说明 |
|------|------|------|------------|------|
| 创建视频任务 | POST | `/v1/task/create` | `controller.RelayTask` | 对外只提供 `content` 数组请求体 |
| 查询视频任务 | GET | `/v1/task/get/{task_id}` | `controller.RelayTaskFetch` | `task_id` 是本地 `task_xxx` |
| 取消视频任务 | POST | `/v1/task/cancel/{task_id}` | `controller.CancelZLHubTask` | 只支持本地 ZLHub 任务 |
| 同步素材审核 | POST | `/v1/asset/upload/sync` | `controller.SubmitAssetReviewSync` | 独立代理，不进 Task 表 |
| 异步素材审核 | POST | `/v1/asset/upload/async` | `controller.SubmitAssetReviewAsync` | 返回素材审核任务 ID |
| 查询素材审核 | GET | `/v1/asset/task/{task_id}` | `controller.QueryAssetTask` | 查询素材审核结果 |

对外视频创建只写一种格式：`model + content[] + 顶层可选参数`。不要再在对外文档中暴露 `prompt/images/size/metadata` 或 OpenAI Video 请求格式。

### 4.2 内部/兼容路由

| 场景 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 视频查询透传 | GET | `/api/zlhub/v1/task/get/{upstream_id}` | 直接请求 ZLHub，上游 ID，不经本地状态和计费 |
| 视频取消透传 | POST | `/api/zlhub/v1/task/cancel/{upstream_id}` | 直接请求 ZLHub，上游 ID，不做本地退款 |
| 视频回调主入口 | POST | `/api/task/callback/zlhub/video` | `BuildRequestBody` 注入给上游 |
| 视频回调旧入口 | POST | `/api/zlhub/callback/video` | 保留历史任务回调兼容 |
| 素材审核兼容提交 | POST | `/api/zlhub/asset/upload` | body 里 `async=true` 走异步，否则同步 |
| 素材审核兼容查询 | GET | `/api/zlhub/asset/task/{task_id}` | 与 `/v1/asset/task/{task_id}` 等价 |
| 素材审核回调 | POST | `/api/zlhub/asset/callback` | 目前只记录日志 |

通用视频兼容路由（如 `/v1/videos`、`/v1/video/generations`）仍由项目通用 router 存在，ZLHub adaptor 也保留 `ConvertToOpenAIVideo`，但这些不是 ZLHub 对外接入契约。

## 5. 视频请求契约

### 5.1 对外请求体

```json
{
  "model": "doubao-seedance-2.0",
  "content": [
    {"type": "text", "text": "一个女孩在海边奔跑"},
    {
      "type": "image_url",
      "image_url": {"url": "https://example.com/ref.jpg"},
      "role": "reference_image"
    }
  ],
  "duration": 6,
  "ratio": "16:9",
  "resolution": "720p",
  "generate_audio": true,
  "callback_url": "https://client.example.com/callback/video"
}
```

| 字段 | 类型 | 必填 | 处理方式 |
|------|------|------|----------|
| `model` | string | 是 | 写入 `TaskSubmitReq.Model`，参与模型映射和计费 |
| `content` | object[] | 是 | 原样转发到上游；同时提取文本、图片、视频用于追踪和计费判断 |
| `callback_url` | string | 否 | 不转发给上游；保存到本地 `Task.PrivateData.CallbackURL` |
| `return_last_frame` | bool | 否 | 原样转发 |
| `service_tier` | string | 否 | 原样转发 |
| `execution_expires_after` | int | 否 | 原样转发 |
| `generate_audio` | bool | 否 | 原样转发，显式 `false` 必须保留 |
| `draft` | bool | 否 | 原样转发，显式 `false` 必须保留 |
| `tools` | object[] | 否 | 原样转发 |
| `safety_identifier` | string | 否 | 原样转发 |
| `ratio` | string | 否 | 原样转发 |
| `resolution` | string | 否 | 原样转发，并用于 Seedance 2.0 价格修正 |
| `duration` | int | 否 | 原样转发，并用于按官方公式估算预扣 token |
| `frames` | int | 否 | 原样转发；存在时上游优先使用 `frames` |
| `watermark` | bool | 否 | 原样转发 |
| `seed` | int | 否 | 原样转发 |
| `camera_fixed` | bool | 否 | 原样转发 |

`content` 支持：

| 类型 | 结构 | 说明 |
|------|------|------|
| 文本 | `{ "type": "text", "text": "..." }` | 可选；纯媒体任务允许无文本 |
| 图片 | `{ "type": "image_url", "image_url": {"url": "..."}, "role": "first_frame" }` | `role` 可为 `first_frame`、`last_frame`、`reference_image` |
| 视频 | `{ "type": "video_url", "video_url": {"url": "..."}, "role": "reference_video" }` | 用于判断输入含视频并修正价格 |
| 音频 | `{ "type": "audio_url", "audio_url": {"url": "..."}, "role": "reference_audio" }` | 不能单独输入音频，上游校验 |
| 样片任务 | `{ "type": "draft_task", "draft_task": {"id": "cgt-xxx"} }` | 由上游决定模型支持 |

### 5.2 内部兼容请求体

`ValidateRequestAndSetAction` 仍兼容历史 `TaskSubmitReq` 分支：

```json
{
  "model": "doubao-seedance-2.0",
  "prompt": "legacy prompt",
  "images": ["https://example.com/a.jpg"],
  "duration": 5,
  "ratio": "16:9"
}
```

该分支通过 `convertToRequestPayload` 转成 ZLHub `content` 请求。保留原因是兼容旧调用和通用 relay，不代表对外契约。新增对外文档或客户对接时不要使用该格式。

## 6. 请求处理流程

### 6.1 校验和存储请求

`relay/channel/task/zlhub/adaptor.go` 的 `ValidateRequestAndSetAction`：

1. 从 `common.GetBodyStorage` 读取原始 body。
2. 如果 JSON 中存在 `content`：
   - 进入 `parseNativeRequest`。
   - 提取 `model`、`duration`、`callback_url`。
   - 从 `content` 中提取首个 `type=text` 文本到 `TaskSubmitReq.Prompt`。
   - 从 `content` 中提取 `image_url.url` 到 `TaskSubmitReq.Images`。
   - 允许纯媒体内容无文本。
   - 将除 `model/content/duration/callback_url` 外的字段放入 `Metadata`，便于计费辅助函数读取。
   - 将原始请求体保存到 gin context：`zlhub_native_request_body`。
3. 如果没有 `content`，回退 `ValidateBasicTaskRequest`，兼容历史 `TaskSubmitReq`。

### 6.2 构造上游请求

`BuildRequestBody` 的规则：

| 输入分支 | 行为 |
|----------|------|
| 原生 `content` 分支 | 解析原始 body，处理模型映射，删除用户 `callback_url`，注入平台内部 `callback_url` |
| 历史兼容分支 | 从 `TaskSubmitReq` 调 `convertToRequestPayload` 转成 ZLHub 请求体，再注入内部 `callback_url` |

上游只接收平台内部回调地址：

```text
{ServerAddress}/api/task/callback/zlhub/video
```

用户提交的 `callback_url` 只保存到本地任务私有数据，任务状态更新完成后由 new-api 主动回调用户。

### 6.3 上游 URL

| 操作 | URL |
|------|-----|
| 创建 | `{ChannelBaseUrl}/v1/task/create` |
| 查询 | `{ChannelBaseUrl}/v1/task/get/{upstream_id}` |
| 取消 | `{ChannelBaseUrl}/v1/task/cancel/{upstream_id}` |

### 6.4 创建响应

ZLHub 上游创建响应只要求能解析到 `id`。new-api 返回给 `/v1/task/create` 调用方的是本地任务响应包：

```json
{
  "code": "success",
  "message": "",
  "data": {
    "id": "task_public",
    "task_id": "task_public",
    "model": "doubao-seedance-2.0",
    "status": "queued",
    "created_at": 1747660800
  }
}
```

内部保存：

| 字段 | 来源 |
|------|------|
| `Task.TaskID` | `RelayInfo.PublicTaskID`，本地公开 ID |
| `Task.PrivateData.UpstreamTaskID` | 上游创建响应 `id` |
| `Task.PrivateData.CallbackURL` | 用户请求里的 `callback_url` |
| `Task.PrivateData.BillingContext` | 预扣时冻结的计费上下文 |
| `Task.Data` | 上游创建响应原文 |

## 7. 查询和返回字段

`GET /v1/task/get/{task_id}` 使用本地任务 ID 查询。平台先用本地任务定位 `Task.PrivateData.UpstreamTaskID`，实时请求 ZLHub `/v1/task/get/{upstream_id}` 并通过 `service.ApplyVideoTaskResult` 写回任务状态、结果和计费终态；如果上游查询失败，则回退返回本地缓存。非 OpenAI Video 路由走 `service.VideoTaskResultFromTask`，返回统一任务响应包：

```json
{
  "code": "success",
  "message": "",
  "data": {
    "id": "task_public",
    "task_id": "task_public",
    "upstream_id": "cgt-20260520184622-8kk4d",
    "model": "doubao-seedance-2.0",
    "status": "succeeded",
    "error": null,
    "content": {
      "video_url": "https://example.com/video.mp4",
      "last_frame_url": "https://example.com/last-frame.png"
    },
    "usage": {
      "completion_tokens": 131137,
      "total_tokens": 131137,
      "tool_usage": {
        "web_search": 1
      }
    },
    "created_at": 1747660800,
    "updated_at": 1747660860,
    "seed": 89117,
    "resolution": "720p",
    "ratio": "16:9",
    "duration": 6,
    "frames": 144,
    "framespersecond": 24,
    "tools": [
      {"type": "web_search"}
    ],
    "safety_identifier": "user-hash",
    "service_tier": "default",
    "execution_expires_after": 172800,
    "generate_audio": true,
    "draft": false,
    "draft_task_id": "cgt-draft"
  }
}
```

字段映射：

| 上游字段 | 下游字段 | 说明 |
|----------|----------|------|
| `id` | `upstream_id` | 上游任务 ID，仅排查使用 |
| 本地 `Task.TaskID` | `id`, `task_id` | 查询、取消、回调幂等使用 |
| `model` | `model` | 优先上游，兜底本地原始模型名 |
| `status` | `status` | 优先上游，兜底本地状态映射 |
| `error` | `error` | 成功时通常是 `null` |
| `content.video_url` | `content.video_url` | 为空时兜底 `Task.ResultURL` |
| `content.last_frame_url` | `content.last_frame_url` | 创建时 `return_last_frame=true` 才可能有 |
| `usage.completion_tokens` | `usage.completion_tokens` | 视频 token 计费依据 |
| `usage.total_tokens` | `usage.total_tokens` | 为空时用 completion tokens 兜底 |
| `usage.tool_usage` | `usage.tool_usage` | 例如 `web_search` 调用次数 |
| `created_at` | `created_at` | 为空时兜底本地创建时间 |
| `updated_at` | `updated_at` | 为空时兜底本地更新时间 |
| `seed` | `seed` | 直接透出 |
| `resolution` | `resolution` | 直接透出 |
| `ratio` | `ratio` | 直接透出 |
| `duration` | `duration` | 与 `frames` 通常二选一 |
| `frames` | `frames` | 与 `duration` 通常二选一 |
| `framespersecond` | `framespersecond` | 保持上游字段名 |
| `tools` | `tools` | 直接透出 |
| `safety_identifier` | `safety_identifier` | 创建时传了才返回 |
| `service_tier` | `service_tier` | 直接透出 |
| `execution_expires_after` | `execution_expires_after` | 直接透出 |
| `generate_audio` | `generate_audio` | 指针字段，显式 `false` 会保留 |
| `draft` | `draft` | 指针字段，显式 `false` 会保留 |
| `draft_task_id` | `draft_task_id` | 直接透出 |
| `cost` | 不返回 | 内部结算和排查可看 `Task.Data`，不向下游暴露 |
| 原始上游完整响应 | 不返回 | 防止出现 `data.data` 嵌套和无关字段 |

## 8. 状态映射

### 8.1 查询响应状态

下游 `data.status` 尽量保留上游状态：

| 上游状态 | 下游状态 |
|----------|----------|
| `queued` | `queued` |
| `running` | `running` |
| `succeeded` | `succeeded` |
| `failed` | `failed` |
| `cancelled` | `cancelled` |
| `expired` | `expired` |

如果没有上游状态，则按本地 `TaskStatus` 兜底：成功 `succeeded`，失败 `failed`，已提交/排队 `queued`，其他 `running`。

### 8.2 内部 TaskStatus 映射

`ParseTaskResult` 映射如下：

| ZLHub 状态 | new-api 内部状态 | Progress | 计费动作 |
|------------|------------------|----------|----------|
| `queued` | `TaskStatusQueued` | `10%` | 继续等待 |
| `running` | `TaskStatusInProgress` | `50%` | 继续等待 |
| `succeeded` | `TaskStatusSuccess` | `100%` | 触发完成结算 |
| `failed` | `TaskStatusFailure` | `100%` | 退款 |
| `cancelled` | `TaskStatusFailure` | `100%` | 退款 |
| `expired` | `TaskStatusFailure` | `100%` | 退款 |
| 空/未知 | `TaskStatusInProgress` | `30%` | 继续等待 |

## 9. 回调

### 9.1 上游回调到平台

平台注入给上游的回调地址：

```text
{ServerAddress}/api/task/callback/zlhub/video
```

旧地址仍保留：

```text
{ServerAddress}/api/zlhub/callback/video
```

`controller.ZLHubVideoCallback` 会：

1. 读取上游回调 body。
2. 调 `service.HandleVideoTaskCallback`。
3. 从 body 中提取上游任务 ID，支持根级 `id/task_id` 和 `data.id/data.task_id`。
4. 根据 `Task.PrivateData.UpstreamTaskID` 找本地任务。
5. 调 adaptor `ParseTaskResult`。
6. 调 `ApplyVideoTaskResult` 更新任务、结算或退款。
7. 如果用户创建时传了 `callback_url`，异步推送用户回调。

### 9.2 平台回调到用户

用户 `callback_url` 规则：

| 项目 | 值 |
|------|----|
| 保存位置 | `Task.PrivateData.CallbackURL` |
| URL 校验 | 只允许 `http` / `https` |
| 请求方法 | POST |
| Content-Type | `application/json` |
| Header | `X-New-Api-Task-Id: task_xxx` |
| 超时 | 5 秒 |
| 重试 | 最多 3 次 |
| 成功条件 | HTTP 2xx |
| 请求体 | 与 `GET /v1/task/get/{task_id}` 响应一致 |

用户回调先更新本地任务再发送，因此用户收到的响应体是清洗后的平台标准结果，不是上游原始 body。

## 10. 轮询

轮询是回调兜底机制，核心在 `service/task_polling.go`：

```text
TaskPollingLoop
  -> GetAllUnFinishSyncTasks
  -> UpdateVideoTasks
  -> updateVideoSingleTask
  -> adaptor.FetchTask
  -> adaptor.ParseTaskResult
  -> ApplyVideoTaskResult
```

关键策略：

| 项目 | 说明 |
|------|------|
| 全局扫描间隔 | 15 秒 |
| ZLHub 单任务上游查询间隔 | 至少 60 秒 |
| 节流字段 | `Task.PrivateData.LastPollAt` |
| 超时控制 | `TaskTimeoutMinutes` |
| 终态处理 | 成功结算，失败/取消/超时退款 |

ZLHub 60 秒节流只限制后台轮询，不限制用户主动调用 `/v1/task/get/{task_id}` 时的实时上游查询。

## 11. 取消任务

对外取消必须使用本地任务 ID：

```http
POST /v1/task/cancel/{task_xxx}
```

`controller.CancelZLHubTask` 的流程：

1. 按本地 `task_xxx` 查 `Task`。
2. 校验任务属于当前用户。
3. 校验渠道类型是 ZLHub。
4. 从 `Task.PrivateData.UpstreamTaskID` 获取上游 `cgt-xxx`。
5. 调 ZLHub `/v1/task/cancel/{cgt-xxx}`。
6. 本地标记失败/取消，并按预扣额度退款。

透传取消 `/api/zlhub/v1/task/cancel/{cgt-xxx}` 不处理本地退款，只用于内部排查。

## 12. 计费

### 12.1 预扣

任务创建时 `EstimateBilling` 会构造 `OtherRatios`：

| key | 来源 | 用途 |
|-----|------|------|
| `estimated_tokens` | 官方视频 token 估算公式 | 只用于模型倍率计费的预扣估算；token 重算时自动排除 |
| `seconds` | 请求 `duration`，没有则默认 `5` | 仅 `ModelPrice` 价格模式使用；token 重算时自动排除 |
| `seedance_price` | 模型、分辨率、输入是否包含视频 | Seedance 2.0 官方价格修正 |

模型倍率计费下，预扣不再按 `duration` 直接放大，而是按火山官方视频 token 估算：

```text
estimated_tokens = (输出帧数 + 估算输入视频帧数) * 输出宽 * 输出高 / 1024
输出帧数 = frames；未传 frames 时为 duration * 24
估算输入视频帧数 = 0；如果 content 包含 video_url，则按 4 秒 * 24 帧估算
preQuota = estimated_tokens * ModelRatio * groupRatio * seedance_price
```

示例：`720p`、`21:9`、`duration=6`、无输入视频时，像素为 `1470x630`，预扣估算为：

```text
estimated_tokens = 1470 * 630 * 6 * 24 / 1024 = 130232.8125
```

这与上游最终返回的 `usage.completion_tokens` 通常接近，完成后仍以真实 token 做差额结算。

`resolution` 来源优先级：

1. 请求顶层 `resolution`。
2. 原生 `content` body 中的 `resolution`。
3. 历史兼容字段中的 `resolution`。

`ratio` 来源优先级：

1. 请求顶层 `ratio`。
2. 原生 `content` body 中的 `ratio`。
3. 历史兼容字段中的 `ratio`。
4. 历史兼容 `size` 如果看起来像 `16:9` 这类宽高比，则作为 `ratio`。

输入是否包含视频由 `content` 中是否存在 `type=video_url` 或 `video_url` 字段判断。

### 12.2 Seedance 2.0 价格修正

后台建议把基础模型价格配置为官方“480p/720p 且输入不含视频”的单价，再由 `seedance_price` 自动修正：

| 模型 | 基础单价 | 场景 | 修正倍率 |
|------|----------|------|----------|
| `doubao-seedance-2.0` | 46 元/百万 token | 480p/720p，输入不含视频 | `1` |
| `doubao-seedance-2.0` | 46 元/百万 token | 480p/720p，输入包含视频 | `28 / 46` |
| `doubao-seedance-2.0` | 46 元/百万 token | 1080p，输入不含视频 | `51 / 46` |
| `doubao-seedance-2.0` | 46 元/百万 token | 1080p，输入包含视频 | `31 / 46` |
| `doubao-seedance-2.0-fast` | 37 元/百万 token | 输入不含视频 | `1` |
| `doubao-seedance-2.0-fast` | 37 元/百万 token | 输入包含视频 | `22 / 37` |

注意：`doubao-seedance-2.0-fast` 不支持 1080p，上游会校验。

### 12.3 完成结算

完成时优先按上游 usage token 重算：

```text
billableTokens = completion_tokens * completionRatio
actualQuota = billableTokens * modelRatio * groupRatio * otherMultiplier
```

`otherMultiplier` 来自 `BillingContext.OtherRatios`，但会排除 `estimated_tokens`、`seconds`、`duration` 这类只用于预扣估算的倍率。因此：

- 预扣会按官方视频 token 公式估算。
- 成功返回 usage 后按真实 token 重算。
- `seedance_price` 会继续参与 token 重算。
- `estimated_tokens` / `seconds` 不会在 token 重算时再次相乘。

如果上游没有返回 token，但返回了实际 `duration`，adaptor 的 `AdjustBillingOnComplete` 可按实际时长兜底。任务失败、取消或超时时调用 `RefundTaskQuota` 退还预扣。

### 12.4 计费模式建议

| 后台配置 | 行为 | 建议 |
|----------|------|------|
| 模型倍率 `ModelRatio` | 支持完成后按真实 token 差额结算 | 推荐用于 ZLHub Seedance |
| 模型价格 `ModelPrice` | 更偏按次/固定价格，完成后跳过 token 差额结算 | 不推荐用于按 token 视频模型 |
| 阶梯计费 `tiered_expr` | 表达式定价，适合复杂模型 | 如使用需遵循 `pkg/billingexpr/expr.md` |

后台「按量计费」页里的「输入价格 $/1M tokens」对应 `ModelRatio` 的展示值。前端会按系统额度换算保存，通常展示价格 `7.333` 对应内部 `ModelRatio=3.6665`。后台「按次计费」页配置的是 `ModelPrice`，它适合固定单次价格，不适合 Seedance 这类按 `completion_tokens` 对账的视频模型。

## 13. 素材审核

素材审核不走 relay 任务系统，不创建 `Task`，不计视频任务费用。它通过 `controller/zlhub_asset.go` 初始化 ZLHub adaptor 后直接请求 `https://asset.zlhub.cn`。

### 13.1 接口

| 场景 | 方法 | 对外路径 | 上游路径 |
|------|------|----------|----------|
| 同步提交审核 | POST | `/v1/asset/upload/sync` | `/api/asset/upload/sync` |
| 异步提交审核 | POST | `/v1/asset/upload/async` | `/api/asset/upload/async` |
| 查询审核结果 | GET | `/v1/asset/task/{task_id}` | `/api/task/{task_id}` |
| 兼容提交 | POST | `/api/zlhub/asset/upload` | 按 `async` 选择 sync/async |
| 兼容查询 | GET | `/api/zlhub/asset/task/{task_id}` | `/api/task/{task_id}` |

### 13.2 请求体

```json
{
  "images": ["https://example.com/photo1.jpg"],
  "asset_type": "Image"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `images` | string[] | 素材 URL 列表，最多 50 条 |
| `asset_type` | string | `Image` / `Video` / `Audio`，默认 `Image` |
| `async` | bool | 仅兼容入口 `/api/zlhub/asset/upload` 使用 |

素材审核 callback 固定为：

```text
{ServerAddress}/api/zlhub/asset/callback
```

当前素材审核回调只记录日志，不更新本地任务，因为素材审核没有本地 Task。

### 13.3 审核结果关键字段

| 字段 | 说明 |
|------|------|
| `submit_review_status` | `1` 通过，`0` 失败 |
| `asset_url` | `Asset://` 地址，可直接放入视频生成 `content.*.url` |
| `downstream_asset_id` | 火山素材 ID |
| `downstream_final_url` | 临时访问地址 |
| `error_code` / `error_message` | 审核失败原因 |

## 14. 数据安全和清洗

下游查询和用户回调不会返回：

| 字段 | 原因 |
|------|------|
| `cost` | 上游成本信息，不作为对外业务字段 |
| 原始上游完整 body | 防止泄露内部结构 |
| `data.data` 嵌套 | 对外统一扁平到 `data` |
| DB 主键、用户 ID、渠道 ID、quota | 平台内部字段 |

`redactVideoResponseBody` 会对部分 base64 视频响应做截断和字段删除，避免任务数据中保存过大的 base64 内容。

## 15. 常见问题和排查

| 问题 | 排查点 |
|------|--------|
| 创建成功但查不到任务 | 确认下游使用的是本地 `task_xxx`，不是上游 `cgt-xxx` |
| 用户 callback_url 没收到 | 检查创建请求是否传了合法 http/https URL；检查 `Task.PrivateData.CallbackURL`；看日志中的 3 次回调尝试 |
| 上游没有回调 | 确认 `ServerAddress` 配置非空且公网可达；轮询会兜底 |
| 计费比预期高 | 检查后台是否用 `ModelRatio` 而不是 `ModelPrice`；检查 `usage.completion_tokens`、`estimated_tokens`、`seedance_price`、分组倍率 |
| 6 秒视频按 1M/token 价格显示异常 | 预扣公式是官方视频 token 估算，完成后公式是 `completion_tokens * modelRatio * groupRatio * seedance_price`，不会再乘 `estimated_tokens` 或 `seconds` |
| 1080p fast 报错 | `doubao-seedance-2.0-fast` 上游不支持 1080p |
| 取消后没退款 | 确认走的是 `/v1/task/cancel/{task_xxx}`，不是 `/api/zlhub/v1/task/cancel/{cgt-xxx}` |
| 素材审核没有回调业务方 | 素材审核当前不支持用户自定义 callback_url，异步结果通过查询接口获取 |
| 查询响应缺上游字段 | 先确认 `Task.Data` 中是否有该字段；`VideoTaskResultFromTask` 只透出白名单字段 |

## 16. 测试覆盖

相关测试：

| 文件 | 覆盖点 |
|------|--------|
| `relay/channel/task/zlhub/adaptor_test.go` | ratio/resolution 映射、官方可选字段、内部 callback_url、创建响应包、媒体无文本、OpenAI 兼容转换 |
| `relay/relay_task_test.go` | 查询结果清洗和字段透出 |
| `service/task_callback_test.go` | 回调响应体清洗、上游任务 ID 提取、用户 callback_url 校验 |
| `service/task_billing_test.go` | ZLHub 轮询节流、任务计费兜底 |
| `relay/channel/task/taskcommon/seedance_billing_test.go` | Seedance 2.0 价格修正倍率 |

建议变更后至少运行：

```bash
go test ./relay/channel/task/zlhub -count=1
go test ./relay ./controller ./relay/channel/task/zlhub -count=1
go test ./service -run "Test(VideoTaskResultResponseBodyIsSanitized|ExtractCallbackUpstreamTaskID|NormalizeTaskCallbackURL)$" -count=1
git diff --check
```
