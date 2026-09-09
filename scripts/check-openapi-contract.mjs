import assert from "node:assert/strict";
import fs from "node:fs";

const spec = JSON.parse(fs.readFileSync("api/openapi.v1.json", "utf8"));
const baseline = JSON.parse(fs.readFileSync("api/openapi.v1.baseline.json", "utf8"));
const server = fs.readFileSync("internal/httpapi/server.go", "utf8");
const methods = new Set(["get", "head", "post", "put", "patch", "delete", "options"]);

assert.match(spec.openapi || "", /^3\.1\./, "public contract must use OpenAPI 3.1");
assert.equal(spec.info?.version, "1.0.0", "public API contract version is independent of the app release");
assert.equal(spec["x-trendinary-contract"]?.apiVersion, "v1");

const operationIds = new Set();
const specRoutes = new Set();
for (const [path, item] of Object.entries(spec.paths || {})) {
  assert.ok(path.startsWith("/api/v1/"), `public spec contains non-v1 path: ${path}`);
  assert.ok(!path.startsWith("/api/v1/admin/"), `private admin path leaked into public spec: ${path}`);
  assert.ok(!path.startsWith("/api/v1/integrations/"), `private integration path leaked into public spec: ${path}`);

  const templateParams = [...path.matchAll(/\{([^}]+)\}/g)].map((match) => match[1]).sort();
  for (const [method, operation] of Object.entries(item)) {
    if (!methods.has(method)) continue;
    assert.ok(operation.operationId, `${method.toUpperCase()} ${path} has no operationId`);
    assert.ok(!operationIds.has(operation.operationId), `duplicate operationId ${operation.operationId}`);
    operationIds.add(operation.operationId);

    const responses = Object.keys(operation.responses || {});
    assert.ok(responses.some((code) => /^2\d\d$/.test(code)), `${method.toUpperCase()} ${path} has no success response`);
    const declaredPathParams = (operation.parameters || [])
      .filter((parameter) => parameter.in === "path" && parameter.required === true)
      .map((parameter) => parameter.name)
      .sort();
    assert.deepEqual(declaredPathParams, templateParams, `${method.toUpperCase()} ${path} path parameters drifted`);
    specRoutes.add(`${method.toUpperCase()} ${path}`);
  }
}

const registered = new Set();
const routePattern = /mux\.HandleFunc\("([A-Z]+) ([^"]+)"/g;
for (const match of server.matchAll(routePattern)) {
  registered.add(`${match[1]} ${match[2]}`);
}
assert.deepEqual([...specRoutes].sort(), [...registered].sort(), "OpenAPI routes must exactly match the public Go mux registrations");

for (const [path, methodsAtPath] of Object.entries(baseline.operations || {})) {
  assert.ok(spec.paths?.[path], `breaking API change: removed path ${path}`);
  for (const [method, previous] of Object.entries(methodsAtPath)) {
    const current = spec.paths[path]?.[method];
    assert.ok(current, `breaking API change: removed ${method.toUpperCase()} ${path}`);
    assert.equal(current.operationId, previous.operationId, `breaking API change: operationId changed for ${method.toUpperCase()} ${path}`);

    const currentResponses = new Set(Object.keys(current.responses || {}));
    for (const code of previous.responses) {
      assert.ok(currentResponses.has(code), `breaking API change: removed ${code} response from ${method.toUpperCase()} ${path}`);
    }

    if (previous.requestBodyRequired) {
      assert.equal(current.requestBody?.required, true, `breaking API change: required request-body contract drifted for ${method.toUpperCase()} ${path}`);
    }

    const currentPathParams = new Set(
      (current.parameters || [])
        .filter((parameter) => parameter.in === "path" && parameter.required === true)
        .map((parameter) => parameter.name),
    );
    for (const name of previous.pathParameters || []) {
      assert.ok(currentPathParams.has(name), `breaking API change: removed required path parameter ${name} from ${method.toUpperCase()} ${path}`);
    }
  }
}

console.log(`OpenAPI contract: ${specRoutes.size} public operations match the Go mux and preserve the v1 compatibility baseline`);
