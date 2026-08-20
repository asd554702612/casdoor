# Token 项目实名认证实时同步接入手册

## 适用范围

本文档给 `token.gepinkeji.com` 项目接入 Casdoor 实名认证使用。

核心要求：

- 用户每次登录 token 项目时，必须从 Casdoor 实时获取最新实名认证状态。
- token 项目不能只依赖本地数据库、历史 session、JWT 旧字段判断实名状态。
- 实名认证完成回调后，也必须再次调用 Casdoor 同步接口复核。
- `clientSecret` 只能放在 token 项目后端，不能出现在前端代码、浏览器、日志和 URL 中。

## 一、当前 Casdoor Application

Casdoor 地址：

```text
https://login.gepinkeji.com
```

当前用于 token 项目的 Application：

| 字段 | 值 |
| --- | --- |
| Application | `app-token-gepinkeji` |
| Organization | `gepin` |
| Client ID | `app-token-gepinkeji-dd884f73a91f` |
| 当前 OAuth 回调 | `https://token.gepinkeji.com/oauth/oidc` |
| 建议新增实名回调 | `https://token.gepinkeji.com/identity/callback` |

进入 Casdoor 后台 Application 页面后，需要确认：

```text
Redirect URIs
```

至少包含：

```text
https://token.gepinkeji.com/oauth/oidc
https://token.gepinkeji.com/identity/callback
```

如果没有新增实名回调地址，子应用实名跳转会被 Casdoor 拒绝，错误类似：

```text
invalid redirectUri
```

## 二、token 项目环境变量

建议在 token 项目后端配置：

```bash
CASDOOR_BASE_URL=https://login.gepinkeji.com
CASDOOR_CLIENT_ID=app-token-gepinkeji-dd884f73a91f
CASDOOR_CLIENT_SECRET=<从 Casdoor Application 后台复制>
CASDOOR_OAUTH_CALLBACK_URL=https://token.gepinkeji.com/oauth/oidc
CASDOOR_IDENTITY_CALLBACK_URL=https://token.gepinkeji.com/identity/callback
```

说明：

- `CASDOOR_CLIENT_ID` 使用 Application 的 `clientId`，不是 Application 名称。
- `CASDOOR_CLIENT_SECRET` 必须只保存在服务端环境变量或密钥管理系统。
- 不要把 `CASDOOR_CLIENT_SECRET` 写进前端环境变量，例如 `NEXT_PUBLIC_*`。

## 三、整体登录与实名流程

推荐流程：

1. 用户进入 token 项目，点击登录。
2. token 项目跳转到 Casdoor OAuth 登录。
3. Casdoor 登录成功后，回调 token 项目：

```text
https://token.gepinkeji.com/oauth/oidc?code=...&state=...
```

4. token 项目后端用 `code` 换取 token。
5. token 项目解析 ID Token 或 UserInfo，拿到 Casdoor 用户 `sub`。
6. token 项目后端立即调用 Casdoor 用户同步接口，实时获取实名状态。
7. token 项目把最新实名状态写入本地用户表或 session。
8. 如果 `isVerified=true && ageChecked=true && isOver16=true`，允许进入业务。
9. 如果未实名或年龄不满足要求，token 项目后端生成实名跳转 URL，把用户跳转到 Casdoor 实名页。
10. 用户在 Casdoor 完成实名后，Casdoor 返回 token 项目的实名回调地址。
11. token 项目在实名回调里校验 `state`，然后再次调用 Casdoor 用户同步接口复核最新状态。
12. 复核通过后再放行。

重点：

```text
每次 OAuth 登录成功后都调用 /api/external/user/sync
```

不要因为本地用户上次已经实名就跳过同步。

## 四、用户同步接口

接口地址：

```text
POST https://login.gepinkeji.com/api/external/user/sync
```

请求体：

```json
{
  "userId": "<Casdoor ID Token 的 sub>"
}
```

请求头：

```text
Content-Type: application/json
X-Casdoor-App-Id: app-token-gepinkeji-dd884f73a91f
X-Casdoor-Timestamp: <Unix 秒级时间戳>
X-Casdoor-Nonce: <随机字符串>
X-Casdoor-Signature: <hex HMAC-SHA256 签名>
```

响应示例：

```json
{
  "status": "ok",
  "msg": "",
  "data": {
    "userId": "sub",
    "owner": "gepin",
    "name": "alice",
    "displayName": "Alice",
    "email": "alice@example.com",
    "phone": "13800000000",
    "isVerified": true,
    "ageChecked": true,
    "isOver16": true
  },
  "data2": null
}
```

隐私说明：

- 该接口不会返回身份证号。
- 该接口不会返回真实姓名。
- 年龄只来自 18 位中国身份证生日解析。
- `isOver16=true` 只有在用户已实名、身份证生日可解析、且已满 16 岁时才会返回。

业务判断建议：

| 条件 | token 项目处理 |
| --- | --- |
| `isVerified=true && ageChecked=true && isOver16=true` | 允许进入业务 |
| `isVerified=false` | 跳转实名认证 |
| `isVerified=true && ageChecked=true && isOver16=false` | 按未满 16 岁处理 |
| `isVerified=true && ageChecked=false` | 按年龄未知处理，不建议放行 |

## 五、同步接口签名

签名有效期：

```text
5 分钟
```

签名串：

```text
timestamp + "\n" + nonce + "\n" + rawBody
```

签名算法：

```text
hex(HMAC-SHA256(clientSecret, signingString))
```

重要要求：

- `rawBody` 是最终发送的原始 JSON 字符串。
- 签名时用的 JSON 字符串必须和 HTTP 请求发送的 body 完全一致。
- 不要签名一个对象，发送时又重新 `JSON.stringify` 另一个对象。
- 不要格式化一次、发送另一次，否则签名会失败。

## 六、Node.js 同步示例

```js
import crypto from "crypto";

function getCasdoorConfig() {
  return {
    baseUrl: process.env.CASDOOR_BASE_URL || "https://login.gepinkeji.com",
    clientId: process.env.CASDOOR_CLIENT_ID,
    clientSecret: process.env.CASDOOR_CLIENT_SECRET,
  };
}

function signCasdoorBody(clientSecret, timestamp, nonce, rawBody) {
  return crypto
    .createHmac("sha256", clientSecret)
    .update(`${timestamp}\n${nonce}\n${rawBody}`)
    .digest("hex");
}

export async function syncCasdoorUserIdentity(casdoorUserSub) {
  const {baseUrl, clientId, clientSecret} = getCasdoorConfig();

  if (!clientId || !clientSecret) {
    throw new Error("missing Casdoor client config");
  }

  const rawBody = JSON.stringify({
    userId: casdoorUserSub,
  });

  const timestamp = Math.floor(Date.now() / 1000).toString();
  const nonce = crypto.randomUUID();
  const signature = signCasdoorBody(clientSecret, timestamp, nonce, rawBody);

  const response = await fetch(`${baseUrl}/api/external/user/sync`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Casdoor-App-Id": clientId,
      "X-Casdoor-Timestamp": timestamp,
      "X-Casdoor-Nonce": nonce,
      "X-Casdoor-Signature": signature,
    },
    body: rawBody,
  });

  const result = await response.json();
  if (!response.ok || result.status !== "ok") {
    throw new Error(result.msg || "sync Casdoor user identity failed");
  }

  return result.data;
}

export function canEnterTokenBusiness(identity) {
  return Boolean(identity?.isVerified && identity?.ageChecked && identity?.isOver16);
}
```

## 七、登录回调中的实时同步

在 token 项目的 OAuth 回调接口中，完成 `code` 换 token 后，立即同步实名信息。

伪代码：

```js
export async function handleCasdoorOAuthCallback(req, res) {
  const {code, state} = req.query;

  // 1. 校验 OAuth state，防止 CSRF
  await verifyOAuthState(req, state);

  // 2. 用 code 换 token
  const tokenResult = await exchangeCodeForToken(code);

  // 3. 解析 ID Token 或调用 UserInfo，拿到 Casdoor sub
  const casdoorUser = await getCasdoorUserFromToken(tokenResult);
  const casdoorUserSub = casdoorUser.sub;

  // 4. 每次登录都实时同步，不读取旧缓存直接放行
  const identity = await syncCasdoorUserIdentity(casdoorUserSub);

  // 5. 保存本地用户和最新实名快照
  await upsertLocalUser({
    casdoorUserId: identity.userId,
    owner: identity.owner,
    name: identity.name,
    displayName: identity.displayName,
    email: identity.email,
    phone: identity.phone,
    isVerified: identity.isVerified,
    ageChecked: identity.ageChecked,
    isOver16: identity.isOver16,
    identitySyncedAt: new Date(),
  });

  // 6. 建立 token 项目自己的登录态
  await createLocalSession(req, res, {
    casdoorUserId: identity.userId,
  });

  // 7. 未满足实名和年龄要求时，跳转 Casdoor 实名页
  if (!canEnterTokenBusiness(identity)) {
    const redirectTo = await buildCasdoorIdentityVerificationUrl(identity.userId, {
      returnTo: "/",
      reason: "login",
    });
    res.redirect(redirectTo);
    return;
  }

  res.redirect("/");
}
```

注意：

- `identitySyncedAt` 可以保存，用于排查问题和展示同步时间。
- 但下次登录仍然必须重新调用同步接口。
- 不建议把 `identitySyncedAt` 当作长期缓存有效期。

## 八、生成实名认证跳转 URL

实名页面地址：

```text
https://login.gepinkeji.com/identity-verification/submit
```

URL 参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `clientId` | 是 | `app-token-gepinkeji-dd884f73a91f` |
| `userId` | 是 | Casdoor 用户 `sub` |
| `redirectUri` | 是 | token 项目实名回调地址 |
| `state` | 建议 | token 项目生成并保存的随机状态 |
| `timestamp` | 是 | Unix 秒级时间戳 |
| `nonce` | 是 | 随机字符串 |
| `signature` | 是 | URL 参数签名 |

跳转签名串：

```text
timestamp + "\n" + nonce + "\n" + clientId + "\n" + userId + "\n" + redirectUri + "\n" + state
```

签名算法：

```text
hex(HMAC-SHA256(clientSecret, signingString))
```

Node.js 示例：

```js
import crypto from "crypto";

export async function buildCasdoorIdentityVerificationUrl(casdoorUserSub, payload = {}) {
  const baseUrl = process.env.CASDOOR_BASE_URL || "https://login.gepinkeji.com";
  const clientId = process.env.CASDOOR_CLIENT_ID;
  const clientSecret = process.env.CASDOOR_CLIENT_SECRET;
  const redirectUri = process.env.CASDOOR_IDENTITY_CALLBACK_URL;

  const timestamp = Math.floor(Date.now() / 1000).toString();
  const nonce = crypto.randomUUID();
  const state = crypto.randomUUID();

  // token 项目需要把 state 和业务上下文保存到服务端，例如 Redis 或数据库。
  await saveIdentityState(state, {
    casdoorUserSub,
    returnTo: payload.returnTo || "/",
    reason: payload.reason || "identity-verification",
    createdAt: new Date().toISOString(),
  });

  const signingString = [
    timestamp,
    nonce,
    clientId,
    casdoorUserSub,
    redirectUri,
    state,
  ].join("\n");

  const signature = crypto
    .createHmac("sha256", clientSecret)
    .update(signingString)
    .digest("hex");

  const url = new URL(`${baseUrl}/identity-verification/submit`);
  url.searchParams.set("clientId", clientId);
  url.searchParams.set("userId", casdoorUserSub);
  url.searchParams.set("redirectUri", redirectUri);
  url.searchParams.set("state", state);
  url.searchParams.set("timestamp", timestamp);
  url.searchParams.set("nonce", nonce);
  url.searchParams.set("signature", signature);

  return url.toString();
}
```

## 九、实名认证回调处理

Casdoor 实名完成后会回调：

```text
https://token.gepinkeji.com/identity/callback?state=<state>&status=approved&isVerified=true&ageChecked=true&isOver16=true
```

回调参数说明：

| 参数 | 说明 |
| --- | --- |
| `state` | token 项目生成的状态值 |
| `status` | Casdoor 实名状态，例如 `approved`、`pending`、`rejected` |
| `isVerified` | 是否已实名 |
| `ageChecked` | 是否完成年龄判断 |
| `isOver16` | 是否满 16 岁 |

安全要求：

- 回调 URL 参数只用于页面提示，不作为最终放行依据。
- token 项目必须校验 `state`。
- token 项目必须再次调用 `/api/external/user/sync` 获取最新状态。
- 不要要求 Casdoor 在回调里返回真实姓名和身份证号。

伪代码：

```js
export async function handleIdentityCallback(req, res) {
  const {state} = req.query;

  const savedState = await consumeIdentityState(state);
  if (!savedState) {
    res.status(400).send("invalid identity state");
    return;
  }

  // 回调参数不可信，必须重新同步 Casdoor 最新实名状态。
  const identity = await syncCasdoorUserIdentity(savedState.casdoorUserSub);

  await updateLocalIdentitySnapshot(identity.userId, {
    isVerified: identity.isVerified,
    ageChecked: identity.ageChecked,
    isOver16: identity.isOver16,
    identitySyncedAt: new Date(),
  });

  if (!canEnterTokenBusiness(identity)) {
    res.redirect("/identity-required");
    return;
  }

  res.redirect(savedState.returnTo || "/");
}
```

## 十、建议的 token 项目路由

token 项目建议新增或确认以下后端路由：

| 路由 | 用途 |
| --- | --- |
| `GET /oauth/oidc` | Casdoor OAuth 登录回调，登录成功后实时同步实名状态 |
| `GET /identity/callback` | Casdoor 实名完成回调，复核实名状态 |
| `GET /identity-required` | token 项目自己的未实名或未满 16 岁提示页 |

如 token 项目有中间件或服务端鉴权函数，建议封装：

```js
async function requireVerifiedOver16(session) {
  const identity = await syncCasdoorUserIdentity(session.casdoorUserId);
  if (!canEnterTokenBusiness(identity)) {
    throw new Error("identity verification required");
  }
  return identity;
}
```

对于高风险业务入口，可以在进入页面或提交订单前再调用一次，确保不是只依赖登录时状态。

## 十一、本地用户表建议字段

token 项目本地用户表建议保存：

| 字段 | 说明 |
| --- | --- |
| `casdoorUserId` | Casdoor `sub`，调用同步接口使用 |
| `casdoorOwner` | 例如 `gepin` |
| `casdoorName` | Casdoor 用户名 |
| `displayName` | 显示名 |
| `email` | 邮箱 |
| `phone` | 手机号 |
| `isVerified` | 最新是否实名 |
| `ageChecked` | 最新是否完成年龄判断 |
| `isOver16` | 最新是否满 16 岁 |
| `identitySyncedAt` | 最近一次从 Casdoor 同步时间 |

不建议保存：

- 身份证号
- 真实姓名
- Casdoor `clientSecret`

## 十二、错误处理建议

同步接口可能返回：

| 错误 | 常见原因 | 处理建议 |
| --- | --- | --- |
| `invalid application` | `CASDOOR_CLIENT_ID` 错误 | 检查 Application 的 `clientId` |
| `missing signature headers` | 缺少签名头 | 检查请求头 |
| `invalid signature` | 签名串、body 或 secret 不一致 | 检查 rawBody 是否完全一致 |
| `expired timestamp` | 服务器时间相差超过 5 分钟 | 检查服务器时间同步 |
| `user does not belong to application organization` | 用户不属于 `gepin` 或 userId 错误 | 检查 ID Token 的 `sub` |
| `invalid redirectUri` | 实名回调未加入 Application Redirect URIs | 在 Casdoor 后台补充回调地址 |

失败策略：

- 登录时同步失败，不建议默认放行。
- 可以展示“实名状态同步失败，请稍后重试”。
- 后端日志记录错误原因，但不要记录 `clientSecret` 和完整签名。

## 十三、上线检查清单

上线前确认：

- `CASDOOR_CLIENT_ID=app-token-gepinkeji-dd884f73a91f`
- `CASDOOR_CLIENT_SECRET` 已配置在 token 项目后端。
- Casdoor Application Redirect URIs 已添加 `https://token.gepinkeji.com/identity/callback`。
- OAuth 登录回调每次都会调用 `syncCasdoorUserIdentity()`。
- 实名回调会校验 `state`。
- 实名回调会再次调用 `syncCasdoorUserIdentity()` 复核。
- 前端和 URL 不展示真实姓名、身份证号、`clientSecret`。
- 同步接口失败时不会默认放行业务。
- 服务器时间已同步，避免 5 分钟签名窗口失效。
