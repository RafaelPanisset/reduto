# Reduto

De onde vieram os votos de cada candidato a deputado nas eleições de 2026. Tem quem foi eleito por uma cidade só.

Você digita o nome de um candidato e vê, num mapa do estado, quanto cada município votou nele. O bloco **"Por que aqui?"** mostra o que o TSE registra sobre o candidato e a cidade-reduto: se ele já foi vereador, prefeito ou vice ali, e quem é o prefeito. Há também três rankings dos eleitos: os que dependem de uma cidade só, os que "fecharam" uma cidade e os mais espalhados.

## O que aparece nos dados (1º turno, 4/10/2026)

- **77 dos 513** deputados federais eleitos tiveram mais da metade dos votos em um único município. Entre os estaduais foram **260 de 1.035**, um em cada quatro.
- **Fabão (PV/MG)**, deputado estadual: 98% dos 52.940 votos vieram de Uberlândia.
- **Thiago Surfista (PL/SP)**, deputado federal: 88% dos 161.971 votos vieram de Guarulhos, uma cidade com 2,9% dos votos do estado. Ele foi eleito vereador de Guarulhos em 2020 e vice-prefeito em 2024.
- Dos **329** deputados eleitos com mais da metade dos votos numa cidade só, **149 (45%)** já tinham sido eleitos vereador, prefeito ou vice naquela cidade em 2020 ou 2024.
- **Ricardo Maia (MDB/BA)**: em Tucano, 74% dos votos para deputado federal foram para ele.
- No outro extremo, **Padre João (PT/MG)** teve votos em 783 cidades, e nenhuma deu mais de 4% do total dele.

## Como funciona

```
TSE: votos por município e zona (316 MB zip, ~3 GB de CSV) ─┐
TSE: cadastro de candidatos de 2026                         │
TSE: cadastros das eleições municipais de 2020 e 2024       ├─► cmd/reduto (Go) ─► site/data/*.json ─► site estático
TSE: municípios da apuração (código TSE ↔ código IBGE)      │       ~12 s
IBGE: malha dos municípios de cada UF (TopoJSON)           ─┘
```

A eleição acabou e os números não mudam mais. Por isso toda a conta é feita uma vez, no build, e o site é só arquivo estático: não há servidor nem banco no ar. O navegador baixa a lista de candidatos (757 KB compactada) e, ao abrir um candidato, três arquivos pequenos: os votos dele, os totais da UF e o mapa da UF.

## Rodar localmente

```sh
./scripts/fetch.sh          # baixa os dados para raw/ (~470 MB)
go run ./cmd/reduto         # gera site/data/
cd site && python3 -m http.server 8000   # abre http://localhost:8000
```

Testes: `go test ./...`. Requer Go 1.27 (com uma versão anterior, o Go baixa a certa sozinho).

## Decisões

- **Uma goroutine por UF.** O CSV vem um por estado, e estados não compartilham candidatos nem municípios. Ler em paralelo e juntar os mapas no fim custa pouco e leva a leitura de ~3 GB para ~4 s. O arquivo `_BRASIL.csv` repete todas as UFs e é ignorado, senão os votos contariam em dobro (há teste para isso).
- **Colunas pelo nome, não pela posição.** Se o TSE reordenar as colunas em outra eleição, o código continua certo; se uma sumir, o build para com erro.
- **O build valida a junção TSE ↔ IBGE.** As duas fontes não foram feitas para se juntar. Se algum município do TSE não tiver polígono na malha do IBGE, o build falha em vez de publicar um mapa com buraco.
- **Saída atômica.** Os arquivos são gerados em `site/data.tmp` e só trocam de lugar com `site/data` no fim. Um build que falha no meio não deixa um site pela metade.
- **Encoding e fuso.** Os CSVs do TSE são latin1 (convertidos campo a campo, sem dependência) e os horários são de Brasília (UTC−3 fixo, sem horário de verão desde 2019).
- **A mesma pessoa em eleições diferentes: nome completo + data de nascimento.** Em 2024 o TSE não divulga o CPF. Esse par, conferido contra o CPF onde ele existe (2026 × 2020), bate em 6.085 de 6.086 casos. O CPF não entra no build nem no site.
- **Só fatos no "Por que aqui?".** O site mostra quem disputou o quê e onde, e o nome completo do prefeito, mas não afirma parentesco: sobrenome igual não prova família. Quando o prefeito de Tucano se chama Ricardo Maia Chaves de Souza Filho e o deputado se chama Ricardo Maia Chaves de Souza, quem lê tira a conclusão.
- **Situação final da candidatura municipal.** O cadastro traz uma linha por turno; vale a do último. Candidaturas indeferidas ou retiradas ficam de fora.
- **"De uma cidade só" ignora cidades com 10% ou mais dos votos do estado.** Sem esse filtro o ranking vira uma lista de capitais: Macapá sozinha tem mais da metade dos votos do Amapá, e concentrar votos ali não diz nada.
- **Cores em raiz quadrada.** No mapa, a cor é a parte dos votos da cidade que foi para o candidato. Em escala linear só o reduto aparece; com a raiz, as cidades com 1% ou 2% também ficam visíveis.

## Limites

- Contam os votos **nominais válidos**: votos de legenda, brancos e nulos ficam de fora.
- O reduto é o **município** com mais votos. Dentro das capitais, o voto por zona ou bairro mostraria mais, mas isso fica para outra vez.
- O Distrito Federal tem um município só e não tem eleição municipal, então fica fora dos rankings e do "Por que aqui?".
- "Mesmo partido" compara siglas: o partido do prefeito eleito em 2024 com o do candidato em 2026. Federações e trocas de partido não entram na conta.
- O que o TSE não registra (família, igreja, rádio, sindicato) não aparece. Um terço dos eleitos "de uma cidade só" não tem nenhuma ligação com a cidade nos dados.

## Fontes

- [TSE, dados abertos](https://dadosabertos.tse.jus.br/): `votacao_candidato_munzona_2026` e `consulta_cand` de 2026, 2024 e 2020.
- [TSE, resultados](https://resultados.tse.jus.br/): cadastro de municípios da apuração (`mun-e006257-cm.json`).
- [IBGE, API de malhas](https://servicodados.ibge.gov.br/api/docs/malhas).
- Mapa desenhado com [d3-geo](https://d3js.org/d3-geo) e [topojson-client](https://github.com/topojson/topojson-client), copiados em `site/vendor/`.
