#!/usr/bin/env bash
# Baixa os dados de entrada para raw/ (fora do git).
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p raw/geo

# fetch URL DESTINO
# - Grava em DESTINO.part e só move para DESTINO quando o download termina: um download
#   cortado no meio nunca fica no lugar do arquivo bom.
# - -R guarda no arquivo a data do servidor, e -z só baixa de novo se o servidor tiver uma
#   versão mais nova que essa (senão o servidor responde 304 e nada é gravado).
# - Se o servidor estiver fora do ar e já existe uma cópia, segue com ela.
fetch() {
  local url=$1 dest=$2
  local since=()
  [ -s "$dest" ] && since=(-z "$dest")
  rm -f "$dest.part"
  if ! curl -fsSR --retry 3 "${since[@]}" -o "$dest.part" "$url"; then
    rm -f "$dest.part"
    if [ -s "$dest" ]; then
      echo "aviso: falha ao baixar $url; usando a cópia de $(date -r "$dest" +%d/%m/%Y)" >&2
      return 0
    fi
    echo "erro: falha ao baixar $url" >&2
    return 1
  fi
  if [ -s "$dest.part" ]; then mv "$dest.part" "$dest"; else rm -f "$dest.part"; fi
}

ODS=https://cdn.tse.jus.br/estatistica/sead/odsele
# Votos de cada candidato por município e zona (1º turno de 2026).
fetch "$ODS/votacao_candidato_munzona/votacao_candidato_munzona_2026.zip" raw/votacao.zip
# Cadastro de candidatos de 2026: gênero, nome completo e data de nascimento.
fetch "$ODS/consulta_cand/consulta_cand_2026.zip" raw/candidatos.zip
# Cadastros das eleições municipais: quem já disputou prefeitura ou câmara, e onde.
for ano in 2024 2020; do
  fetch "$ODS/consulta_cand/consulta_cand_$ano.zip" "raw/candidatos_$ano.zip"
done
# Municípios da apuração: a única fonte do TSE que liga o código TSE ao código IBGE.
fetch https://resultados.tse.jus.br/oficial/ele2026/6257/config/mun-e006257-cm.json raw/municipios.json

# Malha dos municípios de cada UF (TopoJSON, qualidade mínima: o mapa é pequeno). A malha não
# muda; o arquivo só existe se o download terminou (ver fetch), então basta pular os que já há.
for uf in AC AL AM AP BA CE DF ES GO MA MG MS MT PA PB PE PI PR RJ RN RO RR RS SC SE SP TO; do
  [ -s "raw/geo/$uf.json" ] && continue
  fetch "https://servicodados.ibge.gov.br/api/v4/malhas/estados/$uf?intrarregiao=municipio&qualidade=minima&formato=application/json" "raw/geo/$uf.json"
done
echo "ok: raw/ atualizado"
