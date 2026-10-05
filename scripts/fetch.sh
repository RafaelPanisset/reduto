#!/usr/bin/env bash
# Baixa os dados de entrada para raw/ (fora do git).
# -z só baixa de novo se o arquivo do servidor for mais novo que o local.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p raw/geo

ODS=https://cdn.tse.jus.br/estatistica/sead/odsele
# Votos de cada candidato por município e zona (1º turno de 2026).
curl -fsS -z raw/votacao.zip -o raw/votacao.zip "$ODS/votacao_candidato_munzona/votacao_candidato_munzona_2026.zip"
# Cadastro de candidatos de 2026: gênero, nome completo e data de nascimento.
curl -fsS -z raw/candidatos.zip -o raw/candidatos.zip "$ODS/consulta_cand/consulta_cand_2026.zip"
# Cadastros das eleições municipais: quem já disputou prefeitura ou câmara, e onde.
for ano in 2024 2020; do
  curl -fsS -z "raw/candidatos_$ano.zip" -o "raw/candidatos_$ano.zip" "$ODS/consulta_cand/consulta_cand_$ano.zip"
done
# Municípios da apuração: a única fonte do TSE que liga o código TSE ao código IBGE.
curl -fsS -o raw/municipios.json https://resultados.tse.jus.br/oficial/ele2026/6257/config/mun-e006257-cm.json

# Malha dos municípios de cada UF (TopoJSON, qualidade mínima: o mapa é pequeno).
for uf in AC AL AM AP BA CE DF ES GO MA MG MS MT PA PB PE PI PR RJ RN RO RR RS SC SE SP TO; do
  [ -s "raw/geo/$uf.json" ] && continue
  curl -fsS -o "raw/geo/$uf.json" \
    "https://servicodados.ibge.gov.br/api/v4/malhas/estados/$uf?intrarregiao=municipio&qualidade=minima&formato=application/json"
done
echo "ok: raw/ atualizado"
