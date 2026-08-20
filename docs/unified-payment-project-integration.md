# 业务系统统一支付接入手册

## 适用范围

本文档给接入 Casdoor 统一支付的业务系统开发人员使用。

当前支持的业务系统：

| 业务系统 | Casdoor Application | Organization | 已配置 Webhook URL |
| --- | --- | --- | --- |
| Gepin 科技平台 | `admin/app-token-gepinkeji` | `gepin` | `https://token.gepinkeji.com/api/casdoor/payment/webhook` |
| GPTK 平台 | `admin/app-token-gptk` | `gepin` | `https://token.gptk.cc.cd/api/casdoor/payment/webhook` |

统一支付由业务系统后端调用 Casdoor，前端只负责展示二维码或支付状态。不要在浏览器、小程序、App 客户端中保存 `clientSecret`。

## 一、接入流程

业务系统完整流程：

1. 业务系统创建自己的待支付订单。
2. 业务系统后端生成唯一 `externalOrderId`。
3. 业务系统后端把订单金额、标题、用户、订单号提交给 Casdoor。
4. Casdoor 校验签名、金额、币种、支付渠道。
5. Casdoor 创建微信支付订单，返回 `payUrl`。
6. 业务系统前端把 `payUrl` 渲染成二维码。
7. 用户扫码支付。
8. Casdoor 收到微信支付回调。
9. Casdoor 给业务系统发送 `payment.paid` Webhook。
10. 业务系统校验 Webhook 签名，根据 `externalOrderId` 更新自己的订单状态。

## 二、环境变量配置

每个业务系统需要在后端配置：

```bash
CASDOOR_BASE_URL=https://login.gepinkeji.com
CASDOOR_CLIENT_ID=<当前 Application 的 clientId>
CASDOOR_CLIENT_SECRET=<当前 Application 的 clientSecret>
CASDOOR_APPLICATION_NAME=<app-token-gepinkeji 或 app-token-gptk>
CASDOOR_PAYMENT_PRODUCT=external-pay-template
CASDOOR_PAYMENT_PROVIDER=provider_payment_wechat_gepinkeji
CASDOOR_PAYMENT_CURRENCY=CNY
```

注意：

- `CASDOOR_CLIENT_ID` 是 Casdoor Application 的 `clientId`。
- `CASDOOR_APPLICATION_NAME` 仅用于业务系统日志和校验，不参与请求签名。
- `CASDOOR_CLIENT_SECRET` 只能放在后端环境变量或密钥管理系统中。
- 不要把 `clientSecret` 打进日志。
- 不要把 `clientSecret` 返回给前端。

## 三、创建支付接口

接口地址：

```text
POST https://login.gepinkeji.com/api/external/payment/create
```

请求头：

```text
Content-Type: application/json
X-Casdoor-App-Id: <CASDOOR_CLIENT_ID>
X-Casdoor-Timestamp: <当前 Unix 秒级时间戳>
X-Casdoor-Nonce: <随机字符串>
X-Casdoor-Signature: <HMAC-SHA256 签名>
```

请求体示例：

```json
{
  "externalOrderId": "gptk-order-202606250001",
  "userId": "gepin/zhangsan",
  "productName": "external-pay-template",
  "providerName": "provider_payment_wechat_gepinkeji",
  "amount": 88.66,
  "currency": "CNY",
  "displayName": "GPTK 充值",
  "detail": "订单说明"
}
```

字段说明：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `externalOrderId` | 是 | 业务系统自己的订单号，同一个 Application 内唯一 |
| `userId` | 是 | Casdoor 用户 ID，例如 `gepin/zhangsan` |
| `productName` | 是 | 固定传 `external-pay-template` |
| `providerName` | 是 | 固定传 `provider_payment_wechat_gepinkeji` |
| `amount` | 是 | 本次订单金额，单位元，例如 `88.66` |
| `currency` | 建议传 | 固定传 `CNY` |
| `displayName` | 建议传 | 支付标题 |
| `detail` | 可选 | 订单描述 |
| `couponCode` | 可选 | 当前可不传 |

如果没有 `userId`，可以改传：

```json
{
  "owner": "gepin",
  "userName": "zhangsan"
}
```

建议优先使用 `userId`，减少拼接错误。

## 四、参数规则

`externalOrderId` 规则：

```text
1 到 100 个字符，只允许字母、数字、点、下划线、冒号、横线
```

推荐格式：

```text
gptk-order-202606250001
gepin-vip-202606250001
```

金额规则：

```text
0.01 <= amount <= 9999
```

币种规则：

```text
currency = CNY
```

支付渠道：

```text
providerName = provider_payment_wechat_gepinkeji
```

## 五、请求签名

签名算法：

```text
HMAC-SHA256(clientSecret, timestamp + "\n" + nonce + "\n" + rawBody)
```

输出格式：

```text
小写 hex 字符串
```

有效期：

```text
5 分钟
```

重要要求：

- 签名使用原始 JSON 字符串 `rawBody`。
- 签名前后的 JSON 必须完全一致。
- 不要先签名一个 JSON，发送时又重新序列化另一个 JSON。
- 修改 `amount`、`externalOrderId`、`userId` 等任何字段都会导致签名失败。

## 六、Node.js 接入示例

```js
import crypto from "crypto";

export async function createCasdoorPayment(order) {
  const baseUrl = process.env.CASDOOR_BASE_URL;
  const clientId = process.env.CASDOOR_CLIENT_ID;
  const clientSecret = process.env.CASDOOR_CLIENT_SECRET;

  const body = JSON.stringify({
    externalOrderId: order.orderNo,
    userId: order.casdoorUserId,
    productName: process.env.CASDOOR_PAYMENT_PRODUCT,
    providerName: process.env.CASDOOR_PAYMENT_PROVIDER,
    amount: order.payAmount,
    currency: process.env.CASDOOR_PAYMENT_CURRENCY || "CNY",
    displayName: order.title,
    detail: order.description || ""
  });

  const timestamp = Math.floor(Date.now() / 1000).toString();
  const nonce = crypto.randomUUID();

  const signature = crypto
    .createHmac("sha256", clientSecret)
    .update(`${timestamp}\n${nonce}\n${body}`)
    .digest("hex");

  const response = await fetch(`${baseUrl}/api/external/payment/create`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Casdoor-App-Id": clientId,
      "X-Casdoor-Timestamp": timestamp,
      "X-Casdoor-Nonce": nonce,
      "X-Casdoor-Signature": signature
    },
    body
  });

  const result = await response.json();
  if (!response.ok || result.status !== "ok") {
    throw new Error(result.msg || "create Casdoor payment failed");
  }

  return result.data;
}
```

返回数据示例：

```json
{
  "orderId": "gepin/order_xxx",
  "paymentId": "gepin/payment_xxx",
  "externalOrderId": "gptk-order-202606250001",
  "payUrl": "weixin://wxpay/bizpayurl?...",
  "state": "Created",
  "amount": 88.66,
  "currency": "CNY",
  "providerName": "provider_payment_wechat_gepinkeji"
}
```

业务系统需要保存：

- `externalOrderId`
- `orderId`
- `paymentId`
- `payUrl`
- `amount`
- `currency`
- `providerName`

前端拿 `payUrl` 生成二维码即可。

## 七、幂等规则

Casdoor 按以下组合做幂等：

```text
Application + externalOrderId
```

同一个 Application 重复请求同一个 `externalOrderId`：

- 返回同一个 `orderId`
- 返回同一个 `paymentId`
- 不会重复创建 Casdoor 订单

业务系统也必须自己做幂等：

- 同一个业务订单只能创建一笔待支付记录。
- 收到重复 Webhook 时不能重复发货、重复充值、重复开通权益。

## 八、Webhook 接收

业务系统需要提供一个公网可访问的 HTTP 接口，例如：

```text
POST /api/casdoor/payment/webhook
```

Casdoor 生产库已经为两个应用配置好以下回调地址：

| Casdoor Application | Webhook URL |
| --- | --- |
| `admin/app-token-gepinkeji` | `https://token.gepinkeji.com/api/casdoor/payment/webhook` |
| `admin/app-token-gptk` | `https://token.gptk.cc.cd/api/casdoor/payment/webhook` |

业务系统必须确保对应接口存在，并且公网可以被 Casdoor 访问。

Casdoor 支付成功后会推送：

```json
{
  "event": "payment.paid",
  "application": "app-token-gptk",
  "externalOrderId": "gptk-order-202606250001",
  "orderId": "gepin/order_xxx",
  "paymentId": "gepin/payment_xxx",
  "userId": "gepin/zhangsan",
  "products": [
    {
      "owner": "gepin",
      "name": "external-pay-template",
      "displayName": "GPTK 充值",
      "price": 88.66,
      "currency": "CNY",
      "quantity": 1
    }
  ],
  "amount": 88.66,
  "currency": "CNY",
  "providerName": "provider_payment_wechat_gepinkeji",
  "paidTime": "2026-06-25T10:00:00+08:00"
}
```

说明：`application` 字段返回 Application 名称，例如 `app-token-gptk`，不是完整 ID `admin/app-token-gptk`。

请求头：

```text
X-Casdoor-Webhook-Event: payment.paid
X-Casdoor-Webhook-Signature: sha256=<hex>
```

Webhook 验签算法：

```text
HMAC-SHA256(clientSecret, rawWebhookBody)
```

结果格式：

```text
sha256=<hex>
```

Node.js 验签示例：

```js
import crypto from "crypto";

export function verifyCasdoorWebhook(rawBody, signatureHeader) {
  const clientSecret = process.env.CASDOOR_CLIENT_SECRET;

  const expected = "sha256=" + crypto
    .createHmac("sha256", clientSecret)
    .update(rawBody)
    .digest("hex");

  const actual = signatureHeader || "";
  if (expected.length !== actual.length) {
    return false;
  }

  return crypto.timingSafeEqual(
    Buffer.from(expected),
    Buffer.from(actual)
  );
}
```

注意：Webhook 验签必须使用原始请求体 `rawBody`，不能使用已经解析后的 JSON 对象重新序列化。

## 九、Webhook 处理逻辑

业务系统收到 `payment.paid` 后：

1. 校验 `X-Casdoor-Webhook-Signature`。
2. 校验 `event` 必须等于 `payment.paid`。
3. 校验 `application` 等于当前业务系统的 Application 名称。
4. 用 `externalOrderId` 查询自己的业务订单。
5. 校验业务订单存在。
6. 校验订单未支付，已支付则直接返回成功。
7. 校验 `amount` 与业务订单应付金额一致。
8. 校验 `currency` 等于 `CNY`。
9. 校验 `providerName` 等于 `provider_payment_wechat_gepinkeji`。
10. 更新业务订单为已支付。
11. 记录 `orderId`、`paymentId`、`paidTime`。
12. 返回 HTTP 200。

如果返回非 2xx，Casdoor 会按配置重试。

## 十、错误处理

常见错误：

| 错误 | 处理 |
| --- | --- |
| `invalid application` | 检查 `CASDOOR_CLIENT_ID` 是否填错 |
| `missing signature headers` | 检查 4 个签名请求头是否完整 |
| `invalid signature` | 检查 `clientSecret`、签名串、JSON 是否一致 |
| `expired timestamp` | 检查服务器时间，签名有效期为 5 分钟 |
| `amount must be greater than zero` | 金额必须大于 0 |
| `amount is below minimum` | 金额不能小于 `0.01` |
| `amount is above maximum` | 金额不能大于 `9999` |
| `currency mismatch` | 固定传 `CNY` |
| `payment provider is not valid` | 固定传 `provider_payment_wechat_gepinkeji` |
| `user does not belong to application organization` | 用户必须属于 `gepin` 组织 |

## 十一、测试用例

接入完成后必须验证：

- [ ] 使用 `0.01` 创建支付，返回 `payUrl`。
- [ ] 前端能把 `payUrl` 渲染成二维码。
- [ ] 当前业务系统已提供 `/api/casdoor/payment/webhook` 公网接口。
- [ ] 重复请求同一个 `externalOrderId` 返回同一个 `orderId` 和 `paymentId`。
- [ ] 生成签名后篡改 `amount`，Casdoor 返回签名失败。
- [ ] `amount=0` 被拒绝。
- [ ] `currency=USD` 被拒绝。
- [ ] 错误 `providerName` 被拒绝。
- [ ] 用户真实支付后能收到 `payment.paid`。
- [ ] Webhook 签名校验通过。
- [ ] 重复 Webhook 不会导致重复发货、重复充值或重复开通权益。

## 十二、安全要求

- 所有支付创建请求必须从业务系统后端发起。
- 前端不能接触 `clientSecret`。
- 日志中不要打印完整请求头、签名、`clientSecret`。
- 业务系统订单金额必须来自服务端订单表，不能相信前端传入金额。
- 支付成功以 Casdoor `payment.paid` Webhook 为准。
- Webhook 必须验签后再处理业务。
- 处理支付成功时必须做幂等。
