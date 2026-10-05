package site

import (
	"cmp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/RafaelPanisset/reduto/internal/tse"
)

type summary struct {
	Total    int
	Cities   int    // municípios com ao menos um voto
	Top      string // município com mais votos; empate fica com o menor código, para o resultado não variar
	TopVotes int
}

func summarize(byCity map[string]int) summary {
	var s summary
	for city, v := range byCity {
		s.Total += v
		if v == 0 {
			continue
		}
		s.Cities++
		if v > s.TopVotes || (v == s.TopVotes && city < s.Top) {
			s.Top, s.TopVotes = city, v
		}
	}
	return s
}

// own devolve a cidade onde o candidato teve a maior fatia dos votos do cargo, entre as cidades
// com pelo menos min votos. Empate fica com mais votos e depois com o menor código. As frações
// são comparadas em inteiros (a/b > c/d ⇔ a·d > c·b) para não depender de arredondamento.
func own(byCity, totals map[string]int, min int) (city string, votes, total int) {
	for c, v := range byCity {
		t := totals[c]
		if v == 0 || t < min {
			continue
		}
		if city == "" || cmp.Or(cmp.Compare(v*total, votes*t), cmp.Compare(v, votes), cmp.Compare(city, c)) > 0 {
			city, votes, total = c, v, t
		}
	}
	return city, votes, total
}

// officeTotals soma, por cargo e município, os votos de todos os candidatos. É o denominador
// de "quantos % dos votos da cidade foram para este candidato".
func officeTotals(v *tse.Votes) map[int]map[string]int {
	out := map[int]map[string]int{}
	for id, byCity := range v.ByCity {
		office := v.Candidates[id].Office
		if out[office] == nil {
			out[office] = map[string]int{}
		}
		for city, n := range byCity {
			out[office][city] += n
		}
	}
	return out
}

// Palavras que ficam em minúsculas no meio do nome: "Campos dos Goytacazes".
var particles = map[string]bool{"de": true, "da": true, "do": true, "das": true, "dos": true, "e": true}

// Algarismos romanos ficam em maiúsculas: "Pedro II", "Pio IX". "I" fica de fora: sozinho,
// é mais provável que seja uma inicial.
var romans = map[string]bool{
	"II": true, "III": true, "IV": true, "V": true, "VI": true, "VII": true, "VIII": true,
	"IX": true, "X": true, "XI": true, "XII": true, "XIII": true, "XIV": true, "XV": true,
}

// titleCase converte os nomes em maiúsculas do TSE: "PINGO-D'ÁGUA" → "Pingo-d'Água",
// "MIRASSOL D'OESTE" → "Mirassol d'Oeste", "SANT'ANA DO LIVRAMENTO" → "Sant'Ana do Livramento".
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if romans[w] {
			continue
		}
		lw := strings.ToLower(w)
		if i > 0 && particles[lw] {
			words[i] = lw
			continue
		}
		parts := strings.Split(lw, "-")
		for j, p := range parts {
			if pre, post, ok := strings.Cut(p, "'"); ok {
				if pre == "d" {
					parts[j] = "d'" + upperFirst(post)
				} else {
					parts[j] = upperFirst(pre) + "'" + upperFirst(post)
				}
				continue
			}
			parts[j] = upperFirst(p)
		}
		words[i] = strings.Join(parts, "-")
	}
	return strings.Join(words, " ")
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}
