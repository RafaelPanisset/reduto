// Package tse lê os arquivos de dados abertos do TSE usados pelo site.
package tse

import (
	"archive/zip"
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Cargos de deputado, os únicos que o site usa. Governador e senador disputam o estado
// inteiro e presidente o país inteiro: para eles, "reduto" diz pouco.
const (
	FederalDeputy  = 6
	StateDeputy    = 7
	DistrictDeputy = 8
)

type Candidate struct {
	ID     string // SQ_CANDIDATO: único na eleição
	UF     string
	Office int // CD_CARGO
	Number string
	Name   string // nome de urna
	Party  string
	Status string // DS_SIT_TOT_TURNO: "ELEITO POR QP", "SUPLENTE", "NÃO ELEITO"...
}

// Votes guarda os votos nominais válidos do 1º turno de cada candidato a deputado, por
// município. O TSE publica uma linha por zona eleitoral; aqui elas já vêm somadas.
type Votes struct {
	Candidates  map[string]Candidate
	ByCity      map[string]map[string]int // ID do candidato → código TSE do município → votos
	GeneratedAt time.Time                 // quando o TSE gerou o arquivo (DT_GERACAO/HH_GERACAO)
}

// O zip traz um CSV por UF, mais _BRASIL.csv (as linhas de todas as UFs juntas) e _BR.csv
// (presidente). Lemos só os das UFs: com o BRASIL junto, tudo seria contado em dobro.
var ufFile = regexp.MustCompile(`^votacao_candidato_munzona_\d{4}_([A-Z]{2})\.csv$`)

// ReadVotes lê o zip votacao_candidato_munzona_<ano>.zip.
func ReadVotes(zipPath string) (*Votes, error) {
	z, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer z.Close()

	var files []*zip.File
	for _, f := range z.File {
		if m := ufFile.FindStringSubmatch(f.Name); m != nil && m[1] != "BR" {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s: nenhum CSV por UF", zipPath)
	}

	// Uma goroutine por UF. O trabalho é descompactar e parsear ~3 GB de CSV, e as UFs não
	// compartilham candidatos nem municípios: juntar os resultados é só copiar mapas.
	out := &Votes{Candidates: map[string]Candidate{}, ByCity: map[string]map[string]int{}}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs []error
	)
	sem := make(chan struct{}, runtime.NumCPU())
	for _, f := range files {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			part, err := readVotesFile(f)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", f.Name, err))
				return
			}
			maps.Copy(out.Candidates, part.Candidates)
			maps.Copy(out.ByCity, part.ByCity)
			if part.GeneratedAt.After(out.GeneratedAt) {
				out.GeneratedAt = part.GeneratedAt
			}
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return out, nil
}

const (
	colGenDate = iota
	colGenTime
	colRound
	colUF
	colCity
	colOffice
	colID
	colNumber
	colName
	colParty
	colStatus
	colVotes
)

var voteColumns = []string{
	"DT_GERACAO", "HH_GERACAO", "NR_TURNO", "SG_UF", "CD_MUNICIPIO", "CD_CARGO", "SQ_CANDIDATO",
	"NR_CANDIDATO", "NM_URNA_CANDIDATO", "SG_PARTIDO", "DS_SIT_TOT_TURNO", "QT_VOTOS_NOMINAIS_VALIDOS",
}

func readVotesFile(f *zip.File) (*Votes, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	r, col, err := openCSV(rc, voteColumns)
	if err != nil {
		return nil, err
	}

	out := &Votes{Candidates: map[string]Candidate{}, ByCity: map[string]map[string]int{}}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		get := func(c int) string { return rec[col[c]] }
		if get(colRound) != "1" {
			continue
		}
		office, err := strconv.Atoi(get(colOffice))
		if err != nil {
			return nil, fmt.Errorf("CD_CARGO %q: %w", get(colOffice), err)
		}
		if office != FederalDeputy && office != StateDeputy && office != DistrictDeputy {
			continue
		}
		votes, err := strconv.Atoi(get(colVotes))
		if err != nil {
			return nil, fmt.Errorf("QT_VOTOS_NOMINAIS_VALIDOS %q: %w", get(colVotes), err)
		}

		id := get(colID)
		if _, ok := out.Candidates[id]; !ok {
			out.Candidates[id] = Candidate{
				ID: id, UF: get(colUF), Office: office, Number: get(colNumber),
				Name: latin1(get(colName)), Party: latin1(get(colParty)), Status: latin1(get(colStatus)),
			}
			if t, err := time.ParseInLocation("02/01/2006 15:04:05", get(colGenDate)+" "+get(colGenTime), brasilia); err == nil && t.After(out.GeneratedAt) {
				out.GeneratedAt = t
			}
		}
		byCity := out.ByCity[id]
		if byCity == nil {
			byCity = map[string]int{}
			out.ByCity[id] = byCity
		}
		byCity[cityCode(get(colCity))] += votes
	}
}

// Os horários do TSE são de Brasília. Desde 2019 não há horário de verão, então o fuso é fixo.
var brasilia = time.FixedZone("BRT", -3*60*60)

// openCSV prepara a leitura de um CSV do TSE (separado por ";") e devolve, para cada nome em
// columns, a posição da coluna no cabeçalho. Procurar pelo nome, e não pela posição, faz o
// código continuar certo se o TSE mudar a ordem das colunas de um ano para outro.
func openCSV(r io.Reader, columns []string) (*csv.Reader, []int, error) {
	cr := csv.NewReader(bufio.NewReaderSize(r, 1<<20))
	cr.Comma = ';'
	cr.ReuseRecord = true
	header, err := cr.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("cabeçalho: %w", err)
	}
	pos := map[string]int{}
	for i, h := range header {
		pos[h] = i
	}
	idx := make([]int, len(columns))
	for i, c := range columns {
		j, ok := pos[c]
		if !ok {
			return nil, nil, fmt.Errorf("coluna %s ausente", c)
		}
		idx[i] = j
	}
	return cr, idx, nil
}

// latin1 converte um campo em ISO-8859-1 (o encoding dos CSVs do TSE) para UTF-8. Em latin1
// cada byte é o próprio code point, então basta reescrever byte a byte. O encoding/csv não
// valida UTF-8, então os campos chegam aqui com os bytes originais.
func latin1(s string) string {
	b := make([]byte, 0, len(s)+len(s)/4)
	for i := 0; i < len(s); i++ {
		b = utf8.AppendRune(b, rune(s[i]))
	}
	return string(b)
}

// cityCode normaliza o código TSE do município para 5 dígitos: o CSV escreve 1007, o
// cadastro de municípios da apuração escreve 01007.
func cityCode(s string) string {
	if len(s) >= 5 {
		return s
	}
	return strings.Repeat("0", 5-len(s)) + s
}
