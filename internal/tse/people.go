package tse

import (
	"archive/zip"
	"cmp"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Gênero, como o site guarda.
const (
	Female  = "F"
	Male    = "M"
	Unknown = "" // "NÃO DIVULGÁVEL"
)

func gender(ds string) string {
	switch ds {
	case "FEMININO":
		return Female
	case "MASCULINO":
		return Male
	}
	return Unknown
}

// Person é o que o site usa do cadastro de candidatos de 2026.
type Person struct {
	Gender    string // para concordar as palavras: "eleita", "deputada"
	FullName  string
	BirthDate string // dd/mm/aaaa, como o TSE escreve; vazio quando não divulgado
}

// ReadPeople lê o cadastro de candidatos (consulta_cand_<ano>.zip): ID → Person.
func ReadPeople(zipPath string) (map[string]Person, error) {
	out := map[string]Person{}
	_, err := eachRow(zipPath, []string{"SQ_CANDIDATO", "DS_GENERO", "NM_CANDIDATO", "DT_NASCIMENTO"}, func(get func(int) string) error {
		out[strings.Clone(get(0))] = Person{Gender: gender(latin1(get(1))), FullName: latin1(get(2)), BirthDate: strings.Clone(get(3))}
		return nil
	})
	return out, err
}

// Cargos das eleições municipais.
const (
	Mayor     = 11
	ViceMayor = 12
	Councilor = 13
)

// Run é uma candidatura numa eleição municipal.
type Run struct {
	ID            string    // SQ_CANDIDATO
	Date          time.Time // dia da eleição
	Year          int       // ano da eleição: o de Date, não o do arquivo
	Supplementary bool      // eleição suplementar, feita depois que a ordinária foi anulada
	City          string    // código TSE, 5 dígitos
	CityName      string    // em maiúsculas, como o TSE escreve
	UF            string
	Office        int
	Party         string
	Elected       bool
	Status        string // situação final (a do 2º turno, quando houve): "ELEITO", "SUPLENTE", "NÃO ELEITO"
	Name          string // nome de urna
	FullName      string
	BirthDate     string
	Gender        string
}

var registryFile = regexp.MustCompile(`^consulta_cand_(\d{4})_BRASIL\.csv$`)

// ReadMunicipal lê o cadastro de candidatos de um ciclo de eleições municipais
// (consulta_cand_2024.zip, por exemplo). O cadastro traz uma linha por turno, então quem foi ao
// 2º turno aparece duas vezes; fica a linha do último turno, que tem a situação final.
// Candidaturas que não chegaram ao fim (indeferidas, renúncias) ficam de fora: só entram
// eleitos, suplentes e não eleitos. O arquivo de um ciclo também traz as eleições suplementares
// feitas nos anos seguintes, com a data delas.
//
// keep escolhe o que guardar (nil guarda tudo): são ~500 mil candidaturas por ciclo, e o site
// só usa as dos candidatos de 2026 e os prefeitos eleitos.
func ReadMunicipal(zipPath string, keep func(Run) bool) ([]Run, error) {
	type row struct {
		round int
		run   Run
	}
	last := map[string]row{}
	_, err := eachRow(zipPath, []string{
		"SQ_CANDIDATO", "NR_TURNO", "SG_UF", "SG_UE", "NM_UE", "CD_CARGO", "SG_PARTIDO", "DS_SIT_TOT_TURNO",
		"NM_URNA_CANDIDATO", "NM_CANDIDATO", "DT_NASCIMENTO", "DS_GENERO", "DT_ELEICAO", "CD_TIPO_ELEICAO",
	}, func(get func(int) string) error {
		office, err := strconv.Atoi(get(5))
		if err != nil {
			return fmt.Errorf("CD_CARGO %q: %w", get(5), err)
		}
		if office != Mayor && office != ViceMayor && office != Councilor {
			return nil
		}
		round, err := strconv.Atoi(get(1))
		if err != nil {
			return fmt.Errorf("NR_TURNO %q: %w", get(1), err)
		}
		if prev, ok := last[get(0)]; ok && prev.round >= round {
			return nil
		}
		date, err := time.ParseInLocation("02/01/2006", get(12), brasilia)
		if err != nil {
			return fmt.Errorf("DT_ELEICAO %q: %w", get(12), err)
		}
		status := latin1(get(7))
		// strings.Clone: com ReuseRecord, cada campo é um pedaço da linha inteira do CSV, e
		// guardar o campo prenderia a linha toda na memória.
		id := strings.Clone(get(0))
		last[id] = row{round, Run{
			ID: id, Date: date, Year: date.Year(), Supplementary: get(13) == "1",
			City: strings.Clone(cityCode(get(3))), CityName: latin1(get(4)), UF: strings.Clone(get(2)),
			Office: office, Party: latin1(get(6)), Elected: strings.HasPrefix(status, "ELEITO"), Status: status,
			Name: latin1(get(8)), FullName: latin1(get(9)), BirthDate: strings.Clone(get(10)), Gender: gender(latin1(get(11))),
		}}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var out []Run
	for _, r := range last {
		valid := r.run.Elected || r.run.Status == "SUPLENTE" || r.run.Status == "NÃO ELEITO"
		if valid && (keep == nil || keep(r.run)) {
			out = append(out, r.run)
		}
	}
	// Ordem fixa: o mapa acima não tem ordem, e quem usa o resultado não deve depender da sorte.
	slices.SortFunc(out, func(a, b Run) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

// PersonKey identifica a mesma pessoa em cadastros de eleições diferentes: o nome completo sem
// acentos e sem espaços repetidos, mais a data de nascimento. Em 2024 o TSE não divulga o CPF;
// conferido contra o CPF onde ele existe (2026 × 2020), o par bate em 6.085 de 6.086 casos.
// Sem data de nascimento não há chave: só o nome juntaria homônimos.
func PersonKey(fullName, birthDate string) string {
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

// eachRow chama fn para cada linha do consulta_cand_<ano>_BRASIL.csv do zip, com get(i)
// devolvendo a coluna columns[i]. Devolve o ano tirado do nome do arquivo.
func eachRow(zipPath string, columns []string, fn func(get func(int) string) error) (int, error) {
	z, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, err
	}
	defer z.Close()
	var (
		f    *zip.File
		year int
	)
	for _, zf := range z.File {
		if m := registryFile.FindStringSubmatch(zf.Name); m != nil {
			f = zf
			year, _ = strconv.Atoi(m[1])
		}
	}
	if f == nil {
		return 0, fmt.Errorf("%s: sem o CSV consulta_cand_<ano>_BRASIL.csv", zipPath)
	}
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	r, col, err := openCSV(rc, columns)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", f.Name, err)
	}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return year, nil
		}
		if err != nil {
			return 0, fmt.Errorf("%s: %w", f.Name, err)
		}
		if err := fn(func(i int) string { return rec[col[i]] }); err != nil {
			return 0, fmt.Errorf("%s: %w", f.Name, err)
		}
	}
}
