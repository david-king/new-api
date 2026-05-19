# ZLHub 渠道集成指南

本指南说明如何通过 new-api 使用 ZLHub 的视频生成与素材审核服务。详细字段说明请参阅 [ZLHub API 接口文档](../../zlhub-api-docs.md)。

## 渠道配置

| 项目 | 值 |
|------|------|
| 渠道类型 | ZLHub（编号 58） |
| 默认 Base URL | `https://api.zlhub.cn` |
| 支持模型 | `doubao-seedance-2.0` |

### 密钥格式

ZLHub 使用两套独立凭证，用 `|` 分隔：

| 场景 | 密钥格式 | 示例 |
|------|----------|------|
| 仅视频生成 | `video_api_key` | `sk-abc123` |
| 视频 + 素材审核 | `video_api_key\|asset_access_token` | `sk-abc123\|tk-xyz789` |
| 两 Key 相同 | `key` | `sk-abc123`（自动复用） |

> 视频生成 API 使用 `Authorization: Bearer <key>`，素材审核 API 使用 `X-Access-Token: <token>`。new-api 会根据接口自动选用。

## 视频生成 API

视频生成接口采用**原生透传**方式：请求体原样转发到 ZLHub 上游，响应原样返回，只负责加认证和追踪头。

### 接口列表

与 ZLHub 上游 API 格式完全一致。创建任务时自动写入 Task 记录并预扣额度，完成后由轮询系统自动结算。

| 操作 | 方法 | URL | 对应上游 |
|------|------|-----|----------|
| 创建视频任务 | POST | `/api/zlhub/v1/task/create` | `https://api.zlhub.cn/v1/task/create` |
| 查询视频任务 | GET | `/api/zlhub/v1/task/get/{id}` | `https://api.zlhub.cn/v1/task/get/{id}` |
| 取消视频任务 | POST | `/api/zlhub/v1/task/cancel/{id}` | `https://api.zlhub.cn/v1/task/cancel/{id}` |

所有视频接口需要 new-api Token 认证（`Authorization: Bearer <token>`）。`X-Trace-ID` 优先从请求头透传，未传时自动生成。

### 创建视频任务

```
POST /api/zlhub/v1/task/create
```

请求体与上游完全一致，详见 [API 文档 §1.6](../../zlhub-api-docs.md)。

**请求示例：**

```bash
curl -X POST http://your-server/api/zlhub/v1/task/create \
  -H "Authorization: Bearer <your-new-api-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2.0",
    "content": [
        {
            "type": "text",
            "text": "一个女孩在海边奔跑"
        }
    ],
    "generate_audio": true,
    "ratio": "16:9",
    "duration": 5,
    "watermark": false
}'
```

**多模态参考（图片/视频/音频）：**

```json
{
    "model": "doubao-seedance-2.0",
    "content": [
        {"type": "text", "text": "全程使用视频1的第一视角构图"},
        {"type": "image_url", "image_url": {"url": "https://example.com/img1.jpg"}, "role": "reference_image"},
        {"type": "video_url", "video_url": {"url": "https://example.com/vid1.mp4"}, "role": "reference_video"},
        {"type": "audio_url", "audio_url": {"url": "https://example.com/audio1.mp3"}, "role": "reference_audio"}
    ],
    "generate_audio": true,
    "ratio": "16:9",
    "duration": 11,
    "watermark": false
}
```

**首尾帧生成：**

```json
{
    "model": "doubao-seedance-2.0",
    "content": [
        {"type": "text", "text": "根据首帧和尾帧图片，生成流畅过渡的高清视频"},
        {"type": "image_url", "image_url": {"url": "https://example.com/first.jpg"}, "role": "first_frame"},
        {"type": "image_url", "image_url": {"url": "https://example.com/last.jpg"}, "role": "last_frame"}
    ],
    "generate_audio": true,
    "ratio": "16:9",
    "duration": 8,
    "watermark": false
}
```

**使用审核通过的素材（`Asset://`）：**

```json
{
    "model": "doubao-seedance-2.0",
    "content": [
        {"type": "text", "text": "参考图片1中的人物"},
        {"type": "image_url", "image_url": {"url": "Asset://Asset-20260411120001-xxxxx"}, "role": "reference_image"}
    ]
}
```

**响应示例：**

```json
{
    "id": "cgt-20260416141540-t7n9r"
}
```

### 查询视频任务

```
GET /api/zlhub/v1/task/get/{task_id}
```

响应格式与上游完全一致，详见 [API 文档 §1.7.2](../../zlhub-api-docs.md)。

### 取消视频任务

```
POST /api/zlhub/v1/task/cancel/{task_id}
```

无需请求体。取消后任务状态变为 `cancelled`。

**响应示例：**

```json
{
    "code": "success",
    "message": "",
    "data": {
        "id": "cgt-20260421174743-w9q85",
        "status": "cancelled"
    }
}
```

### 透传 X-Trace-ID

视频接口的 `X-Trace-ID` 请求头会被原样透传给 ZLHub。未传时系统自动生成 32 位随机字符串。

### 回调

视频生成完成后，ZLHub 自动回调 `{ServerAddress}/api/zlhub/callback/video`。

如需自定义回调地址，在创建任务请求体中传入 `callback_url` 字段即可。

### 可选查询参数

所有视频接口支持 `channel_id` 查询参数指定 ZLHub 渠道 ID，不指定则自动选择第一个启用的 ZLHub 渠道。

### 任务状态

| 状态 | 说明 |
|------|------|
| queued | 排队中 |
| running | 运行中 |
| cancelled | 已取消 |
| succeeded | 成功 |
| failed | 失败 |
| expired | 超时 |

### 注意事项

1. **不支持 base64**：ZLHub 接口不支持 base64 格式的图片，素材必须是公网 URL 或 `Asset://` 协议地址
2. **查询频率**：创建 10 分钟后未收到回调可主动查询，每分钟最多查询一次
3. **下载时效**：任务完成后 24 小时内下载

## 素材审核 API

素材审核用于将图片/视频/音频素材提交审核，审核通过后获得的 `Asset://` 地址可直接用于视频生成。

### 接口列表

| 操作 | 方法 | URL | 对应上游 |
|------|------|-----|----------|
| 同步提交审核 | POST | `/api/zlhub/asset/upload?async=false` | `https://asset.zlhub.cn/api/asset/upload/sync` |
| 异步提交审核 | POST | `/api/zlhub/asset/upload?async=true` | `https://asset.zlhub.cn/api/asset/upload/async` |
| 查询审核结果 | GET | `/api/zlhub/asset/task/{task_id}` | `https://asset.zlhub.cn/api/task/{task_id}` |

所有素材审核接口需要 new-api Token 认证，`X-Access-Token` 由系统根据渠道 Key 自动填充。

### 提交审核

**请求体：**

```json
{
    "images": ["https://example.com/photo1.jpg", "https://example.com/photo2.jpg"],
    "asset_type": "Image",
    "async": false
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| images | string[] | 是 | 素材 URL 列表（最多 50 条），仅 http/https，不支持 base64 |
| asset_type | string | 否 | `Image` / `Video` / `Audio`，默认 `Image` |
| async | bool | 否 | `true`=异步（立即返回任务 ID），`false`=同步（等待审核结果，默认） |

同步提交超时 60 秒降级为异步模式。完整响应格式见 [API 文档 §2.7](../../zlhub-api-docs.md)。

### 查询审核结果

```
GET /api/zlhub/asset/task/{task_id}
```

### 回调

素材审核完成后自动回调 `{ServerAddress}/api/zlhub/asset/callback`，无需手动配置。

### 审核结果关键字段

| 字段 | 说明 |
|------|------|
| `submit_review_status` | 1 = 通过，0 = 失败 |
| `asset_url` | `Asset://` 协议地址，可直接用于视频生成 |
| `downstream_asset_id` | 火山引擎素材 ID，用于拼接 `asset://` 地址 |
| `downstream_final_url` | 带签名的临时访问地址（12 小时有效） |
| `error_code` / `error_message` | 审核失败时的错误码和描述 |

## 素材类型与格式要求

| 类型 | 支持格式 | 大小限制 | 其他 |
|------|----------|----------|------|
| Image | jpeg, jpg, png, webp, bmp, tiff, tif, gif, heic, heif | <30MB | 宽高比 0.4~2.5，宽高 300~6000px |
| Video | mp4, mov | ≤50MB | 480p/720p, 2~15s, FPS 24~60, 总像素 409600~927408 |
| Audio | wav, mp3 | ≤15MB | 2~15s |

> **同一批次所有 URL 必须为同一类型**，不允许混合提交。URL 必须带有受支持的扩展名。

## 错误码

### 视频生成

由响应体 `code` 字段标识，`success` 表示成功。失败时 `data.error` 包含错误详情：
- `data.error.code`：火山原生错误码（如 InvalidParameter、QuotaExceeded）
- `data.error.message`：火山原生错误描述

### 素材审核

| code | 说明 |
|------|------|
| 200 | 成功 |
| 202 | 已接收，处理中 |
| 400 | 参数错误 |
| 401 | 令牌无效或用户已禁用 |
| 429 | 当前 IP 请求频率超限 |
| 500 | 服务内部错误 |

## 完整调用示例（Python）

```python
import requests

BASE = "http://your-server/api/zlhub"
TOKEN = "your-new-api-token"
HEADERS = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}

# 创建视频任务（原生透传，请求体与 ZLHub 上游一致）
resp = requests.post(f"{BASE}/v1/task/create", headers=HEADERS, json={
    "model": "doubao-seedance-2.0",
    "content": [{"type": "text", "text": "一个女孩在海边奔跑"}],
    "duration": 5,
    "ratio": "16:9",
    "watermark": False
})
task_id = resp.json()["id"]
print(f"任务已创建: {task_id}")

# 查询任务状态
resp = requests.get(f"{BASE}/v1/task/get/{task_id}", headers=HEADERS)
print(resp.json())

# 取消任务
resp = requests.post(f"{BASE}/v1/task/cancel/{task_id}", headers=HEADERS)
print(resp.json())

# 素材审核（同步）
resp = requests.post(f"{BASE}/asset/upload", headers=HEADERS, json={
    "images": ["https://example.com/photo.jpg"],
    "asset_type": "Image",
    "async": False
})
print(resp.json())
```

## 相关资源

- [ZLHub API 接口文档完整版](../../zlhub-api-docs.md) — 原始字段说明、响应示例、错误码等