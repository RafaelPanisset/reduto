// Reduto: busca, ficha do candidato com mapa e rankings. Tudo vem dos arquivos JSON gerados
// pelo comando cmd/reduto; não há servidor.

const OFFICES = {
  6: { m: 'Deputado federal', f: 'Deputada federal', vote: 'deputado federal' },
  7: { m: 'Deputado estadual', f: 'Deputada estadual', vote: 'deputado estadual' },
  8: { m: 'Deputado distrital', f: 'Deputada distrital', vote: 'deputado distrital' },
};

// [preposição, nome]: "em São Paulo", "na Bahia", "no Acre".
const UFS = {
  AC: ['no', 'Acre'], AL: ['em', 'Alagoas'], AM: ['no', 'Amazonas'], AP: ['no', 'Amapá'],
  BA: ['na', 'Bahia'], CE: ['no', 'Ceará'], DF: ['no', 'Distrito Federal'], ES: ['no', 'Espírito Santo'],
  GO: ['em', 'Goiás'], MA: ['no', 'Maranhão'], MG: ['em', 'Minas Gerais'], MS: ['no', 'Mato Grosso do Sul'],
  MT: ['no', 'Mato Grosso'], PA: ['no', 'Pará'], PB: ['na', 'Paraíba'], PE: ['em', 'Pernambuco'],
  PI: ['no', 'Piauí'], PR: ['no', 'Paraná'], RJ: ['no', 'Rio de Janeiro'], RN: ['no', 'Rio Grande do Norte'],
  RO: ['em', 'Rondônia'], RR: ['em', 'Roraima'], RS: ['no', 'Rio Grande do Sul'], SC: ['em', 'Santa Catarina'],
  SE: ['em', 'Sergipe'], SP: ['em', 'São Paulo'], TO: ['no', 'Tocantins'],
};

const $ = (sel, root = document) => root.querySelector(sel);
const nf = new Intl.NumberFormat('pt-BR');
const nf0 = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 0 });
const nf1 = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 1 });

// Porcentagem com uma casa abaixo de 10% ("2,9%") e sem casas acima ("88%"). Nunca mostra
// "100%" para o que não é tudo: 99,8% fica 99,8%.
function pct(x) {
  const v = 100 * x;
  let s = (v > 0 && v < 10 ? nf1 : nf0).format(v);
  if (x < 1 && s === '100') s = nf1.format(Math.floor(v * 10) / 10);
  return s + '%';
}

// Cidades que pedem artigo: "do Rio de Janeiro", "no Recife". As outras: "de Guarulhos", "em Tucano".
const WITH_ARTICLE = new Set(['Rio de Janeiro', 'Recife']);
const dePrep = (city) => (WITH_ARTICLE.has(city) ? 'do' : 'de');
const de = (city) => `${dePrep(city)} ${esc(city)}`;
const em = (city) => `${WITH_ARTICLE.has(city) ? 'no' : 'em'} ${esc(city)}`;
const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);

const esc = (s) => String(s).replace(/[&<>"']/g, (ch) => `&#${ch.charCodeAt(0)};`);
const norm = (s) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
// Concordância de gênero; "N" é quem o TSE marca como "não divulgável".
const gw = (c, m, f, n = `${m}(a)`) => (c.gender === 'F' ? f : c.gender === 'M' ? m : n);
const officeName = (c) => gw(c, OFFICES[c.office].m, OFFICES[c.office].f, OFFICES[c.office].m);

const MUNICIPAL = { 11: ['prefeito', 'prefeita'], 12: ['vice-prefeito', 'vice-prefeita'], 13: ['vereador', 'vereadora'] };
const munOffice = (office, gender) => MUNICIPAL[office][gender === 'F' ? 1 : 0];

// "eleito vice-prefeito de Guarulhos (NOVO)" / "candidato a prefeito de Foz do Iguaçu (PP), não eleito"
function runText(c, r) {
  const office = munOffice(r.office, c.gender);
  return r.elected
    ? `${gw(c, 'eleito', 'eleita')} ${office} ${de(r.cityName)} (${esc(r.party)})`
    : `${gw(c, 'candidato', 'candidata')} a ${office} ${de(r.cityName)} (${esc(r.party)}), ${gw(c, 'não eleito', 'não eleita')}`;
}

// Etiqueta curta dos rankings: o que liga o candidato à cidade-reduto.
function whyLabel(c) {
  const l = c.local;
  if (l) {
    const office = munOffice(l.office, c.gender);
    return l.elected
      ? `${gw(c, 'eleito', 'eleita')} ${office} lá em ${l.year}`
      : `${gw(c, 'candidato', 'candidata')} a ${office} lá em ${l.year}, sem ganhar`;
  }
  return c.mayorParty ? 'a prefeitura de lá é do mesmo partido' : '';
}

function statusLabel(c) {
  if (c.elected) return gw(c, 'Eleito', 'Eleita');
  if (c.status === 'SUPLENTE') return 'Suplente';
  if (c.status === 'NÃO ELEITO') return gw(c, 'Não eleito', 'Não eleita');
  return c.status.charAt(0) + c.status.slice(1).toLowerCase();
}

// Cada arquivo é baixado uma vez só; um erro não fica guardado, para dar para tentar de novo.
const cache = new Map();
function getJSON(url) {
  if (!cache.has(url)) {
    const p = fetch(url).then((r) => {
      if (!r.ok) throw new Error(`${url}: HTTP ${r.status}`);
      return r.json();
    });
    p.catch(() => cache.delete(url));
    cache.set(url, p);
  }
  return cache.get(url);
}

let all = [];
let stateTotals = {};
const byId = new Map();
// Peso da cidade-reduto no estado: quanto dos votos do estado, para o cargo, saiu dela.
const topWeight = (c) => c.topTotal / stateTotals[c.uf][c.office];

init();

async function init() {
  try {
    const data = await getJSON('data/candidates.json');
    all = data.candidates;
    stateTotals = data.totals;
    for (const c of all) {
      c.key = norm(`${c.name} ${c.party} ${c.uf}`);
      byId.set(c.id, c);
    }
    // "2026-10-05T10:14:37-03:00": mostra como está, no horário de Brasília.
    const [d, t] = data.generated.split('T');
    $('#generated').textContent = `gerados pelo TSE em ${d.split('-').reverse().join('/')} às ${t.slice(0, 5)}`;
    setupSearch();
    setupRankings();
    window.addEventListener('hashchange', route);
    route();
  } catch (e) {
    $('#hint').textContent = `Não foi possível carregar os dados (${e.message}).`;
  }
}

// Busca

function setupSearch() {
  const q = $('#q');
  const list = $('#suggestions');
  let items = [];
  let active = -1;

  const render = () => {
    list.innerHTML = items.map((c, i) => `
      <li role="option" id="opt-${i}" data-id="${esc(c.id)}" aria-selected="${i === active}">
        <span class="who">${esc(c.name)}</span>
        <span class="meta">${esc(c.party)}/${esc(c.uf)} · ${officeName(c)} · ${nf.format(c.votes)} votos · ${esc(statusLabel(c))}</span>
      </li>`).join('');
    list.hidden = items.length === 0;
    q.setAttribute('aria-expanded', String(!list.hidden));
    if (active >= 0) q.setAttribute('aria-activedescendant', `opt-${active}`);
    else q.removeAttribute('aria-activedescendant');
  };
  const close = () => { items = []; active = -1; render(); };
  const choose = (c) => {
    q.value = '';
    close();
    q.blur();
    location.hash = c.id;
  };

  q.addEventListener('input', () => {
    const terms = norm(q.value).split(/\s+/).filter(Boolean);
    if (terms.length === 0) return close();
    // Primeiro quem começa com o que foi digitado, depois os eleitos, depois os mais votados.
    items = all
      .filter((c) => terms.every((t) => c.key.includes(t)))
      .sort((a, b) => (b.key.startsWith(terms[0]) - a.key.startsWith(terms[0])) || (b.elected - a.elected) || (b.votes - a.votes))
      .slice(0, 8);
    active = items.length ? 0 : -1;
    render();
  });
  q.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      if (!items.length) return;
      e.preventDefault();
      active = (active + (e.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length;
      render();
    } else if (e.key === 'Enter' && active >= 0) {
      e.preventDefault();
      choose(items[active]);
    } else if (e.key === 'Escape') {
      close();
    }
  });
  // mousedown, e não click: o click viria depois do blur, que já fechou a lista.
  list.addEventListener('mousedown', (e) => {
    const li = e.target.closest('li');
    if (li) {
      e.preventDefault();
      choose(byId.get(li.dataset.id));
    }
  });
  q.addEventListener('blur', close);

  q.disabled = false;
  const examples = [
    ranked(RANKINGS[0], 6, '')[0],
    ranked(RANKINGS[1], 6, '')[0],
    all.find((c) => c.office === 6),
  ];
  $('#hint').innerHTML = 'Experimente: ' + examples
    .map((c) => `<button type="button" data-id="${esc(c.id)}">${esc(c.name)}</button>`).join(' · ');
  $('#hint').addEventListener('click', (e) => {
    const b = e.target.closest('button');
    if (b) location.hash = b.dataset.id;
  });
}

// Ficha do candidato

let routeToken = 0;

async function route() {
  const token = ++routeToken;
  const sec = $('#candidate');
  const c = byId.get(decodeURIComponent(location.hash.slice(1)));
  if (!c) {
    sec.hidden = true;
    sec.innerHTML = '';
    document.title = 'Reduto: de onde vêm os votos dos deputados';
    return;
  }
  sec.hidden = false;
  sec.innerHTML = `<div class="card"><p>Carregando ${esc(c.name)}…</p></div>`;
  sec.scrollIntoView({ behavior: 'smooth', block: 'start' });
  document.title = `${c.name} (${c.party}/${c.uf}): Reduto`;
  try {
    const [detail, uf, topo] = await Promise.all([
      getJSON(`data/c/${c.id}.json`), getJSON(`data/uf/${c.uf}.json`), getJSON(`data/geo/${c.uf}.json`),
    ]);
    // Se outro candidato foi aberto enquanto este carregava, não sobrescreve.
    if (token === routeToken) renderCandidate(sec, c, detail, uf, topo);
  } catch (e) {
    if (token === routeToken) sec.innerHTML = `<div class="card"><p>Não foi possível carregar (${esc(e.message)}).</p></div>`;
  }
}

function renderCandidate(sec, c, detail, uf, topo) {
  const office = String(c.office);
  const cargo = OFFICES[c.office].vote;
  const cities = new Map(uf.cities.map((x) => [x.tse, x]));
  const votes = new Map(detail.votes);
  const top = cities.get(c.top);
  const [prep, ufName] = UFS[c.uf];

  const story = uf.cities.length === 1
    ? `<p class="story">O Distrito Federal tem um município só, então aqui não dá para falar em reduto: os ${nf.format(c.votes)} votos vieram todos ${de(top.name)}.</p>`
    : `<p class="story"><strong>${pct(c.topVotes / c.votes)}</strong> dos votos vieram ${dePrep(top.name)} <strong>${esc(top.name)}</strong>,
         cidade que tem ${pct(top.totals[office] / uf.totals[office])} dos votos para ${cargo} ${prep} ${ufName}.</p>
       <p class="story">${cap(em(top.name))}, <strong>${pct(c.topVotes / c.topTotal)}</strong> dos votos para ${cargo} foram para ${esc(c.name)}.</p>`;

  const map = drawMap(topo, uf, office, votes, top.ibge);
  const why = uf.cities.length === 1 ? '' : whyBlock(c, top, detail.history || []);
  const rows = detail.votes.slice(0, 10).map(([tse, v]) => {
    const city = cities.get(tse);
    return `<tr><td>${esc(city.name)}</td><td class="num">${nf.format(v)}</td>
      <td class="num">${pct(v / c.votes)}</td><td class="num">${pct(v / city.totals[office])}</td></tr>`;
  }).join('');

  sec.innerHTML = `
    <article class="card">
      <div class="card-head">
        <h2>${esc(c.name)}</h2>
        <span class="badge${c.elected ? ' elected' : ''}">${esc(statusLabel(c))}</span>
      </div>
      <p class="sub">${officeName(c)} · ${esc(c.party)} · ${ufName} · nº ${esc(c.number)}</p>
      <div class="stats">
        <div class="stat"><b>${nf.format(c.votes)}</b><span>votos</span></div>
        <div class="stat"><b>${nf.format(c.cities)}</b><span>de ${nf.format(uf.cities.length)} cidades deram voto</span></div>
        <div class="stat"><b>${pct(c.topVotes / c.votes)}</b><span>vieram ${de(top.name)}</span></div>
      </div>
      ${story}
      ${why}
      <p class="share"><button type="button" class="linklike" id="copy">Copiar link desta página</button></p>
      <div class="map-box">
        ${map.svg}
        <div class="tooltip" hidden></div>
      </div>
      <div class="legend">
        <span>Parte dos votos da cidade para ${cargo} que foi para ${esc(c.name)}:</span>
        <span>0%</span><span class="ramp"></span><span>${pct(map.max)}</span>
        <span><span class="key-top"></span> ${esc(top.name)}</span>
      </div>
      <table>
        <caption>Onde ${esc(c.name)} teve mais votos</caption>
        <thead><tr><th>Cidade</th><th>Votos</th><th title="Parte dos votos de ${esc(c.name)} que veio da cidade">Do total</th>
          <th title="Parte dos votos para ${cargo} na cidade que foi para ${esc(c.name)}">Da cidade</th></tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </article>`;

  $('#copy', sec).addEventListener('click', async (e) => {
    try {
      await navigator.clipboard.writeText(location.href);
      e.target.textContent = 'Link copiado';
    } catch {
      e.target.textContent = location.href;
    }
  });

  const box = $('.map-box', sec);
  const tip = $('.tooltip', box);
  const show = (e) => {
    const p = e.target.closest('path');
    const city = p && map.byIBGE.get(p.dataset.ibge);
    if (!city) { tip.hidden = true; return; } // fora do mapa ou área sem município (lagoas do RS)
    const v = votes.get(city.tse) || 0;
    tip.innerHTML = `<strong>${esc(city.name)}</strong><br>${nf.format(v)} votos (${pct(v / c.votes)} do total)<br>
      ${pct(city.totals[office] ? v / city.totals[office] : 0)} dos votos da cidade para ${cargo}`;
    tip.hidden = false;
    const r = box.getBoundingClientRect();
    const x = Math.max(0, Math.min(e.clientX - r.left + 14, r.width - tip.offsetWidth));
    const y = Math.min(e.clientY - r.top + 14, r.height - tip.offsetHeight);
    tip.style.left = `${x}px`;
    tip.style.top = `${y}px`;
  };
  box.addEventListener('pointermove', show);
  box.addEventListener('pointerdown', show); // toque no celular
  // No toque, o navegador dispara pointerleave logo depois de soltar o dedo; ali o tooltip
  // tem que ficar, e some ao tocar fora de um município.
  box.addEventListener('pointerleave', (e) => { if (e.pointerType !== 'touch') tip.hidden = true; });
}

// Por que aqui: o que o TSE registra sobre o candidato e a cidade-reduto. Só fatos; parentesco,
// por exemplo, não aparece, porque sobrenome igual não prova nada. Quem lê tira a conclusão.
function whyBlock(c, top, history) {
  const here = history.filter((r) => r.city === top.tse);
  const elsewhere = history.filter((r) => r.city !== top.tse);
  const items = here.length
    ? here.map((r) => `<li><b>${r.year}</b> ${runText(c, r)}</li>`)
    : [`<li>Não disputou eleição municipal ${em(top.name)} em 2020 nem em 2024.</li>`];
  const m = top.mayor;
  if (m) {
    const same = m.party === c.party ? `, o mesmo partido de ${esc(c.name)}` : '';
    items.push(`<li><b>${m.year}</b> ${m.gender === 'F' ? 'eleita prefeita' : 'eleito prefeito'} ${de(top.name)}:
      ${esc(m.name)} (${esc(m.party)})${same}. <span class="muted">Nome completo: ${esc(m.fullName)}.</span></li>`);
  }
  if (elsewhere.length) {
    items.push(`<li class="muted">Em outras cidades: ${elsewhere.map((r) => `${r.year}, ${runText(c, r)}`).join('; ')}.</li>`);
  }
  return `
    <section class="why">
      <h3>Por que ${esc(top.name)}?</h3>
      <ul>${items.join('')}</ul>
      <p class="muted small">Eleições municipais de 2020 e 2024, cruzadas pelo nome completo e pela data de nascimento no cadastro de candidatos do TSE.</p>
    </section>`;
}

// O d3-geo trabalha na esfera: um anel no sentido "errado" vira o planeta inteiro menos o
// município, e o mapa vira um borrão. Se a área passa de meia esfera, inverte os anéis.
function fixWinding(f) {
  if (d3.geoArea(f) <= 2 * Math.PI) return;
  const g = f.geometry;
  if (g.type === 'Polygon') g.coordinates.forEach((r) => r.reverse());
  if (g.type === 'MultiPolygon') g.coordinates.forEach((p) => p.forEach((r) => r.reverse()));
}

function drawMap(topo, uf, office, votes, topIBGE) {
  const fc = topojson.feature(topo, Object.values(topo.objects)[0]);
  fc.features.forEach(fixWinding);
  const W = 800;
  const path = d3.geoPath(d3.geoMercator().fitWidth(W, fc));
  const H = Math.ceil(path.bounds(fc)[1][1]);

  const byIBGE = new Map(uf.cities.map((x) => [x.ibge, x]));
  const shares = new Map();
  let max = 0;
  for (const city of uf.cities) {
    const total = city.totals[office] || 0;
    const s = total ? (votes.get(city.tse) || 0) / total : 0;
    shares.set(city.ibge, s);
    max = Math.max(max, s);
  }

  // O reduto vai por último, para o contorno dele ficar por cima dos vizinhos.
  const features = [...fc.features].sort((a, b) =>
    (a.properties.codarea === topIBGE) - (b.properties.codarea === topIBGE));
  const paths = features.map((f) => {
    const code = f.properties.codarea;
    const s = shares.get(code) || 0;
    // Raiz quadrada: sem ela, só o reduto aparece e as cidades com 1% ou 2% somem no claro.
    const t = max > 0 ? Math.sqrt(s / max) : 0;
    const fill = s > 0 ? `color-mix(in oklab, var(--map-hi) ${(100 * t).toFixed(1)}%, var(--map-lo))` : 'var(--map-zero)';
    return `<path d="${path(f) || ''}" data-ibge="${esc(code)}" style="fill:${fill}"${code === topIBGE ? ' class="top"' : ''}></path>`;
  }).join('');

  return {
    svg: `<svg viewBox="0 0 ${W} ${H}" role="img" aria-label="Mapa dos municípios com a parte dos votos de cada um">${paths}</svg>`,
    max,
    byIBGE,
  };
}

// Rankings

const RANKINGS = [
  {
    id: 'uma-cidade',
    label: 'De uma cidade só',
    help: 'Eleitos que tiveram a maior parte dos votos em um único município. Ficam de fora as cidades que sozinhas têm 10% ou mais dos votos do estado, como as capitais: concentrar votos nelas não surpreende.',
    filter: (c) => topWeight(c) < 0.1,
    value: (c) => c.topVotes / c.votes,
    order: -1,
    text: (c) => `${pct(c.topVotes / c.votes)} dos votos vieram ${de(c.topName)}, cidade com ${pct(topWeight(c))} dos votos do estado`,
  },
  {
    id: 'donos',
    label: 'Donos da cidade',
    help: 'Eleitos que ficaram com a maior fatia dos votos de um município. Só entram municípios com pelo menos 10 mil votos para o cargo.',
    filter: (c) => c.topTotal >= 10000,
    value: (c) => c.topVotes / c.topTotal,
    order: -1,
    text: (c) => `ficou com ${pct(c.topVotes / c.topTotal)} dos votos para ${OFFICES[c.office].vote} ${em(c.topName)}`,
  },
  {
    id: 'espalhados',
    label: 'Os mais espalhados',
    help: 'Eleitos cujo voto não depende de nenhuma cidade: a que mais deu votos é uma fatia pequena do total.',
    value: (c) => c.topVotes / c.votes,
    order: 1,
    text: (c) => `votos em ${nf.format(c.cities)} cidades; a que mais deu foi ${esc(c.topName)}, com ${pct(c.topVotes / c.votes)}`,
  },
];

function ranked(r, office, uf) {
  return all
    .filter((c) => c.elected && c.office === office && c.uf !== 'DF' && (!uf || c.uf === uf) && (!r.filter || r.filter(c)))
    .sort((a, b) => r.order * (r.value(a) - r.value(b)) || b.votes - a.votes)
    .slice(0, 20);
}

function setupRankings() {
  let current = RANKINGS[0];
  const tabs = $('#tabs');
  const fOffice = $('#f-office');
  const fUF = $('#f-uf');

  tabs.innerHTML = RANKINGS.map((r) =>
    `<button type="button" role="tab" data-id="${r.id}" aria-selected="${r === current}">${r.label}</button>`).join('');
  fUF.innerHTML += Object.entries(UFS)
    .filter(([uf]) => uf !== 'DF')
    .sort((a, b) => a[1][1].localeCompare(b[1][1], 'pt-BR'))
    .map(([uf, [, name]]) => `<option value="${uf}">${name}</option>`).join('');

  const render = () => {
    for (const b of tabs.children) b.setAttribute('aria-selected', String(b.dataset.id === current.id));
    $('#ranking-help').textContent = current.help;
    const list = ranked(current, Number(fOffice.value), fUF.value);
    $('#ranking').innerHTML = list.length === 0
      ? '<li class="help">Nenhum eleito com esse filtro.</li>'
      : list.map((c) => `
        <li><button type="button" data-id="${esc(c.id)}">
          <span class="who">${esc(c.name)} <small>${esc(c.party)}/${esc(c.uf)} · ${nf.format(c.votes)} votos</small></span>
          <span class="what">${current.text(c)}</span>
          <span class="tag">${whyLabel(c)}</span>
          <span class="bar"><i style="width:${(100 * current.value(c)).toFixed(1)}%"></i></span>
        </button></li>`).join('');
  };

  tabs.addEventListener('click', (e) => {
    const b = e.target.closest('button');
    if (!b) return;
    current = RANKINGS.find((r) => r.id === b.dataset.id);
    render();
  });
  fOffice.addEventListener('change', render);
  fUF.addEventListener('change', render);
  $('#ranking').addEventListener('click', (e) => {
    const b = e.target.closest('button');
    if (b) location.hash = b.dataset.id;
  });
  // Quantos dos eleitos "de uma cidade só" já tinham sido eleitos naquela cidade.
  const single = all.filter((c) => c.elected && c.uf !== 'DF' && c.topVotes >= c.votes / 2);
  const local = single.filter((c) => c.local && c.local.elected).length;
  $('#insight').innerHTML = `Dos <strong>${nf.format(single.length)}</strong> deputados eleitos com mais da metade dos votos numa cidade só,
    <strong>${nf.format(local)} (${pct(local / single.length)})</strong> já tinham sido eleitos vereador, prefeito ou vice naquela cidade em 2020 ou 2024.`;
  $('#rankings').hidden = false;
  render();
}
