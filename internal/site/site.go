// Package site gera os arquivos estáticos (JSON) que o front-end lê.
//
// A eleição acabou e os números não mudam mais, então toda a conta é feita uma vez aqui e o
// site é só arquivo estático: não há servidor nem banco no ar.
//
// Arquivos gerados em outDir:
//
//	candidates.json  um resumo por candidato (busca e rankings)
//	c/<id>.json      votos do candidato em cada município (carregado ao abrir o candidato)
//	uf/<UF>.json     municípios da UF com o total de votos de cada cargo
//	geo/<UF>.json    malha dos municípios da UF (TopoJSON do IBGE, copiado como veio)
package site

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/RafaelPanisset/reduto/internal/tse"
)

type Input struct {
	Votes  *tse.Votes
	People map[string]tse.Person // cadastro de 2026: ID → pessoa
	Runs   []tse.Run             // candidaturas nas eleições municipais (2020, 2024)
	Cities map[string]tse.City   // código TSE → município
	GeoDir string                // um <UF>.json do IBGE por UF
}

// Report conta o que o build produziu, para o log.
type Report struct {
	Candidates  int // candidatos com voto
	WithHistory int // desses, quantos aparecem nas eleições municipais
}

type candidateOut struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Gender  string `json:"gender,omitempty"`
	Party   string `json:"party"`
	UF      string `json:"uf"`
	Office  int    `json:"office"`
	Number  string `json:"number"`
	Elected bool   `json:"elected"`
	Status  string `json:"status"` // como o TSE escreve: "ELEITO POR QP", "SUPLENTE"...
	Votes   int    `json:"votes"`
	Cities  int    `json:"cities"` // municípios onde teve ao menos um voto
	// O reduto: o município onde teve mais votos.
	Top      string `json:"top"`
	TopName  string `json:"topName"`
	TopVotes int    `json:"topVotes"`
	TopTotal int    `json:"topTotal"` // votos para o mesmo cargo, de todos os candidatos, nesse município
	// Por que ali: a candidatura municipal mais forte no reduto e se o prefeito de lá é do mesmo partido.
	Local      *localOut `json:"local,omitempty"`
	MayorParty bool      `json:"mayorParty,omitempty"`
}

// detailOut é o c/<id>.json: carregado só quando alguém abre o candidato.
type detailOut struct {
	Votes   []cityVotes `json:"votes"`
	History []runOut    `json:"history,omitempty"`
}

// cityVotes vira ["31879", 5872] no JSON: são milhões de pares, e sem os nomes dos campos
// os arquivos ficam com cerca de metade do tamanho.
type cityVotes struct {
	City  string
	Votes int
}

// O código do município só tem dígitos, então dá para montar o JSON sem escapar nada.
func (c cityVotes) MarshalJSON() ([]byte, error) {
	return []byte(`["` + c.City + `",` + strconv.Itoa(c.Votes) + `]`), nil
}

type indexOut struct {
	Generated  time.Time                 `json:"generated"` // quando o TSE gerou o arquivo de votos
	Totals     map[string]map[string]int `json:"totals"`    // UF → cargo → votos no estado
	Candidates []candidateOut            `json:"candidates"`
}

type cityOut struct {
	TSE    string         `json:"tse"`
	IBGE   string         `json:"ibge"`
	Name   string         `json:"name"`
	Totals map[string]int `json:"totals"` // cargo → votos de todos os candidatos
	Mayor  *mayorOut      `json:"mayor,omitempty"`
}

type ufOut struct {
	UF     string         `json:"uf"`
	Totals map[string]int `json:"totals"` // cargo → votos no estado
	Cities []cityOut      `json:"cities"`
}

// Build escreve os arquivos em outDir. Monta tudo em um diretório temporário e só no fim
// troca pelo antigo, para nunca deixar no lugar um site gerado pela metade.
func Build(in Input, outDir string) (rep Report, err error) {
	tmp := outDir + ".tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return rep, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tmp)
		}
	}()
	for _, d := range []string{"c", "uf", "geo"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			return rep, err
		}
	}

	totals := officeTotals(in.Votes)
	hist := newHistory(in.Runs)
	var unknown []string
	index := indexOut{Generated: in.Votes.GeneratedAt, Totals: map[string]map[string]int{}}
	for office, byCity := range totals {
		for city, v := range byCity {
			uf := in.Cities[city].UF
			if index.Totals[uf] == nil {
				index.Totals[uf] = map[string]int{}
			}
			index.Totals[uf][strconv.Itoa(office)] += v
		}
	}
	for id, c := range in.Votes.Candidates {
		byCity := in.Votes.ByCity[id]
		s := summarize(byCity)
		if s.Total == 0 {
			continue
		}
		votes := make([]cityVotes, 0, s.Cities)
		for city, v := range byCity {
			if _, ok := in.Cities[city]; !ok {
				unknown = append(unknown, city)
			}
			if v > 0 {
				votes = append(votes, cityVotes{city, v})
			}
		}
		slices.SortFunc(votes, func(a, b cityVotes) int {
			return cmp.Or(cmp.Compare(b.Votes, a.Votes), cmp.Compare(a.City, b.City))
		})
		person := in.People[id]
		runs := hist.of(person)
		if len(runs) > 0 {
			rep.WithHistory++
		}
		if err := writeJSON(filepath.Join(tmp, "c", id+".json"), detailOut{Votes: votes, History: runs}); err != nil {
			return rep, err
		}
		mayor := hist.mayor(s.Top)
		index.Candidates = append(index.Candidates, candidateOut{
			ID: id, Name: titleCase(c.Name), Gender: person.Gender, Party: c.Party, UF: c.UF,
			Office: c.Office, Number: c.Number, Elected: strings.HasPrefix(c.Status, "ELEITO"), Status: c.Status,
			Votes: s.Total, Cities: s.Cities,
			Top: s.Top, TopName: titleCase(in.Cities[s.Top].Name), TopVotes: s.TopVotes, TopTotal: totals[c.Office][s.Top],
			Local: local(runs, s.Top), MayorParty: mayor != nil && mayor.Party == c.Party,
		})
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		unknown = slices.Compact(unknown)
		return rep, fmt.Errorf("%d municípios com votos não estão no cadastro de municípios: %v", len(unknown), unknown)
	}
	// Ordem estável (mais votados primeiro): o mesmo dado gera sempre o mesmo arquivo.
	slices.SortFunc(index.Candidates, func(a, b candidateOut) int {
		return cmp.Or(cmp.Compare(b.Votes, a.Votes), cmp.Compare(a.ID, b.ID))
	})
	if err := writeJSON(filepath.Join(tmp, "candidates.json"), index); err != nil {
		return rep, err
	}

	byUF := map[string][]tse.City{}
	for _, c := range in.Cities {
		byUF[c.UF] = append(byUF[c.UF], c)
	}
	for uf, cities := range byUF {
		if err := writeUF(tmp, uf, cities, totals, hist); err != nil {
			return rep, err
		}
		if err := copyGeo(in.GeoDir, tmp, uf, cities); err != nil {
			return rep, err
		}
	}

	if err := os.RemoveAll(outDir); err != nil {
		return rep, err
	}
	rep.Candidates = len(index.Candidates)
	return rep, os.Rename(tmp, outDir)
}

func writeUF(dir, uf string, cities []tse.City, totals map[int]map[string]int, hist history) error {
	out := ufOut{UF: uf, Totals: map[string]int{}}
	for _, c := range cities {
		co := cityOut{TSE: c.Code, IBGE: c.IBGE, Name: titleCase(c.Name), Totals: map[string]int{}, Mayor: hist.mayor(c.Code)}
		for office, byCity := range totals {
			if v := byCity[c.Code]; v > 0 {
				co.Totals[strconv.Itoa(office)] = v
				out.Totals[strconv.Itoa(office)] += v
			}
		}
		out.Cities = append(out.Cities, co)
	}
	slices.SortFunc(out.Cities, func(a, b cityOut) int { return cmp.Compare(a.TSE, b.TSE) })
	return writeJSON(filepath.Join(dir, "uf", uf+".json"), out)
}

// copyGeo copia a malha da UF e confere que todo município do TSE tem um polígono com o
// mesmo código IBGE. As duas fontes não foram feitas para se juntar: é aqui que um
// município novo ou um código trocado apareceria, em vez de virar um buraco no mapa.
func copyGeo(geoDir, outDir, uf string, cities []tse.City) error {
	src := filepath.Join(geoDir, uf+".json")
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	var topo struct {
		Objects map[string]struct {
			Geometries []struct {
				Properties struct {
					Code string `json:"codarea"`
				} `json:"properties"`
			} `json:"geometries"`
		} `json:"objects"`
	}
	if err := json.Unmarshal(data, &topo); err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	has := map[string]bool{}
	for _, o := range topo.Objects {
		for _, g := range o.Geometries {
			has[g.Properties.Code] = true
		}
	}
	var missing []string
	for _, c := range cities {
		if !has[c.IBGE] {
			missing = append(missing, fmt.Sprintf("%s (IBGE %q)", c.Name, c.IBGE))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: %d municípios sem polígono: %v", src, len(missing), missing)
	}
	return os.WriteFile(filepath.Join(outDir, "geo", uf+".json"), data, 0o644)
}

func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
