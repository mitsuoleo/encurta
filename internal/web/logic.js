"use strict";

const UILogic = (() => {
  function validURL(value) {
    try {
      const url = new URL(value);
      return ["http:", "https:"].includes(url.protocol) && !!url.hostname;
    } catch {
      return false;
    }
  }

  function validAlias(value) {
    return value.length >= 3
      && value.length <= 20
      && /^[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*$/.test(value);
  }

  function expiryISO(value, now = Date.now()) {
    if (!value) return "";
    const date = new Date(value);
    if (Number.isNaN(date.getTime()) || date.getTime() <= now) return null;
    return date.toISOString();
  }

  function localDateTime(value) {
    if (!value) return "";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "";
    const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
    return local.toISOString().slice(0, 16);
  }

  function linkStatus(link, now = Date.now()) {
    if (link.expires_at && new Date(link.expires_at).getTime() <= now) return "Expirado";
    if (!link.is_active) return "Inativo";
    return "Ativo";
  }

  function formatDate(value) {
    if (!value) return "Sem validade";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "Data indisponível";
    return new Intl.DateTimeFormat("pt-BR", { dateStyle: "medium", timeStyle: "short" }).format(date);
  }

  function mergePage(current, incoming) {
    const seen = new Set(current.map((link) => link.short_code));
    return [...current, ...incoming.filter((link) => !seen.has(link.short_code))];
  }

  function responseMessage(status, code) {
    const known = {
      "invalid url": "URL inválida.",
      "invalid alias": "Alias inválido ou reservado.",
      "invalid expiration": "Validade inválida. Escolha uma data futura.",
      "invalid payload": "Confira os dados enviados.",
      "short code collision": "Este alias já está em uso.",
      "password must be at least 8 characters": "Use pelo menos 8 caracteres na senha.",
      "password is too long": "A senha é longa demais.",
      "link not found": "Link não encontrado ou sem permissão.",
      "cache unavailable": "Não foi possível atualizar o link no momento. Tente novamente.",
    };
    if (known[code]) return known[code];
    if (status === 404) return "Recurso não encontrado ou sem permissão.";
    if (status === 409) return "Este valor já está em uso.";
    if (status === 400) return "Confira os dados enviados e tente novamente.";
    return "Não foi possível concluir a ação. Tente novamente.";
  }

  return Object.freeze({ validURL, validAlias, expiryISO, localDateTime, linkStatus, formatDate, mergePage, responseMessage });
})();

if (typeof module !== "undefined") module.exports = UILogic;
