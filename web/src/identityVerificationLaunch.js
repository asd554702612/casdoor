const identityVerificationLaunchParams = [
  "clientId",
  "userId",
  "redirectUri",
  "timestamp",
  "nonce",
  "signature",
];

export function hasIdentityVerificationLaunchParams(search) {
  const query = new URLSearchParams(search || "");
  return identityVerificationLaunchParams.every(param => query.get(param) !== null && query.get(param) !== "");
}

export function getIdentityVerificationSubmitTarget(launchInfo, account) {
  if (launchInfo?.userId) {
    return {owner: "", name: ""};
  }
  return {
    owner: account?.owner || "",
    name: account?.name || "",
  };
}
