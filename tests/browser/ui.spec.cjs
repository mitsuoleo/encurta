const { test, expect } = require("@playwright/test");
const { createServer } = require("../../scripts/ui-test-server.cjs");

let server;
test.beforeAll(async () => {
  server = createServer();
  await new Promise((resolve) => server.listen(8099, "127.0.0.1", resolve));
});
test.afterAll(async () => {
  if (server) await new Promise((resolve) => server.close(resolve));
});

function link(code, overrides = {}) {
  return {
    short_code: code,
    short_url: "http://127.0.0.1:8099/" + code,
    original_url: "https://example.com/original",
    is_active: true,
    expires_at: null,
    created_at: "2026-09-26T12:00:00Z",
    ...overrides,
  };
}

async function mockAPI(page, options = {}) {
  let links = options.links || [];
  let listAttempts = 0;
  await page.route("**/auth/*", async (route) => {
    if (route.request().url().endsWith("/logout")) {
      await route.fulfill({ status: 204, body: "" });
      return;
    }
    await route.fulfill({
      status: route.request().url().endsWith("/register") ? 201 : 200,
      json: { access_token: "browser-test-token", expires_in: 3600, email: "test@example.com" },
    });
  });
  await page.route(/\/links(?:\/|\?|$)/, async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const code = url.pathname.split("/")[2];
    if (url.pathname === "/links" && request.method() === "GET") {
      listAttempts++;
      if (options.listStatus && listAttempts === 1) {
        await route.fulfill({ status: options.listStatus, json: {} });
        return;
      }
      if (options.sessionExpired) {
        await route.fulfill({ status: 401, json: {} });
        return;
      }
      const offset = Number(url.searchParams.get("offset") || 0);
      const limit = Number(url.searchParams.get("limit") || 20);
      await route.fulfill({ json: { links: links.slice(offset, offset + limit), total: links.length, limit, offset } });
      return;
    }
    if (url.pathname === "/links" && request.method() === "POST") {
      const body = request.postDataJSON();
      if (body.alias === "auth") {
        await route.fulfill({ status: 400, json: { error: "invalid alias" } });
        return;
      }
      if (body.url.includes("localhost")) {
        await route.fulfill({ status: 400, json: { error: "invalid url" } });
        return;
      }
      if (body.url.includes("/failure")) {
        await route.fulfill({ status: 503, json: {} });
        return;
      }
      const created = link(body.alias || "new1234", { original_url: body.url, expires_at: body.expires_at || null });
      links = [created, ...links];
      await route.fulfill({ status: 201, json: { short_code: created.short_code, short_url: created.short_url } });
      return;
    }
    if (url.pathname.endsWith("/analytics")) {
      await route.fulfill({ json: {
        total_clicks: 3,
        unique_visitors: 2,
        clicks_by_day: [{ day: "2026-09-26", count: 3 }],
        top_referrers: [],
        device_breakdown: [{ name: "desktop", count: 3 }],
        browser_breakdown: [],
      } });
      return;
    }
    if (request.method() === "PATCH") {
      const body = request.postDataJSON();
      links = links.map((item) => item.short_code === code
        ? {
          ...item,
          original_url: body.url ?? item.original_url,
          is_active: body.is_active ?? item.is_active,
          expires_at: body.expires_at === "" ? null : body.expires_at ?? item.expires_at,
        }
        : item);
      await route.fulfill({ json: links.find((item) => item.short_code === code) });
      return;
    }
    if (request.method() === "DELETE") {
      links = links.map((item) => item.short_code === code ? { ...item, is_active: false } : item);
      await route.fulfill({ status: 204, body: "" });
      return;
    }
    await route.fulfill({ status: 404, json: { error: "link not found" } });
  });
}

test("cadastro, criação, detalhes, edição e estado do link", async ({ page }) => {
  await mockAPI(page);
  await page.goto("/");
  await page.getByRole("button", { name: "Criar conta" }).click();
  await page.getByRole("textbox", { name: "E-mail" }).fill("test@example.com");
  await page.getByRole("textbox", { name: "Senha" }).fill("password123");
  await page.locator("#auth-go").click();
  await expect(page.getByRole("heading", { name: "Seus links, em ordem." })).toBeVisible();
  await expect(page.getByText("Ainda não há links aqui.")).toBeVisible();

  await page.getByRole("textbox", { name: "URL de destino" }).fill("https://example.com/article");
  await page.getByRole("textbox", { name: "Alias opcional" }).fill("meu-link");
  await page.getByRole("button", { name: "Criar link" }).click();
  await expect(page.locator("#created")).toHaveText("http://127.0.0.1:8099/meu-link");
  await expect(page.locator("#qr")).toBeVisible();
  await page.getByRole("button", { name: "Detalhes" }).click();
  await expect(page.getByText("3", { exact: true }).first()).toBeVisible();
  await expect(page.getByRole("progressbar", { name: "2026-09-26: 3" })).toBeVisible();

  await page.getByRole("button", { name: "Editar destino e validade" }).click();
  await page.getByRole("textbox", { name: "Novo destino" }).fill("https://example.com/updated");
  await page.getByRole("button", { name: "Salvar alterações" }).click();
  await expect(page.locator("#details-destination")).toContainText("https://example.com/updated");

  await page.getByRole("button", { name: "Desativar link" }).first().click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("dialog").getByRole("button", { name: "Cancelar" }).click();
  await expect(page.locator("#details-status")).toHaveText("Ativo");
  await expect(page.locator("#toggle-active")).toBeFocused();
  await page.getByRole("button", { name: "Desativar link" }).first().click();
  await page.getByRole("dialog").getByRole("button", { name: "Desativar link" }).click();
  await expect(page.locator("#details-status")).toHaveText("Inativo");
  await page.getByRole("button", { name: "Reativar link" }).click();
  await expect(page.locator("#details-status")).toHaveText("Ativo");
  await expect(page.getByRole("button", { name: "Desativar link" })).toBeVisible();
  await expect(page.locator("#toggle-active")).toBeFocused();
  await page.getByRole("button", { name: "Fechar" }).click();
  await expect(page.getByRole("button", { name: "Detalhes" })).toBeFocused();
  await page.getByRole("button", { name: "Sair" }).click();
  await expect(page.getByRole("heading", { name: "Acesse seu espaço" })).toBeVisible();
});

test("erros ficam nos campos e a lista recupera após limite de requisições", async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("us_token", "browser-test-token");
    localStorage.setItem("us_email", "test@example.com");
  });
  await mockAPI(page, { links: [link("existing")], listStatus: 429 });
  await page.goto("/");
  await expect(page.getByText("Muitas tentativas. Aguarde um pouco e tente novamente.")).toBeVisible();
  await page.getByRole("button", { name: "Tentar novamente" }).click();
  await expect(page.getByRole("button", { name: "Detalhes" })).toBeVisible();

  await page.getByRole("button", { name: "Criar link" }).click();
  await expect(page.locator("#url-error")).toContainText("URL http ou https");
  await page.getByRole("textbox", { name: "URL de destino" }).fill("http://localhost/private");
  await page.getByRole("button", { name: "Criar link" }).click();
  await expect(page.locator("#url-error")).toHaveText("URL inválida.");
  await page.getByRole("textbox", { name: "URL de destino" }).fill("https://example.com/article");
  await page.getByRole("textbox", { name: "Alias opcional" }).fill("auth");
  await page.getByRole("button", { name: "Criar link" }).click();
  await expect(page.locator("#alias-error")).toHaveText("Alias inválido ou reservado.");
  await page.getByRole("textbox", { name: "Alias opcional" }).fill("");
  await page.getByRole("textbox", { name: "URL de destino" }).fill("https://example.com/failure");
  await page.getByRole("button", { name: "Criar link" }).click();
  await expect(page.locator("#create-err")).toContainText("indisponível");
  await expect(page.getByRole("textbox", { name: "URL de destino" })).toHaveValue("https://example.com/failure");
  await expect(page.getByRole("button", { name: "Detalhes" })).toBeVisible();
});

test("sessão expirada volta ao login e o tema persiste em tela estreita", async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("us_token", "expired-token");
    localStorage.setItem("us_email", "test@example.com");
  });
  await mockAPI(page, { sessionExpired: true });
  await page.setViewportSize({ width: 320, height: 700 });
  await page.goto("/");
  await expect(page.getByText("Sua sessão expirou. Entre novamente para continuar.")).toBeVisible();
  await page.getByRole("button", { name: "Ativar tema escuro" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const width = await page.evaluate(() => document.documentElement.scrollWidth);
  expect(width).toBe(320);
});

test("link expirado solicita ajuste da validade antes de reativar", async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("us_token", "browser-test-token");
    localStorage.setItem("us_email", "test@example.com");
  });
  await mockAPI(page, { links: [link("expired", { is_active: false, expires_at: "2020-01-01T00:00:00Z" })] });
  await page.goto("/");
  await page.getByRole("button", { name: "Detalhes" }).click();
  await expect(page.locator("#details-status")).toHaveText("Expirado");
  await page.getByRole("button", { name: "Ajustar validade" }).click();
  await expect(page.locator("#edit-form")).toBeVisible();
  await expect(page.getByText("Atualize ou remova a validade antes de reativar.")).toBeVisible();
  await page.getByRole("textbox", { name: "Nova validade opcional" }).fill("");
  await page.getByRole("button", { name: "Salvar alterações" }).click();
  await page.getByRole("button", { name: "Reativar link" }).click();
  await expect(page.locator("#details-status")).toHaveText("Ativo");
});

test("analytics do link anterior desaparece ao selecionar outro, inclusive após erro", async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("us_token", "browser-test-token");
    localStorage.setItem("us_email", "test@example.com");
  });
  await mockAPI(page, { links: [link("one"), link("two")] });
  let releaseFailedResponse;
  const failedResponse = new Promise((resolve) => { releaseFailedResponse = resolve; });
  await page.route(/\/links\/(one|two)\/analytics$/, async (route) => {
    const code = new URL(route.request().url()).pathname.split("/")[2];
    if (code === "two") {
      await failedResponse;
      await route.fulfill({ status: 500, json: {} });
      return;
    }
    await route.fulfill({ json: {
      total_clicks: 7,
      unique_visitors: 5,
      clicks_by_day: [],
      top_referrers: [],
      device_breakdown: [],
      browser_breakdown: [],
    } });
  });
  await page.goto("/");
  await page.locator('button[data-code="one"]').click();
  await expect(page.locator("#stats")).toContainText("7");
  await page.locator('button[data-code="two"]').click();
  await expect(page.locator("#details-title")).toHaveText("two");
  try {
    await expect(page.locator("#stats")).toBeEmpty();
  } finally {
    releaseFailedResponse();
  }
  await expect(page.locator("#stats-err")).toContainText("indisponível");
  await expect(page.locator("#stats")).toBeEmpty();
});
