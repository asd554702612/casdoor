const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

function runMvScript(mockFs) {
  const script = fs.readFileSync(path.join(__dirname, "mv.js"), "utf8");
  const sandbox = {
    __dirname: "/web",
    console: {
      error() {},
      log() {},
    },
    process: {
      exit(code) {
        const err = new Error(`process.exit(${code})`);
        err.code = "PROCESS_EXIT";
        err.exitCode = code;
        throw err;
      },
    },
    require(name) {
      if (name === "fs") {
        return mockFs;
      }
      if (name === "path") {
        return path;
      }
      return require(name);
    },
  };

  vm.runInNewContext(script, sandbox, {filename: "mv.js"});
}

test("falls back to copy and remove when rename crosses devices", () => {
  const calls = [];
  const mockFs = {
    existsSync(filePath) {
      return filePath === "/web/build-temp" || filePath === "/web/build";
    },
    rmSync(filePath, options) {
      calls.push(["rm", filePath, JSON.parse(JSON.stringify(options))]);
    },
    renameSync() {
      const err = new Error("cross-device link not permitted");
      err.code = "EXDEV";
      throw err;
    },
    cpSync(source, target, options) {
      calls.push(["cp", source, target, JSON.parse(JSON.stringify(options))]);
    },
  };

  assert.doesNotThrow(() => runMvScript(mockFs));
  assert.deepEqual(calls, [
    ["rm", "/web/build", {recursive: true, force: true}],
    ["cp", "/web/build-temp", "/web/build", {recursive: true}],
    ["rm", "/web/build-temp", {recursive: true, force: true}],
  ]);
});
