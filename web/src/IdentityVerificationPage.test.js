/* global expect, test */

import {getIdentityVerificationSubmitTarget, hasIdentityVerificationLaunchParams} from "./identityVerificationLaunch";

test("recognizes a complete signed launch query without an account", () => {
  const search = "?clientId=client&userId=sub-001&redirectUri=https%3A%2F%2Fchild.example%2Fcallback&state=s&timestamp=1&nonce=n&signature=h";
  expect(hasIdentityVerificationLaunchParams(search)).toBe(true);
  expect(hasIdentityVerificationLaunchParams(search.replace("&state=s", ""))).toBe(true);
  expect(hasIdentityVerificationLaunchParams("?userId=sub-001")).toBe(false);
});

test("uses an empty account target for a signed launch submit", () => {
  expect(getIdentityVerificationSubmitTarget({userId: "sub-001"}, {owner: "gepin", name: "other"})).toEqual({owner: "", name: ""});
  expect(getIdentityVerificationSubmitTarget(null, {owner: "gepin", name: "alice"})).toEqual({owner: "gepin", name: "alice"});
});
