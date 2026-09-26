function drawQR(img, text) {
  const mods = qrMatrix(text);
  const n = mods.length, quiet = 4;
  const scale = Math.max(2, Math.floor(160 / (n + quiet * 2)));
  const dim = (n + quiet * 2) * scale;
  const c = document.createElement("canvas");
  c.width = c.height = dim;
  const ctx = c.getContext("2d");
  ctx.fillStyle = "#fff";
  ctx.fillRect(0, 0, dim, dim);
  ctx.fillStyle = "#000";
  for (let y = 0; y < n; y++) {
    for (let x = 0; x < n; x++) {
      if (mods[y][x]) ctx.fillRect((x + quiet) * scale, (y + quiet) * scale, scale, scale);
    }
  }
  img.src = c.toDataURL("image/png");
}

const QR_CAP_L = [0, 17, 32, 53, 78, 106, 134, 154, 192, 232, 271];
const QR_DATA_L = [0, 19, 34, 55, 80, 108, 136, 156, 194, 232, 274];
const QR_ECW_L = [0, 7, 10, 15, 20, 26, 36, 40, 48, 60, 72];
const QR_ECB_L = [0, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4];
const QR_ALIGN = [
  [], [], [6, 18], [6, 22], [6, 26], [6, 30], [6, 34], [6, 22, 38], [6, 24, 42], [6, 26, 46], [6, 28, 50]
];

function qrMatrix(text) {
  const data = new TextEncoder().encode(text);
  let ver = 1;
  while (ver <= 10 && data.length + 2 > QR_CAP_L[ver]) ver++;
  if (ver > 10) throw new Error("qr too long");
  const size = 21 + (ver - 1) * 4;
  const dataCw = QR_DATA_L[ver];
  const ecLen = QR_ECW_L[ver];
  const blocks = QR_ECB_L[ver];
  const bits = [];
  putBits(bits, 0x4, 4);
  putBits(bits, data.length, ver < 10 ? 8 : 16);
  for (const b of data) putBits(bits, b, 8);
  const capBits = dataCw * 8;
  putBits(bits, 0, Math.min(4, capBits - bits.length));
  while (bits.length % 8) bits.push(0);
  const bytes = [];
  for (let i = 0; i < bits.length; i += 8) {
    let v = 0;
    for (let j = 0; j < 8; j++) v = (v << 1) | (bits[i + j] || 0);
    bytes.push(v);
  }
  const pad = [0xec, 0x11];
  let p = 0;
  while (bytes.length < dataCw) {
    bytes.push(pad[p % 2]);
    p++;
  }
  const rs = reedSolomon(bytes, ecLen, blocks);
  const reserved = qrReserved(size, ver);
  const best = bestMask(size, ver, rs, reserved);
  return best;
}

function putBits(bits, val, n) {
  for (let i = n - 1; i >= 0; i--) bits.push((val >> i) & 1);
}

function gfMul(a, b, exp, log) {
  if (!a || !b) return 0;
  return exp[(log[a] + log[b]) % 255];
}

function rsTables() {
  const exp = new Uint8Array(512), log = new Uint8Array(256);
  let x = 1;
  for (let i = 0; i < 255; i++) {
    exp[i] = x;
    log[x] = i;
    x <<= 1;
    if (x & 0x100) x ^= 0x11d;
  }
  for (let i = 255; i < 512; i++) exp[i] = exp[i - 255];
  return { exp, log };
}

function rsGenerator(ecLen, exp, log) {
  let gen = [1];
  for (let i = 0; i < ecLen; i++) {
    const next = new Array(gen.length + 1).fill(0);
    for (let j = 0; j < gen.length; j++) {
      next[j] ^= gen[j];
      next[j + 1] ^= gfMul(gen[j], exp[i], exp, log);
    }
    gen = next;
  }
  return gen;
}

function reedSolomon(data, ecTotal, blocks) {
  const { exp, log } = rsTables();
  const n = data.length;
  const base = Math.floor(n / blocks);
  const extra = n % blocks;
  const groups = [];
  let off = 0;
  for (let b = 0; b < blocks; b++) {
    const dl = base + (b >= blocks - extra ? 1 : 0);
    groups.push(data.slice(off, off + dl));
    off += dl;
  }
  const ecEach = Math.floor(ecTotal / blocks);
  const gen = rsGenerator(ecEach, exp, log);
  const ecs = groups.map((block) => {
    const rec = new Uint8Array(block.length + ecEach);
    rec.set(block);
    for (let i = 0; i < block.length; i++) {
      const coef = rec[i];
      if (!coef) continue;
      for (let j = 0; j < gen.length; j++) rec[i + j] ^= gfMul(coef, gen[j], exp, log);
    }
    return Array.from(rec.slice(block.length));
  });
  const out = [];
  const maxD = Math.max(...groups.map((g) => g.length));
  for (let i = 0; i < maxD; i++) for (const g of groups) if (i < g.length) out.push(g[i]);
  for (let i = 0; i < ecEach; i++) for (const e of ecs) out.push(e[i]);
  return out;
}

function qrReserved(size, ver) {
  const r = Array.from({ length: size }, () => new Uint8Array(size));
  function fill(x, y, w, h) {
    for (let j = 0; j < h; j++) for (let i = 0; i < w; i++) {
      if (x + i >= 0 && y + j >= 0 && x + i < size && y + j < size) r[y + j][x + i] = 1;
    }
  }
  function finder(x, y) { fill(x - 1, y - 1, 9, 9); }
  finder(0, 0); finder(size - 7, 0); finder(0, size - 7);
  fill(8, 0, 1, 9); fill(0, 8, 9, 1);
  fill(size - 8, 8, 8, 1); fill(8, size - 8, 1, 8);
  for (let i = 0; i < size; i++) { r[6][i] = 1; r[i][6] = 1; }
  for (const y of QR_ALIGN[ver]) for (const x of QR_ALIGN[ver]) {
    if ((x < 9 && y < 9) || (x > size - 10 && y < 9) || (x < 9 && y > size - 10)) continue;
    fill(x - 2, y - 2, 5, 5);
  }
  if (ver >= 7) {
    fill(0, size - 11, 6, 3);
    fill(size - 11, 0, 3, 6);
  }
  return r;
}

function placeFinders(m, size) {
  function finder(x, y) {
    for (let j = 0; j < 7; j++) for (let i = 0; i < 7; i++) {
      const edge = i === 0 || i === 6 || j === 0 || j === 6;
      const core = i >= 2 && i <= 4 && j >= 2 && j <= 4;
      m[y + j][x + i] = edge || core ? 1 : 0;
    }
  }
  finder(0, 0); finder(size - 7, 0); finder(0, size - 7);
  for (let i = 0; i < size; i++) {
    m[6][i] = i % 2 === 0 ? 1 : 0;
    m[i][6] = i % 2 === 0 ? 1 : 0;
  }
}

function placeAlign(m, ver, size) {
  for (const y of QR_ALIGN[ver]) for (const x of QR_ALIGN[ver]) {
    if ((x < 9 && y < 9) || (x > size - 10 && y < 9) || (x < 9 && y > size - 10)) continue;
    for (let j = -2; j <= 2; j++) for (let i = -2; i <= 2; i++) {
      const a = Math.max(Math.abs(i), Math.abs(j));
      m[y + j][x + i] = a !== 1 ? 1 : 0;
    }
  }
}

function maskFn(id, x, y) {
  switch (id) {
    case 0: return (x + y) % 2 === 0;
    case 1: return y % 2 === 0;
    case 2: return x % 3 === 0;
    case 3: return (x + y) % 3 === 0;
    case 4: return (Math.floor(y / 2) + Math.floor(x / 3)) % 2 === 0;
    case 5: return ((x * y) % 2) + ((x * y) % 3) === 0;
    case 6: return (((x * y) % 2) + ((x * y) % 3)) % 2 === 0;
    default: return (((x + y) % 2) + ((x * y) % 3)) % 2 === 0;
  }
}

function formatBits(mask) {
  const data = (0b01 << 3) | mask;
  let d = data << 10;
  const gen = 0b10100110111;
  for (let i = 14; i >= 10; i--) if ((d >> i) & 1) d ^= gen << (i - 10);
  return (data << 10 | d) ^ 0b101010000010010;
}

function applyFormat(m, size, mask) {
  const f = formatBits(mask);
  const bit = (i) => (f >> (14 - i)) & 1;
  const pos = [[0,8],[1,8],[2,8],[3,8],[4,8],[5,8],[7,8],[8,8],[8,7],[8,5],[8,4],[8,3],[8,2],[8,1],[8,0]];
  pos.forEach(([x, y], i) => { m[y][x] = bit(i); });
  const pos2 = [[8,size-1],[8,size-2],[8,size-3],[8,size-4],[8,size-5],[8,size-6],[8,size-7],[size-8,8],[size-7,8],[size-6,8],[size-5,8],[size-4,8],[size-3,8],[size-2,8],[size-1,8]];
  pos2.forEach(([x, y], i) => { m[y][x] = bit(i); });
  m[size - 8][8] = 1;
}

function placeData(m, reserved, data, mask) {
  const size = m.length;
  let bit = 0, bits = [];
  for (const b of data) for (let i = 7; i >= 0; i--) bits.push((b >> i) & 1);
  let dir = -1, col = size - 1;
  while (col > 0) {
    if (col === 6) col--;
    for (let i = 0; i < size; i++) {
      const y = dir < 0 ? size - 1 - i : i;
      for (let dx = 0; dx < 2; dx++) {
        const x = col - dx;
        if (reserved[y][x]) continue;
        let v = bits[bit++] || 0;
        if (maskFn(mask, x, y)) v ^= 1;
        m[y][x] = v;
      }
    }
    dir *= -1;
    col -= 2;
  }
}

function scoreQR(m) {
  const size = m.length;
  let s = 0;
  for (let y = 0; y < size; y++) {
    let run = 1;
    for (let x = 1; x < size; x++) {
      if (m[y][x] === m[y][x - 1]) run++;
      else { if (run >= 5) s += run - 2; run = 1; }
    }
    if (run >= 5) s += run - 2;
  }
  for (let x = 0; x < size; x++) {
    let run = 1;
    for (let y = 1; y < size; y++) {
      if (m[y][x] === m[y - 1][x]) run++;
      else { if (run >= 5) s += run - 2; run = 1; }
    }
    if (run >= 5) s += run - 2;
  }
  let dark = 0;
  for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) if (m[y][x]) dark++;
  s += Math.abs(Math.floor(dark * 100 / (size * size) / 5) - 10) * 10;
  return s;
}

function bestMask(size, ver, data, reserved) {
  let best = null, bestScore = Infinity;
  for (let mask = 0; mask < 8; mask++) {
    const m = Array.from({ length: size }, () => new Uint8Array(size));
    placeFinders(m, size);
    placeAlign(m, ver, size);
    placeData(m, reserved, data, mask);
    applyFormat(m, size, mask);
    const sc = scoreQR(m);
    if (sc < bestScore) { bestScore = sc; best = m; }
  }
  return best;
}
