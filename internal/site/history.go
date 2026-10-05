package site

import (
	"cmp"
	"slices"

	"github.com/RafaelPanisset/reduto/internal/tse"
)

// O histórico municipal de um candidato vem dos cadastros dos ciclos de 2020 e 2024, que
// incluem as eleições suplementares feitas depois (até 2026). A mesma pessoa é encontrada por
// tse.PersonKey: nome completo + data de nascimento.

type runOut struct {
	Year          int    `json:"year"`
	Date          string `json:"date"` // aaaa-mm-dd
	Supplementary bool   `json:"supplementary,omitempty"`
	City          string `json:"city"`
	CityName      string `json:"cityName"`
	UF            string `json:"uf"`
	Office        int    `json:"office"`
	Party         string `json:"party"`
	Elected       bool   `json:"elected"`
}

// localOut resume, para os rankings, a candidatura mais forte do candidato numa cidade.
type localOut struct {
	Year    int  `json:"year"`
	Office  int  `json:"office"`
	Elected bool `json:"elected"`
}

type mayorOut struct {
	Year          int    `json:"year"`
	Supplementary bool   `json:"supplementary,omitempty"`
	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	Party         string `json:"party"`
	Gender        string `json:"gender,omitempty"`
}

type history struct {
	byPerson map[string][]tse.Run
	mayors   map[string]tse.Run // código TSE → prefeito eleito na eleição mais recente da cidade
}

func newHistory(runs []tse.Run) history {
	h := history{byPerson: map[string][]tse.Run{}, mayors: map[string]tse.Run{}}
	for _, r := range runs {
		if k := tse.PersonKey(r.FullName, r.BirthDate); k != "" {
			h.byPerson[k] = append(h.byPerson[k], r)
		}
		if r.Office != tse.Mayor || !r.Elected {
			continue
		}
		// Onde a eleição foi anulada e refeita, há mais de um prefeito eleito no mesmo ciclo:
		// vale o da eleição mais recente. O desempate pelo ID só existe para o resultado não
		// depender da ordem de leitura.
		if prev, ok := h.mayors[r.City]; !ok || cmp.Or(r.Date.Compare(prev.Date), cmp.Compare(r.ID, prev.ID)) > 0 {
			h.mayors[r.City] = r
		}
	}
	return h
}

// matchable diz se dá para procurar a pessoa nos cadastros municipais. Sem data de nascimento
// não dá, e o site não pode afirmar que ela "não disputou".
func matchable(p tse.Person) bool { return tse.PersonKey(p.FullName, p.BirthDate) != "" }

// of devolve as candidaturas municipais da pessoa, da mais antiga para a mais recente.
func (h history) of(p tse.Person) []runOut {
	runs := slices.Clone(h.byPerson[tse.PersonKey(p.FullName, p.BirthDate)])
	slices.SortFunc(runs, func(a, b tse.Run) int {
		return cmp.Or(a.Date.Compare(b.Date), cmp.Compare(a.Office, b.Office), cmp.Compare(a.City, b.City), cmp.Compare(a.ID, b.ID))
	})
	var out []runOut
	for _, r := range runs {
		out = append(out, runOut{
			Year: r.Year, Date: r.Date.Format("2006-01-02"), Supplementary: r.Supplementary,
			City: r.City, CityName: titleCase(r.CityName), UF: r.UF, Office: r.Office, Party: r.Party, Elected: r.Elected,
		})
	}
	return out
}

// local escolhe a candidatura mais forte na cidade: eleito antes de não eleito; prefeito antes
// de vice, vice antes de vereador; a mais recente primeiro.
func local(runs []runOut, city string) *localOut {
	var best *runOut
	for i, r := range runs {
		if r.City != city {
			continue
		}
		if best == nil || cmp.Or(
			-compareBool(r.Elected, best.Elected), cmp.Compare(r.Office, best.Office), -cmp.Compare(r.Date, best.Date),
		) < 0 {
			best = &runs[i]
		}
	}
	if best == nil {
		return nil
	}
	return &localOut{Year: best.Year, Office: best.Office, Elected: best.Elected}
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

func (h history) mayor(city string) *mayorOut {
	r, ok := h.mayors[city]
	if !ok {
		return nil
	}
	return &mayorOut{
		Year: r.Year, Supplementary: r.Supplementary,
		Name: titleCase(r.Name), FullName: titleCase(r.FullName), Party: r.Party, Gender: r.Gender,
	}
}
