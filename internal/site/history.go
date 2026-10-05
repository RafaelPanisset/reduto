package site

import (
	"cmp"
	"slices"
	"strings"

	"github.com/RafaelPanisset/reduto/internal/tse"
)

// O histórico municipal de um candidato vem dos cadastros de 2020 e 2024. Em 2024 o TSE não
// divulga o CPF, então a mesma pessoa é encontrada por nome completo + data de nascimento.
// Conferido contra o CPF de 2020, esse par bate em 6.085 de 6.086 casos.

type runOut struct {
	Year     int    `json:"year"`
	City     string `json:"city"`
	CityName string `json:"cityName"`
	UF       string `json:"uf"`
	Office   int    `json:"office"`
	Party    string `json:"party"`
	Elected  bool   `json:"elected"`
}

// localOut resume, para os rankings, a candidatura mais forte do candidato na cidade-reduto.
type localOut struct {
	Year    int  `json:"year"`
	Office  int  `json:"office"`
	Elected bool `json:"elected"`
}

type mayorOut struct {
	Year     int    `json:"year"`
	Name     string `json:"name"`
	FullName string `json:"fullName"`
	Party    string `json:"party"`
	Gender   string `json:"gender,omitempty"`
}

type history struct {
	byPerson map[string][]tse.Run
	mayors   map[string]tse.Run // código TSE → prefeito eleito na eleição municipal mais recente
}

func newHistory(runs []tse.Run) history {
	h := history{byPerson: map[string][]tse.Run{}, mayors: map[string]tse.Run{}}
	latest := 0
	for _, r := range runs {
		latest = max(latest, r.Year)
	}
	for _, r := range runs {
		if k := personKey(r.FullName, r.BirthDate); k != "" {
			h.byPerson[k] = append(h.byPerson[k], r)
		}
		if r.Year == latest && r.Office == tse.Mayor && r.Elected {
			h.mayors[r.City] = r
		}
	}
	return h
}

// of devolve as candidaturas municipais da pessoa, da mais antiga para a mais recente.
func (h history) of(p tse.Person) []runOut {
	var out []runOut
	for _, r := range h.byPerson[personKey(p.FullName, p.BirthDate)] {
		out = append(out, runOut{
			Year: r.Year, City: r.City, CityName: titleCase(r.CityName), UF: r.UF,
			Office: r.Office, Party: r.Party, Elected: r.Elected,
		})
	}
	slices.SortFunc(out, func(a, b runOut) int {
		return cmp.Or(cmp.Compare(a.Year, b.Year), cmp.Compare(a.Office, b.Office), cmp.Compare(a.City, b.City))
	})
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
			-compareBool(r.Elected, best.Elected), cmp.Compare(r.Office, best.Office), -cmp.Compare(r.Year, best.Year),
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
	return &mayorOut{Year: r.Year, Name: titleCase(r.Name), FullName: titleCase(r.FullName), Party: r.Party, Gender: r.Gender}
}

// personKey é o nome completo sem acentos e sem espaços repetidos, mais a data de nascimento.
// Sem data de nascimento não há chave: só o nome junta homônimos.
func personKey(fullName, birthDate string) string {
	if birthDate == "" {
		return ""
	}
	return foldName(fullName) + "|" + birthDate
}

// Os nomes vêm do latin1 do TSE, então os acentos possíveis são só estes.
var unaccent = strings.NewReplacer(
	"Á", "A", "À", "A", "Â", "A", "Ã", "A", "Ä", "A", "É", "E", "È", "E", "Ê", "E", "Ë", "E",
	"Í", "I", "Ì", "I", "Î", "I", "Ï", "I", "Ó", "O", "Ò", "O", "Ô", "O", "Õ", "O", "Ö", "O",
	"Ú", "U", "Ù", "U", "Û", "U", "Ü", "U", "Ç", "C", "Ñ", "N", "Ý", "Y",
)

func foldName(s string) string {
	return strings.Join(strings.Fields(unaccent.Replace(strings.ToUpper(s))), " ")
}
