# Casdoor 统一支付配置手册

## 适用范围

本文档给 Casdoor 管理员使用，说明如何在 Casdoor 中配置统一支付能力，让公司其他产品通过 Casdoor 创建微信支付订单。

当前生产环境：

- Casdoor 地址：`https://login.gepinkeji.com`
- 统一支付组织：`gepin`
- 已接入应用：
  - `admin/app-token-gepinkeji`，组织为 `gepin`
  - `admin/app-token-gptk`，组织为 `gepin`
- 统一支付接口：`POST /api/external/payment/create`
- 支付成功事件：`payment.paid`

## 配置总览

统一支付需要配置 5 类对象：

| 对象 | 用途 |
| --- | --- |
| Application | 给每个业务系统分配 `clientId` 和 `clientSecret` |
| Cert | 保存微信支付商户证书序列号和商户私钥 |
| Provider | 保存微信支付商户号、AppID、APIv3Key 等支付渠道配置 |
| Product | 作为统一支付模板，限制币种、金额范围和可用支付渠道 |
| Webhook | 支付成功后把 `payment.paid` 推送给业务系统 |

## 一、Application 配置

每个业务系统使用一个独立 Application，不要共用 `clientSecret`。

当前两个应用：

| 业务系统 | Casdoor Application | Organization |
| --- | --- | --- |
| Gepin 科技平台 | `admin/app-token-gepinkeji` | `gepin` |
| GPTK 平台 | `admin/app-token-gptk` | `gepin` |

在 Casdoor 后台进入：

```text
Applications -> 选择应用
```

确认以下配置：

| 字段 | 要求 |
| --- | --- |
| `owner` | `admin` |
| `name` | `app-token-gepinkeji` 或 `app-token-gptk` |
| `organization` | `gepin` |
| `clientId` | 非空，提供给业务系统作为 `X-Casdoor-App-Id` |
| `clientSecret` | 非空，只能给对应业务系统后端保存 |

注意：

- `X-Casdoor-App-Id` 使用 Application 的 `clientId`，不是 Application 的 `name`。
- `clientSecret` 用于业务系统请求签名，也用于业务系统校验 Casdoor Webhook 签名。
- 如果 `clientSecret` 泄露，需要立即在 Casdoor 中重置，并同步业务系统环境变量。

## 二、微信支付 Cert 配置

进入：

```text
Certs -> Add
```

建议配置：

| 字段 | 值 |
| --- | --- |
| `owner` | `gepin` |
| `name` | `cert-wechat-pay-gepinkeji` |
| `certificate` | 微信支付商户证书序列号 |
| `privateKey` | 微信支付商户 API 私钥，必须是 PEM 格式 |

私钥格式必须包含头尾：

```text
-----BEGIN PRIVATE KEY-----
...
-----END PRIVATE KEY-----
```

不要把微信支付私钥、APIv3Key、商户证书内容写入文档、代码仓库或前端配置。

## 三、微信支付 Provider 配置

进入：

```text
Providers -> Add
```

配置微信支付渠道：

| 字段 | 值 |
| --- | --- |
| `owner` | `gepin` |
| `name` | `provider_payment_wechat_gepinkeji` |
| `category` | `Payment` |
| `type` | `WeChat Pay` |
| `clientId` | 微信支付商户号 `MchID` |
| `clientSecret` | 微信支付 `APIv3Key` |
| `clientId2` | 微信支付 `AppID` |
| `cert` | `cert-wechat-pay-gepinkeji` |

配置完成后，业务系统下单时必须传：

```text
providerName=provider_payment_wechat_gepinkeji
```

## 四、统一支付模板 Product 配置

进入：

```text
Products -> Add
```

创建或确认以下 Product：

| 字段 | 值 |
| --- | --- |
| `owner` | `gepin` |
| `name` | `external-pay-template` |
| `state` | `Published` |
| `currency` | `CNY` |
| `providers` | `provider_payment_wechat_gepinkeji` |
| `allowExternalCustomAmount` | 开启 |
| `externalMinAmount` | `0.01` |
| `externalMaxAmount` | `9999` |
| `quantity` | `0` |

说明：

- 这个 Product 是支付模板，不是真实库存商品。
- 外部自定义金额订单会跳过库存扣减。
- 业务系统传入的 `amount` 必须大于 `0`，并落在 `externalMinAmount` 和 `externalMaxAmount` 之间。
- 业务系统传入的 `currency` 必须是 `CNY`。

## 五、Webhook 配置

每个 Application 建议单独配置一个 Webhook，这样不同业务系统可以收到自己的支付成功通知。

支付成功事件固定为：

```text
payment.paid
```

Webhook 请求会包含：

```text
X-Casdoor-Webhook-Event: payment.paid
X-Casdoor-Webhook-Signature: sha256=<hex>
```

签名密钥为对应 Application 的 `clientSecret`。

### 推荐配置

Gepin 科技平台：

| 字段 | 值 |
| --- | --- |
| `owner` | `admin` |
| `name` | `webhook-payment-paid-gepinkeji` |
| `organization` | `admin/app-token-gepinkeji` |
| `url` | `https://token.gepinkeji.com/api/casdoor/payment/webhook` |
| `method` | `POST` |
| `contentType` | `application/json` |
| `events` | `payment.paid` |
| `isEnabled` | 开启 |

GPTK 平台：

| 字段 | 值 |
| --- | --- |
| `owner` | `admin` |
| `name` | `webhook-payment-paid-gptk` |
| `organization` | `admin/app-token-gptk` |
| `url` | `https://token.gptk.cc.cd/api/casdoor/payment/webhook` |
| `method` | `POST` |
| `contentType` | `application/json` |
| `events` | `payment.paid` |
| `isEnabled` | 开启 |

注意：当前支付 Webhook 按 Application 匹配，`organization` 字段需要等于 Application 完整 ID 或 Application 名称。线上这两个应用的完整 ID 是 `admin/app-token-gepinkeji` 和 `admin/app-token-gptk`。如果后台页面的 Organization 下拉框不能选择应用名，或事件下拉框不能选择 `payment.paid`，需要由管理员通过数据库或运维脚本写入。

SQL 模板如下，执行前先备份数据库，并替换 URL：

```sql
insert into webhook (
  owner, name, created_time, organization, url, method, content_type,
  headers, events, token_fields, object_fields, is_user_extended,
  single_org_only, is_enabled, max_retries, retry_interval, use_exponential_backoff
) values (
  'admin',
  'webhook-payment-paid-gptk',
  to_char(now(), 'YYYY-MM-DD"T"HH24:MI:SSOF'),
  'admin/app-token-gptk',
  'https://example.com/api/casdoor/payment/webhook',
  'POST',
  'application/json',
  '[]',
  '["payment.paid"]',
  '[]',
  '[]',
  false,
  false,
  true,
  3,
  60,
  false
)
on conflict (owner, name) do update set
  organization = excluded.organization,
  url = excluded.url,
  method = excluded.method,
  content_type = excluded.content_type,
  events = excluded.events,
  is_enabled = excluded.is_enabled,
  max_retries = excluded.max_retries,
  retry_interval = excluded.retry_interval,
  use_exponential_backoff = excluded.use_exponential_backoff;
```

## 六、接口连通性验证

不带签名请求：

```bash
curl -s -X POST https://login.gepinkeji.com/api/external/payment/create \
  -H 'Content-Type: application/json' \
  --data '{}'
```

预期结果：

- 返回 401 或错误响应
- 错误原因与签名、应用或鉴权有关
- 说明接口已经存在并进入外部支付鉴权逻辑

带签名测试需要使用对应 Application 的 `clientId` 和 `clientSecret`，测试金额建议使用 `0.01`。

## 七、支付成功验证

业务系统真实扫码支付后，Casdoor 应产生以下数据：

| 数据 | 预期 |
| --- | --- |
| Payment | 状态变为 `Paid` |
| Order | 状态变为 `Paid` |
| Transaction | 金额为 `-payment.Price` |
| Webhook Event | 生成 `payment.paid` |
| Webhook Payload | 包含 `externalOrderId`、`amount`、`currency`、`providerName` |

如果业务系统没有收到 Webhook：

1. 在 Casdoor 后台查看 `Webhook Events`。
2. 确认 Webhook 的 `organization` 是否等于 Application ID 或 Application 名称。
3. 确认 `events` 包含 `payment.paid`。
4. 确认业务系统回调地址公网可访问。
5. 对失败事件执行 Replay。

## 八、上线检查清单

- [ ] 每个业务系统都有独立 Application。
- [ ] 每个 Application 的 `clientSecret` 已安全交付给对应后端负责人。
- [ ] 微信支付 Cert 已配置并保存。
- [ ] 微信支付 Provider 已配置为 `provider_payment_wechat_gepinkeji`。
- [ ] Product `external-pay-template` 已发布。
- [ ] Product 已开启 `allowExternalCustomAmount`。
- [ ] Product 金额范围为 `0.01` 到 `9999`。
- [ ] Product 绑定了 `provider_payment_wechat_gepinkeji`。
- [ ] 每个业务系统都配置了 `payment.paid` Webhook。
- [ ] 业务系统完成签名请求和 Webhook 验签。
- [ ] 已使用 `0.01` 完成真实支付闭环验证。
