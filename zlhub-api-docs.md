# ZLHub API 接口文档完整总结

---

## 一、视频生成接口（ZLHub）

### 1.1 基础信息

- 域名：`https://api.zlhub.cn`
- Content-Type：`application/json`

### 1.2 请求头

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| Authorization | string | 是 | 鉴权 API_KEY，格式为 `Bearer <API_KEY>` |
| Content-Type | string | 是 | 固定为 `application/json` |
| X-Trace-ID | string | 否 | 请求跟踪ID，32位随机字符串，用于问题排查，每次请求必须不同 |

### 1.3 接口列表

| 接口 | 方法 | URL |
|------|------|-----|
| 创建视频生成任务 | POST | `https://api.zlhub.cn/v1/task/create` |
| 查询视频任务状态 | GET | `https://api.zlhub.cn/v1/task/get/{id}` |
| 取消或删除视频任务 | POST | `https://api.zlhub.cn/v1/task/cancel/{id}` |

### 1.4 接口差异说明

- 为提高系统运行性能，ZLHub 接口**不支持 base64 格式**的图片
- 需先将素材上传到云存储（TOS/OSS）等，获取公网链接后再传到接口
- 如无云存储，可联系技术提供公共 TOS 云存储

### 1.5 任务流程

1. 调用创建视频生成任务接口创建任务
2. 创建任务时设置 `callback_url` 接收任务状态变化回调
3. 定时使用查询接口查询任务状态（可选，作为辅助）
   - 建议任务创建 10 分钟后还未收到状态回调时主动查询
   - 每个任务每分钟查询一次，**严禁频繁查询**
   - 任务 running → 过段时间再查询
   - 任务完成 → 返回视频链接，**24 小时内**下载生成的视频文件

### 1.6 请求示例

#### 1.6.1 创建视频任务（多模态参考）

```bash
curl -X POST https://api.zlhub.cn/v1/task/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Trace-ID: $TRACE_ID" \
  -d '{
    "model": "doubao-seedance-2.0",
    "content": [
         {
            "type": "text",
            "text": "全程使用视频1的第一视角构图，全程使用音频1作为背景音乐。第一人称视角果茶宣传广告..."
        },
        {
            "type": "image_url",
            "image_url": {
                "url": "https://ark-project.tos-cn-beijing.volces.com/doc_image/r2v_tea_pic1.jpg"
            },
            "role": "reference_image"
        },
        {
            "type": "image_url",
            "image_url": {
                "url": "https://ark-project.tos-cn-beijing.volces.com/doc_image/r2v_tea_pic2.jpg"
            },
            "role": "reference_image"
        },
        {
          "type": "video_url",
          "video_url": {
              "url": "https://ark-project.tos-cn-beijing.volces.com/doc_video/r2v_tea_video1.mp4"
          },
          "role": "reference_video"
        },
        {
          "type": "audio_url",
          "audio_url": {
              "url": "https://ark-project.tos-cn-beijing.volces.com/doc_audio/r2v_tea_audio1.mp3"
          },
          "role": "reference_audio"
        }
    ],
    "generate_audio": true,
    "ratio": "16:9",
    "duration": 11,
    "watermark": false
}'
```

#### 1.6.2 首尾帧生成

```bash
curl -X POST https://api.zlhub.cn/v1/task/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Trace-ID: $TRACE_ID" \
  -d '{
    "model": "doubao-seedance-2.0",
    "content": [
        {
            "type": "text",
            "text": "根据首帧和尾帧图片，生成流畅过渡的高清视频"
        },
        {
            "type": "image_url",
            "image_url": {
                "url": "shturl.cc/TEGz0jMcrNvHl7Ame2VgXGEvMo3tT"
            },
            "role": "first_frame"
        },
        {
            "type": "image_url",
            "image_url": {
                "url": "shturl.cc/cNsvd8699SW5JvU2CV2gIKrLGbp6"
            },
            "role": "last_frame"
        }
    ],
    "generate_audio": true,
    "ratio": "16:9",
    "duration": 8,
    "watermark": false
}'
```

#### 1.6.3 使用已授权真人素材

通过真人认证和本人授权后，可将该真人的相关素材调用素材送审接口，素材入库成功后获得独立素材 ID（asset ID）。

在 `content.<模态>_url.url` 字段中传入素材审核系统返回的 `asset_url` 字段值即可。

```bash
curl -X POST https://api.zlhub.cn/v1/task/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Trace-ID: $TRACE_ID" \
  -d '{
    "model": "doubao-seedance-2.0",
    "content": [
         {
            "type": "text",
            "text": "<your prompt>"
        },
        {
            "type": "image_url",
            "image_url": {
                "url": "Asset://asset-20260421174640-kqkcc"
            },
            "role": "reference_image"
        },
        {
            "type": "video_url",
            "video_url": {
                "url": "Asset://asset-20260421174640-kqkcc"
            },
            "role": "reference_video"
        },
        {
            "type": "audio_url",
            "audio_url": {
                "url": "Asset://asset-20260421174640-kqkcc"
            },
            "role": "reference_audio"
        }
    ]
}'
```

### 1.7 响应示例

#### 1.7.1 创建任务响应

```json
{
    "id": "cgt-20260416141540-t7n9r"
}
```

#### 1.7.2 查询任务响应

```bash
curl -X GET https://api.zlhub.cn/v1/task/get/cgt-20260421174743-w9q85 \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Trace-ID: $TRACE_ID"
```

```json
{
    "code": "success",
    "message": "",
    "data": {
        "content": {
            "last_frame_url": "null",
            "video_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/...?X-Tos-Algorithm=..."
        },
        "cost": {
            "currency": "CNY",
            "input_cost": "0.0000000000",
            "output_cost": "5.0093914000",
            "total_cost": "5.0093914000"
        },
        "created_at": 1776764863,
        "draft": false,
        "duration": 5,
        "execution_expires_after": 172800,
        "framespersecond": 24,
        "generate_audio": true,
        "id": "cgt-20260421174743-w9q85",
        "model": "doubao-seedance-2.0",
        "ratio": "9:16",
        "resolution": "720p",
        "seed": 54405,
        "service_tier": "default",
        "status": "succeeded",
        "updated_at": 1776765155,
        "usage": {
            "completion_tokens": 108900,
            "total_tokens": 108900
        }
    }
}
```

### 1.8 响应字段说明

> 注：该响应结构基于火山方舟 Seedance 2.0 原生返回扩展，额外增加 zlhub 平台 cost 完整计费对象。顶层 `code`、`message` 为平台新增字段，火山方舟原始接口无此结构。

| 字段路径 | 字段名 | 类型 | 说明 |
|----------|--------|------|------|
| code | 响应码 | string | 接口状态，success 表示成功 |
| message | 提示信息 | string | 错误/提示文案，成功时为空 |
| data | 业务数据 | object | 视频生成结果主体 |
| data.content.last_frame_url | 末帧图片地址 | string | 视频最后一帧图链接 |
| data.content.video_url | 视频地址 | string | 生成视频的 TOS 下载链接（带签名，有时效） |
| data.cost | 计费信息 | object | 完整计费明细 |
| data.cost.currency | 货币单位 | string | 货币类型，CNY 为人民币 |
| data.cost.input_cost | 输入费用 | string | 输入内容计费金额 |
| data.cost.output_cost | 输出费用 | string | 生成视频计费金额 |
| data.cost.total_cost | 总费用 | string | 本次请求总花费 |
| data.created_at | 创建时间戳 | number | 任务创建时间（Unix 秒级时间戳） |
| data.draft | 是否草稿 | boolean | 是否为草稿任务，false 为正式任务 |
| data.duration | 视频时长 | number | 生成视频时长，单位秒 |
| data.execution_expires_after | 执行过期时间 | number | 任务执行超时时间，单位秒 |
| data.framespersecond | 帧率 | number | 视频帧率，FPS |
| data.generate_audio | 是否生成音频 | boolean | 是否附带生成音频 |
| data.id | 任务 ID | string | 本次视频生成唯一任务标识 |
| data.model | 模型名称 | string | 使用的生成模型 |
| data.ratio | 视频比例 | string | 画面宽高比 |
| data.resolution | 分辨率 | string | 视频分辨率 |
| data.seed | 随机种子 | number | 生成随机种子，用于复现结果 |
| data.service_tier | 服务等级 | string | 服务档位 |
| data.status | 任务状态 | string | queued/running/cancelled/succeeded/failed/expired |
| data.error | 错误信息 | object/null | 错误提示对象，成功时返回 null |
| data.error.code | 错误码 | string | 火山原生错误码（如 InvalidParameter、QuotaExceeded） |
| data.error.message | 错误提示信息 | string | 火山原生错误描述 |
| data.updated_at | 更新时间戳 | number | 任务最后更新时间戳 |
| data.usage.completion_tokens | 生成 tokens | number | 生成消耗 tokens |
| data.usage.total_tokens | 总 tokens | number | 本次总消耗 tokens |

### 1.9 任务状态说明

| 状态 | 说明 |
|------|------|
| queued | 排队中 |
| running | 运行中 |
| cancelled | 已取消 |
| succeeded | 成功 |
| failed | 失败 |
| expired | 超时 |

### 1.10 提示词技巧

#### 素材引用格式

提示词中必须使用 **"素材类型+序号"** 格式引用素材，序号为请求体中该素材在同类素材中的排序。

- 「图片 n」→ content 数组中第 n 个 `type="image_url"` 的参考图片（从 1 开始计数）
- 「视频 n」→ content 数组中第 n 个 `type="video_url"` 的参考视频
- 「音频 n」→ content 数组中第 n 个 `type="audio_url"` 的参考音频
- **不支持使用 Asset ID 指代素材**

#### 多模态参考

- **图片参考**：`参考 / 提取 / 结合 +「图片 n」中的「主体 / 被参考元素描述」，生成「画面描述」，保持「主体 / 被参考元素描述」特征一致。`
- **视频参考**：`参考「视频 n」的「动作描述 / 运镜描述 / 特效描述」，生成「画面描述」，保持动作细节 / 运镜 / 特效一致。`
- **音频参考**：
  - 音色参考：`「角色」说："「台词」，音色参考「音频 n」。`
  - 音频内容参考：`理想出现时机 +「音频 n」。`

#### 编辑视频

- **增加元素**：清晰描述「元素特征」+「出现时机」+「出现位置」
- **删除元素**：点明需要删除的元素，对于保持不变的元素在提示词中加以强调
- **修改元素**：清晰描述更换元素即可

#### 延长视频

- **延长视频**：`向前/向后延长「视频n」+「需延长的视频描述」`
- **轨道补全**：`「视频1」+「过渡画面描述」+接「视频2」+「过渡画面描述」+接「视频3」`

---

## 二、素材审核接口

### 2.1 基础信息

- 域名：`https://asset.zlhub.cn`
- Content-Type：`application/json`

### 2.2 请求头

| 参数名 | 必填 | 说明 |
|--------|------|------|
| X-Access-Token | 是 | 访问令牌（非大模型秘钥，由管理员分配） |
| X-Track-Id | 是 | 请求跟踪ID，32位无横杠十六进制字符串，每次请求必须唯一 |
| Content-Type | 是 | `application/json` |

**X-Track-Id 生成方式：**

| 语言 | 代码 |
|------|------|
| Python | `uuid.uuid4().hex` |
| Java | `UUID.randomUUID().toString().replace("-", "")` |
| JavaScript | `crypto.randomUUID().replaceAll('-', '')` |

### 2.3 接口列表

| 接口 | 方法 | URL | 说明 |
|------|------|-----|------|
| 同步提交审核 | POST | `/api/asset/upload/sync` | 等待审核完成返回结果 |
| 异步提交审核 | POST | `/api/asset/upload/async` | 立即返回任务ID |
| 查询任务结果 | GET | `/api/task/:task_id` | 查询审核结果 |

### 2.4 同步与异步接口差异

#### 方案 A：同步提交 (`/upload/sync`)

- **响应机制**：服务端挂起等待审核完成，直接返回审核结果（HTTP 200）。若等待超过 60 秒未完成，则降级返回任务 ID（HTTP 202）
- **结果获取**：大多数情况下直接在响应中获取，仅在超时降级时需轮询查询或等待回调
- **适用场景**：
  1. 单个素材提审
  2. C端用户交互（如用户上传头像需立即反馈）
  3. 需要立即获知审核结果以进行下一步业务逻辑的场景
- **最佳实践**：建议将客户端的请求超时时间设置在 60 秒以上，防止主动断连

#### 方案 B：异步提交 (`/upload/async`)

- **响应机制**：提交后立即返回任务 ID（HTTP 202），不包含审核结果
- **结果获取**：支持主动轮询（调用查询接口）和被动接收（配置 `callback_url` 接收服务端推送）
- **适用场景**：
  1. 批量提交大量素材
  2. 后台定时或离线处理任务
  3. 对吞吐量要求高、对单次响应时间要求不高的场景
- **最佳实践**：建议轮询间隔设置在 3 秒左右

### 2.5 请求参数

```json
{
    "images": ["https://example.com/a.jpg", "https://example.com/b.jpg"],
    "asset_type": "Image",
    "callback_url": "https://your-domain.com/api/callback"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| images | string[] | 是 | 素材 URL 列表，最多 50 条，只支持 http/https |
| asset_type | string | 否 | 默认 Image，可选 Image / Video / Audio |
| callback_url | string | 否 | 接收审核结果的回调地址，必须为公网可访问的 HTTP(S) 地址 |

**重要限制：**
- 同一批次所有 URL 必须为同一类型，Image / Video / Audio 不允许混合提交
- 系统通过 URL 扩展名自动推断素材类型，URL 必须带有受支持的扩展名，否则整个请求将被拒绝

**支持的扩展名：**

| 类型 | 支持的扩展名 |
|------|-------------|
| Image | .jpeg .jpg .png .webp .bmp .tiff .tif .gif .heic .heif |
| Video | .mp4 .mov |
| Audio | .wav .mp3 |

### 2.6 请求示例

#### 2.6.1 同步提交

```http
POST /api/asset/upload/sync HTTP/1.1
Host: api.example.com
Content-Type: application/json
X-Access-Token: tk_abc123def456
X-Track-Id: 550e8400e29b41d4a716446655440000

{
    "images": ["https://example.com/a.jpg"],
    "asset_type": "Image",
    "callback_url": "https://your-domain.com/api/callback"
}
```

#### 2.6.2 异步提交

```http
POST /api/asset/upload/async HTTP/1.1
Host: api.example.com
Content-Type: application/json
X-Access-Token: tk_abc123def456
X-Track-Id: 550e8400e29b41d4a716446655440000

{
    "images": ["https://example.com/a.jpg", "https://example.com/b.jpg"],
    "asset_type": "Image",
    "callback_url": "https://your-domain.com/api/callback"
}
```

#### 2.6.3 查询任务结果

```http
GET /api/task/task-20260411120000-a1b2c3d4 HTTP/1.1
Host: api.example.com
X-Access-Token: tk_abc123def456
X-Track-Id: 550e8400e29b41d4a716446655440000
```

### 2.7 响应示例

#### 2.7.1 同步提交 — 审核完成 (HTTP 200)

```json
{
    "code": 200,
    "task_id": "task-20260411120100-e5f6g7h8",
    "status": "completed",
    "result": {
        "review_batch_id": "task-20260411120100-e5f6g7h8",
        "items": [
            {
                "asset_id": "local-20260411120101-1234abcd",
                "source_url": "https://example.com/a.jpg",
                "asset_url": "Asset://Asset-20260411120101-xxxxx",
                "downstream_asset_id": "Asset-20260411120101-xxxxx",
                "downstream_final_url": "https://ark-media-asset.tos-cn-beijing.volces.com/...?X-Tos-Algorithm=...",
                "downstream_url_expire_at": "2026-04-12T00:01:01Z",
                "submit_review_status": 1,
                "asset_type": "Image",
                "error_code": "",
                "error_message": "",
                "createtime": "2026-04-11T12:01:01Z"
            }
        ]
    }
}
```

#### 2.7.2 同步提交 — 超时未完成 (HTTP 202)

```json
{
    "code": 202,
    "task_id": "task-20260411120100-e5f6g7h8",
    "status": "processing",
    "message": "任务已受理，但在超时时间内尚未完成，请稍后通过 GET /api/task/:task_id 查询"
}
```

#### 2.7.3 异步提交 — 成功 (HTTP 202)

```json
{
    "code": 202,
    "task_id": "task-20260411120000-a1b2c3d4",
    "message": "任务已受理"
}
```

#### 2.7.4 查询 — 处理中

```json
{
    "code": 200,
    "task_id": "task-20260411120000-a1b2c3d4",
    "track_id": "550e8400e29b41d4a716446655440000",
    "status": "processing",
    "error_message": "",
    "total_count": 2,
    "done_count": 1,
    "result": null
}
```

#### 2.7.5 查询 — 已完成

```json
{
    "code": 200,
    "task_id": "task-20260411120000-a1b2c3d4",
    "track_id": "550e8400e29b41d4a716446655440000",
    "status": "completed",
    "error_message": "",
    "total_count": 2,
    "done_count": 2,
    "result": {
        "review_batch_id": "task-20260411120000-a1b2c3d4",
        "items": [
            {
                "asset_id": "local-20260411120001-abc12345",
                "source_url": "https://example.com/photo1.jpg",
                "asset_url": "Asset://Asset-20260411120001-xxxxx",
                "downstream_asset_id": "Asset-20260411120001-xxxxx",
                "downstream_final_url": "https://ark-media-asset.tos-cn-beijing.volces.com/...",
                "downstream_url_expire_at": "2026-04-12T00:00:01Z",
                "submit_review_status": 1,
                "asset_type": "Image",
                "error_code": "",
                "error_message": "",
                "createtime": "2026-04-11T12:00:01Z"
            },
            {
                "asset_id": "local-20260411120002-def67890",
                "source_url": "https://example.com/photo2.jpg",
                "asset_url": "Asset://Asset-20260411120002-yyyyy",
                "downstream_asset_id": "Asset-20260411120002-yyyyy",
                "downstream_final_url": "https://ark-media-asset.tos-cn-beijing.volces.com/...",
                "downstream_url_expire_at": "2026-04-12T00:00:02Z",
                "submit_review_status": 1,
                "asset_type": "Image",
                "error_code": "",
                "error_message": "",
                "createtime": "2026-04-11T12:00:02Z"
            }
        ]
    }
}
```

#### 2.7.6 查询 — 部分素材审核失败

```json
{
    "code": 200,
    "task_id": "task-20260411120000-a1b2c3d4",
    "status": "completed",
    "total_count": 2,
    "done_count": 2,
    "result": {
        "review_batch_id": "task-20260411120000-a1b2c3d4",
        "items": [
            {
                "asset_id": "local-20260411120001-abc12345",
                "source_url": "https://example.com/good.jpg",
                "asset_url": "Asset://Asset-20260411120001-xxxxx",
                "downstream_asset_id": "Asset-20260411120001-xxxxx",
                "downstream_final_url": "https://ark-media-asset.tos-cn-beijing.volces.com/...",
                "downstream_url_expire_at": "2026-04-12T00:00:01Z",
                "submit_review_status": 1,
                "asset_type": "Image",
                "error_code": "",
                "error_message": ""
            },
            {
                "asset_id": "",
                "source_url": "https://example.com/bad.jpg",
                "asset_url": "",
                "downstream_asset_id": "",
                "downstream_final_url": "",
                "downstream_url_expire_at": "",
                "submit_review_status": 0,
                "asset_type": "Image",
                "error_code": "InvalidImageFormat",
                "error_message": "图片格式不支持"
            }
        ]
    }
}
```

### 2.8 结果字段说明

| 字段 | 说明 |
|------|------|
| asset_id | 系统生成的素材 ID |
| source_url | 您提交的原始 URL |
| asset_url | `Asset://` 协议地址，可直接用于视频生成 API |
| downstream_asset_id | 火山引擎素材 ID |
| downstream_final_url | 火山引擎生成的带签名访问地址（有效期 12 小时，过期后查询时自动刷新） |
| downstream_url_expire_at | downstream_final_url 的过期时间（RFC3339 格式） |
| submit_review_status | 1 = 审核通过，0 = 未通过或失败 |
| asset_type | 素材类型：Image / Video / Audio |
| error_code | 审核失败时的错误码（成功时为空字符串） |
| error_message | 审核失败时的错误描述（成功时为空字符串） |
| createtime | 火山引擎侧素材创建时间 |

### 2.9 回调机制（Callback）

在调用同步提交或异步提交接口时，如果在请求体中携带了有效的 `callback_url` 参数，当整个任务审核完成（或因严重错误提前结束）时，系统会向该 URL 发起 POST 回调请求。

**回调请求格式：**

```json
{
    "code": 200,
    "task_id": "task-20260411120000-a1b2c3d4",
    "track_id": "550e8400e29b41d4a716446655440000",
    "status": "completed",
    "total_count": 2,
    "done_count": 2,
    "result": {
        "review_batch_id": "task-20260411120000-a1b2c3d4",
        "items": [
            {
                "asset_id": "local-20260411120001-abc12345",
                "source_url": "https://example.com/photo1.jpg",
                "submit_review_status": 1
            }
        ]
    }
}
```

**回调响应要求：**
- 处理完回调数据后，必须返回 HTTP 状态码 200
- **超时重试**：如果接口响应超时（默认超过 5 秒）或返回非 2xx 状态码，系统可能会进行有限次数的退避重试
- **异步处理**：建议回调接收接口仅做数据保存/状态更新，不要在接口内执行耗时操作

### 2.10 使用素材生成视频

审核通过后，将 `downstream_asset_id` 按 `asset://<id>` 格式拼接，传入 Seedance 2.0 视频生成 API：

```json
{
    "type": "image_url",
    "image_url": {
        "url": "asset://Asset-20260411120001-xxxxx"
    },
    "role": "reference_image"
}
```

---

## 三、错误码汇总

### 3.1 视频生成接口

由响应体 `code` 字段标识，`success` 表示成功。失败时 `data.error` 包含错误详情：

- `data.error.code`：火山原生错误码（如 InvalidParameter、QuotaExceeded）
- `data.error.message`：火山原生错误描述

### 3.2 素材审核接口

| code | 说明 |
|------|------|
| 200 | 成功 |
| 202 | 已接收，处理中 |
| 400 | 参数错误、Track ID 格式错误、URL 扩展名不受支持、混合类型素材等 |
| 401 | 令牌无效或用户已禁用 |
| 403 | 任务不属于当前用户 |
| 404 | 任务不存在 |
| 429 | 当前IP请求频率超限 |
| 500 | 服务内部错误 |

**素材审核错误场景：**

| 错误场景 | 响应内容示例 |
|----------|-------------|
| 请求体格式错误 | `{"code": 400, "message": "请求体格式错误"}` |
| 令牌无效 | `{"code": 401, "message": "访问令牌无效或用户已禁用"}` |
| Track ID 格式错误 | `{"code": 400, "message": "X-Track-Id 格式错误..."}` |
| 请求频率超限 | `{"code": 429, "message": "当前IP请求频率超限"}` |
| 素材列表为空 | `{"code": 400, "message": "素材URL列表不能为空"}` |
| 超过数量限制 | `{"code": 400, "message": "单次最多提交50条素材"}` |
| 包含 base64 内容 | `{"code": 400, "message": "不支持base64内容，请提交URL地址"}` |
| URL 扩展名不受支持 | `{"code": 400, "message": "第1条URL文件类型不受支持..."}` |
| 混合类型素材 | `{"code": 400, "message": "不支持混合类型素材..."}` |
| URL 类型与 asset_type 不符 | `{"code": 400, "message": "第1条URL为Video素材..."}` |
| 服务内部错误 | `{"code": 500, "message": "创建任务失败"}` |

---

## 四、素材参数限制

### 4.1 图像要求

| 项目 | 要求 |
|------|------|
| 格式 | jpeg、png、webp、bmp、tiff、gif、heic / heif |
| 宽高比（宽/高） | (0.4, 2.5) |
| 宽高长度（px） | (300, 6000) |
| 大小 | 单张图片 < 30 MB |

### 4.2 视频要求

| 项目 | 要求 |
|------|------|
| 格式 | mp4、mov |
| 分辨率 | 480p、720p |
| 时长 | [2, 15] 秒 |
| 尺寸（宽高比） | [0.4, 2.5] |
| 尺寸（宽高长度） | [300, 6000] px |
| 尺寸（总像素数） | 宽×高 需在 [409600, 927408] 区间 |
| 大小 | 单个视频 ≤ 50 MB |
| 帧率 (FPS) | [24, 60] |

### 4.3 音频要求

| 项目 | 要求 |
|------|------|
| 格式 | wav、mp3 |
| 时长 | [2, 15] 秒 |
| 大小 | 单个音频 ≤ 15 MB |

---

## 五、注意事项

### 5.1 接口调用注意事项

- **公网访问限制**：素材 URL 必须是公网可访问的 http/https 地址，不支持 base64 内联内容
- **数量限制**：单次最多提交 50 条素材 URL
- **URL 时效性**：`downstream_final_url` 有效期 12 小时，过期后查询时系统自动刷新
- **性能建议**：建议每次请求只传 1 张素材，多张素材请并发提交多个请求（单次请求包含多张素材时服务端会串行处理，整体耗时较长）
- **轮询频率**：提交后约 10~15 秒返回审核结果，建议轮询查询的间隔为 3 秒
- **视频生成查询**：任务创建 10 分钟后还未收到回调时可主动查询，每个任务每分钟查询一次，严禁频繁查询
- **视频下载时效**：任务完成后 24 小时内下载生成的视频文件

---

## 六、完整调用示例（Python）

### 6.1 素材审核流程

```python
import requests, json, time

# ── 配置（替换为实际值）──
API_BASE = "https://asset.zlhub.cn"
TOKEN = "your_access_token"

# ── 第一步：提交审核 ──
resp = requests.post(
    f"{API_BASE}/api/asset/upload/async",
    json={
        "images": ["https://example.com/photo.jpg"],
        "asset_type": "Image"
    },
    headers={
        "X-Access-Token": TOKEN,
        "Content-Type": "application/json"
    }
)
print(f"HTTP状态: {resp.status_code}")

result = resp.json()
print(f"响应: {json.dumps(result, ensure_ascii=False, indent=2)}")

task_id = result.get("task_id")
if not task_id:
    print(f"错误: {result}")
    exit(1)

# 从响应头获取 Track ID（服务端自动生成）
track_id = resp.headers.get("X-Track-Id", "")
print(f"任务ID: {task_id}")
print(f"Track ID: {track_id}")

# ── 第二步：轮询查询结果 ──
for i in range(60):
    resp = requests.get(
        f"{API_BASE}/api/task/{task_id}",
        headers={"X-Access-Token": TOKEN}
    )
    result = resp.json()
    status = result.get("status")
    print(f"轮询#{i+1} 状态: {status} (已完成 {result.get('done_count',0)}/{result.get('total_count',0)})")

    if status == "completed":
        print(f"\n审核完成！完整结果:")
        print(json.dumps(result, ensure_ascii=False, indent=2))
        for item in result["result"]["items"]:
            if item["error_code"]:
                print(f"  [失败] {item['source_url']} → {item['error_code']}: {item['error_message']}")
            else:
                print(f"  [通过] {item['source_url']} → asset://{item['downstream_asset_id']}")
        break

    if status == "failed":
        print(f"任务失败: {result.get('error_message', '')}")
        break

    time.sleep(3)

# ── 第三步：使用素材生成视频 ──
# 取审核通过的素材 ID，拼接 asset:// 协议地址
asset_ref = f"asset://{result['result']['items'][0]['downstream_asset_id']}"

# 传入 Seedance 2.0 视频生成 API 的 image_url 字段
print(f"视频生成引用地址: {asset_ref}")
# 输出: 视频生成引用地址: asset://Asset-20260411120001-xxxxx
```
