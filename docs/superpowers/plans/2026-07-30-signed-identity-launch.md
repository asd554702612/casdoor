# Signed Identity Launch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow a valid, signed new-api identity-verification launch URL to complete verification for its target Casdoor user without requiring a Casdoor login, while preserving the existing direct-login flow.

**Architecture:** Casdoor will validate the signed launch request and store a short-lived target-user authorization context in the current browser session. Identity read and submit handlers will prefer that context over the normal Casdoor user session, and clear it after a successful submit. The frontend will bypass the login redirect only when launch parameters are present, serialize launch validation before loading identity data, and submit without using an unrelated logged-in account.

**Tech Stack:** Go 1.25, Beego, Xorm, Casbin, React 18, Jest/React Testing Library.

## Global Constraints

- Preserve all existing validation for `clientId`, target user, organization, redirect URI, timestamp, nonce, and HMAC signature.
- Never trust a bare URL `userId`; only a server-created launch context may authorize target-user reads or writes.
- Keep direct `/identity-verification/submit` behavior unchanged for users without launch parameters.
- Do not modify new-api `oidc_id` mappings or unrelated existing working-tree changes.
- Do not build code on the production server; run tests and builds locally.
- Clear the launch context after successful submission and when it is expired or invalid.

---

### Task 1: Make signed launch validation independent of the current Casdoor login

**Files:**
- Modify: `object/identity_verification.go:520-560`
- Modify: `object/identity_verification_test.go:664-748`

**Interfaces:**
- Replace `ValidateIdentityVerificationLaunch(application, user, sessionUserId, req, now)` with `ValidateIdentityVerificationLaunch(application, user, req, now)`.
- Add exported `object.IdentityVerificationLaunchTTL = 5 * time.Minute` and use it for both signature expiry and launch-session expiry.
- The function continues to return `*IdentityVerificationLaunchInfo` and rejects every existing invalid-input case except a mismatched current login session.

- [ ] **Step 1: Write the failing test**

Add a test case proving a signed request validates when there is no current Casdoor session:

```go
func TestValidateIdentityVerificationLaunchAllowsAnonymousSession(t *testing.T) {
	now := time.Unix(1781712000, 0)
	application := &Application{
		Name: "app-gepin", ClientId: "client-id", ClientSecret: "client-secret", Organization: "gepin",
		RedirectUris: []string{"https://child.example.com/identity/callback"},
	}
	user := &User{Id: "sub-001", Owner: "gepin", Name: "alice"}
	req := &IdentityVerificationLaunchRequest{
		ClientId: application.ClientId, UserId: user.Id,
		RedirectUri: "https://child.example.com/identity/callback",
		State: "state-001", Timestamp: "1781712000", Nonce: "nonce-001",
	}
	req.Signature = SignIdentityVerificationLaunch(application.ClientSecret, req.Timestamp, req.Nonce, req.ClientId, req.UserId, req.RedirectUri, req.State)

	if _, err := ValidateIdentityVerificationLaunch(application, user, req, now); err != nil {
		t.Fatalf("anonymous signed launch should validate: %v", err)
	}
}
```

Update the existing valid and invalid-input tests to use the new function signature, and remove the obsolete “wrong logged in user” case.

- [ ] **Step 2: Run the focused test and verify it fails for the expected reason**

Run:

```bash
go test ./object -run 'TestValidateIdentityVerificationLaunch' -count=1
```

Expected: compilation failure because the production function still requires `sessionUserId`.

- [ ] **Step 3: Implement the minimal validation change**

Add `IdentityVerificationLaunchTTL` in `object/identity_verification.go`, replace the signature-expiry reference in `VerifyIdentityVerificationLaunch` with that exported constant, then remove the `sessionUserId` parameter and the `sessionUserId == "" || sessionUserId != user.GetId()` branch. Keep the application, user, organization, redirect URI, timestamp, nonce, and signature checks in their current order.

- [ ] **Step 4: Run the focused test and verify it passes**

Run:

```bash
go test ./object -run 'TestValidateIdentityVerificationLaunch' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add object/identity_verification.go object/identity_verification_test.go
git -c core.hooksPath=/dev/null commit -m "feat: allow anonymous signed identity launches"
```

### Task 2: Store and consume a server-side target-user launch context

**Files:**
- Modify: `controllers/identity_verification.go`
- Modify: `controllers/identity_verification_test.go`

**Interfaces:**
- Add a private controller session context with `UserId`, `Owner`, `Name`, `RedirectUri`, `State`, and `ExpiresAt`.
- Add private helpers to set, load, validate, and clear that context through `c.SetSession`, `c.GetSession`, and `c.DelSession` using JSON strings, matching the existing `SessionData` pattern.
- Add `decodeIdentityVerificationLaunchSession(raw string, now time.Time)`, returning an expired/invalid error instead of a context.
- Add `chooseIdentityVerificationTarget(launchUser, currentUser *object.User) (*object.User, bool)`, where the boolean indicates a launch target.

- [ ] **Step 1: Write failing controller helper tests**

Cover these behaviors:

```go
func TestDecodeIdentityVerificationLaunchSessionExpires(t *testing.T) {
	raw := `{"userId":"gepin/target","owner":"gepin","name":"target","expiresAt":100}`
	if _, err := decodeIdentityVerificationLaunchSession(raw, time.Unix(101, 0)); err == nil {
		t.Fatal("expired launch context should be rejected")
	}
}

func TestChooseIdentityVerificationTargetPrefersLaunchUser(t *testing.T) {
	launchUser := &object.User{Owner: "gepin", Name: "target"}
	currentUser := &object.User{Owner: "gepin", Name: "other"}
	got, fromLaunch := chooseIdentityVerificationTarget(launchUser, currentUser)
	if got != launchUser || !fromLaunch {
		t.Fatalf("launch target should win: got=%v fromLaunch=%v", got, fromLaunch)
	}
}

func TestChooseIdentityVerificationTargetFallsBackToSessionUser(t *testing.T) {
	currentUser := &object.User{Owner: "gepin", Name: "current"}
	got, fromLaunch := chooseIdentityVerificationTarget(nil, currentUser)
	if got != currentUser || fromLaunch {
		t.Fatalf("session user should be used without launch context: got=%v fromLaunch=%v", got, fromLaunch)
	}
}
```

Use small pure helpers for JSON decode/expiry and target selection so the tests do not require a database or a real browser session.

- [ ] **Step 2: Run the focused controller tests and verify they fail**

Run:

```bash
go test ./controllers -run 'TestIdentityVerification(LaunchContext|Target)' -count=1
```

Expected: compilation failure because the helpers do not exist yet.

- [ ] **Step 3: Implement context helpers and target resolution**

Use the exported five-minute `object.IdentityVerificationLaunchTTL` to derive `ExpiresAt` from the validated request timestamp. When loading a context, reject and delete it if JSON is invalid, `UserId` is empty, or `ExpiresAt <= time.Now().Unix()`.

The target resolver must load the user from the stored `UserId` and verify the stored owner/name still match the loaded user. If no launch context exists, require `GetSessionUsername()` and load that user exactly as the existing direct flow does.

- [ ] **Step 4: Change `GetIdentityVerificationLaunch` to work without login**

Remove the initial `GetSessionUsername()` requirement. Validate the signed request with the new function signature, save the context only after validation succeeds, and return the same launch info. Clear any prior launch context before processing a new launch request so an invalid new URL cannot reuse an older target.

- [ ] **Step 5: Make identity read and submit use the resolved target**

For `GetIdentityVerification`, use the launch target when present and skip `IsAdminOrSelf` only for that validated context. Preserve all existing admin/owner checks for ordinary sessions.

For `SubmitIdentityVerification`, use the launch target when present, ignore empty owner/name fields from the public launch form, reject non-empty fields that point to another user, and preserve the current session/owner checks for ordinary submissions. After the database update and rule evaluation succeed, clear the launch context before returning the target user’s identity response.

- [ ] **Step 6: Add endpoint-level regression coverage**

Add tests that exercise the target-selection branch for both read and submit paths, asserting that a logged-out request with a valid launch context updates the target user and never the unrelated current session user. Also assert that a direct submission without a session still returns “Please login first”, and that a second request after successful submit cannot reuse the cleared context.

- [ ] **Step 7: Run backend tests**

Run:

```bash
go test ./controllers ./object -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add controllers/identity_verification.go controllers/identity_verification_test.go
git -c core.hooksPath=/dev/null commit -m "feat: bind identity verification to signed launch target"
```

### Task 3: Allow the signed launch page to render without a Casdoor login

**Files:**
- Modify: `web/src/EntryPage.js`
- Modify: `web/src/IdentityVerificationPage.js`
- Test: `web/src/IdentityVerificationPage.test.js`

**Interfaces:**
- Add a route-level predicate that bypasses `renderLoginIfNotLoggedIn` only when the page URL contains launch parameters.
- Export `hasIdentityVerificationLaunchParams(search)` from `EntryPage.js` for the route test.
- Make launch validation return a Promise<boolean> so identity loading waits until server-side launch context creation succeeds.
- Export `getIdentityVerificationSubmitTarget(launchInfo, account)` from `IdentityVerificationPage.js` for the submit-target test.
- Submit with blank owner/name for launch mode; normal mode continues to send the current account owner/name.

- [ ] **Step 1: Write failing frontend tests**

Add tests for:

```js
import {hasIdentityVerificationLaunchParams} from "./EntryPage";
import {getIdentityVerificationSubmitTarget} from "./IdentityVerificationPage";

test("recognizes a complete signed launch query without an account", () => {
	const search = "?clientId=client&userId=sub-001&redirectUri=https%3A%2F%2Fchild.example%2Fcallback&state=s&timestamp=1&nonce=n&signature=h";
	expect(hasIdentityVerificationLaunchParams(search)).toBe(true);
	expect(hasIdentityVerificationLaunchParams("?userId=sub-001")).toBe(false);
});

test("uses an empty account target for a signed launch submit", () => {
	expect(getIdentityVerificationSubmitTarget({userId: "sub-001"}, {owner: "gepin", name: "other"})).toEqual({owner: "", name: ""});
	expect(getIdentityVerificationSubmitTarget(null, {owner: "gepin", name: "alice"})).toEqual({owner: "gepin", name: "alice"});
});
```

Use the existing network boundary (`UserBackend`) for the lifecycle regression and assert that launch validation resolves before the first identity request; do not test React state by duplicating implementation details.

- [ ] **Step 2: Run the focused frontend tests and verify the relevant assertions fail**

Run:

```bash
cd web && CI=true npx react-scripts test src/IdentityVerificationPage.test.js --watchAll=false
```

Expected: the current route redirects without an account and the current page calls identity loading before launch validation.

- [ ] **Step 3: Implement the route and lifecycle changes**

In `EntryPage`, route `/identity-verification/submit` directly to `IdentityVerificationPage` when launch parameters are present; otherwise keep `renderLoginIfNotLoggedIn`.

In `IdentityVerificationPage`, validate launch parameters before calling `refreshSelf`, avoid reading `this.props.account` in launch mode, send empty owner/name on launch submit, and remove the post-submit `refreshSelf` call because the backend consumes the launch context. Keep the current account-based prefill only for ordinary logged-in mode.

In `UserBackend`, keep the existing submit helper unchanged because it already serializes empty owner/name values in the request body.

- [ ] **Step 4: Run frontend tests and lint**

Run:

```bash
cd web && CI=true npx react-scripts test src/IdentityVerificationPage.test.js --watchAll=false
cd web && npx eslint src/EntryPage.js src/IdentityVerificationPage.js
```

Expected: PASS with no new warnings or lint errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/EntryPage.js web/src/IdentityVerificationPage.js web/src/IdentityVerificationPage.test.js
git -c core.hooksPath=/dev/null commit -m "feat: render signed identity launches without login"
```

### Task 4: Review, integration verification, and cleanup

**Files:**
- Modify only if required by verification: files from Tasks 1–3.

- [ ] **Step 1: Run the complete backend test suite**

```bash
go test ./... -count=1
```

- [ ] **Step 2: Build the frontend locally**

```bash
cd web && CI=true npm run build
```

Do not build on the production server.

- [ ] **Step 3: Review the diff for unused code**

```bash
git diff HEAD~3 --check
rg -n 'identityVerificationLaunch|launchInfo|fromLaunch' controllers web/src
```

Remove any helper, import, or test fixture that is not used by the final flow.

- [ ] **Step 4: Perform a manual local flow check**

Verify: valid signed link + no Casdoor cookie loads target identity; valid link + different Casdoor cookie still loads target identity; invalid/expired link is rejected; successful submit writes target user and returns callback state; direct page without login still redirects to login.

- [ ] **Step 5: Commit any final verification fix**

If verification requires a final fix, stage only the affected files and commit it with:

```bash
git -c core.hooksPath=/dev/null commit -m "test: verify signed identity launch flow"
```
