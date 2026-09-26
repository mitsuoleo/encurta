const test = require("node:test");
const assert = require("node:assert/strict");
const {
  validURL,
  validAlias,
  expiryISO,
  linkStatus,
  mergePage,
  responseMessage,
} = require("./logic.js");

test("only absolute HTTP destinations are accepted", () => {
  assert.equal(validURL("https://example.com/a"), true);
  assert.equal(validURL("http://example.com"), true);
  for (const input of ["javascript:alert(1)", "file:///etc/passwd", "/relative", "https://"]) {
    assert.equal(validURL(input), false, input);
  }
});

test("aliases respect the public syntax and length", () => {
  assert.equal(validAlias("meu-link"), true);
  for (const input of ["ab", "-alias", "alias-", "a--b", "bad_space", "a".repeat(21)]) {
    assert.equal(validAlias(input), false, input);
  }
});

test("expiry uses UTC and rejects past or invalid local input", () => {
  const now = new Date("2026-09-26T12:00:00Z").getTime();
  assert.equal(expiryISO("", now), "");
  assert.equal(expiryISO("2026-09-27T10:30", now)?.endsWith("Z"), true);
  assert.equal(expiryISO("2026-09-25T10:30", now), null);
  assert.equal(expiryISO("invalid", now), null);
});

test("expired links never appear active, even when also deactivated", () => {
  const now = new Date("2026-09-26T12:00:00Z").getTime();
  assert.equal(linkStatus({ is_active: false, expires_at: "2026-09-25T12:00:00Z" }, now), "Expirado");
  assert.equal(linkStatus({ is_active: false, expires_at: null }, now), "Inativo");
  assert.equal(linkStatus({ is_active: true, expires_at: null }, now), "Ativo");
});

test("appending a page keeps existing links and ignores duplicate codes", () => {
  const original = [{ short_code: "abc", original_url: "https://a.example" }];
  const next = mergePage(original, [
    { short_code: "abc", original_url: "https://stale.example" },
    { short_code: "def", original_url: "https://b.example" },
  ]);
  assert.deepEqual(next.map((link) => link.short_code), ["abc", "def"]);
  assert.equal(next[0].original_url, "https://a.example");
  assert.equal(original.length, 1);
});

test("API errors are presented in Portuguese without leaking raw server text", () => {
  assert.equal(responseMessage(400, "invalid alias"), "Alias inválido ou reservado.");
  assert.equal(responseMessage(404, "link not found"), "Link não encontrado ou sem permissão.");
  assert.equal(responseMessage(400, "private server detail"), "Confira os dados enviados e tente novamente.");
  assert.equal(responseMessage(503, "private server detail"), "Não foi possível concluir a ação. Tente novamente.");
});
