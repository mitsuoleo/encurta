const http = require("node:http");
const fs = require("node:fs");
const path = require("node:path");

const root = path.resolve(__dirname, "../internal/web");
const assets = new Map([
  ["/", ["index.html", "text/html; charset=utf-8"]],
  ["/ui/app.css", ["app.css", "text/css; charset=utf-8"]],
  ["/ui/app.js", ["app.js", "text/javascript; charset=utf-8"]],
  ["/ui/logic.js", ["logic.js", "text/javascript; charset=utf-8"]],
  ["/ui/qr.js", ["qr.js", "text/javascript; charset=utf-8"]],
]);

function createServer() {
  return http.createServer((request, response) => {
    const entry = assets.get(new URL(request.url, "http://localhost").pathname);
    if (!entry) {
      response.writeHead(404).end();
      return;
    }
    response.setHeader("Content-Type", entry[1]);
    response.setHeader("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'");
    response.setHeader("X-Content-Type-Options", "nosniff");
    fs.createReadStream(path.join(root, entry[0])).pipe(response);
  });
}

module.exports = { createServer };
