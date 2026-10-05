package tse

import (
	"archive/zip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// writeZip cria um zip com os arquivos dados e devolve o caminho.
func writeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

const voteHeader = `"DT_GERACAO";"HH_GERACAO";"NR_TURNO";"SG_UF";"CD_MUNICIPIO";"NR_ZONA";"CD_CARGO";"SQ_CANDIDATO";"NR_CANDIDATO";"NM_URNA_CANDIDATO";"SG_PARTIDO";"DS_SIT_TOT_TURNO";"QT_VOTOS_NOMINAIS_VALIDOS"` + "\n"

func voteRow(round, uf, city, zone, office, id, name string, votes string) string {
	return `"05/10/2026";"10:14:37";` + round + `;"` + uf + `";` + city + `;` + zone + `;` + office + `;` + id +
		`;1234;"` + name + `";"PT";"ELEITO POR QP";` + votes + "\n"
}

func TestReadVotes(t *testing.T) {
	// "JO\xc3O" é "JOÃO" em latin1.
	se := voteHeader +
		voteRow("1", "SE", "31879", "15", "6", "100", "JO\xc3O", "10") +
		voteRow("1", "SE", "31879", "16", "6", "100", "JO\xc3O", "5") + // outra zona da mesma cidade
		voteRow("1", "SE", "1007", "1", "6", "100", "JO\xc3O", "2") + // código com 4 dígitos
		voteRow("1", "SE", "31879", "15", "7", "200", "MARIA", "7") +
		voteRow("1", "SE", "31879", "15", "3", "300", "GOVERNADOR", "99") + // cargo fora do site
		voteRow("2", "SE", "31879", "15", "6", "100", "JO\xc3O", "50") // 2º turno: ignorado
	brasil := voteHeader + voteRow("1", "SE", "31879", "15", "6", "100", "JO\xc3O", "1000")
	path := writeZip(t, map[string]string{
		"votacao_candidato_munzona_2026_SE.csv":     se,
		"votacao_candidato_munzona_2026_BRASIL.csv": brasil,
		"votacao_candidato_munzona_2026_BR.csv":     brasil,
		"leiame.pdf":                                "x",
	})

	v, err := ReadVotes(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Candidates) != 2 {
		t.Fatalf("candidatos = %d, quero 2 (só deputados): %+v", len(v.Candidates), v.Candidates)
	}
	if got := v.ByCity["100"]["31879"]; got != 15 {
		t.Errorf("votos de 100 em 31879 = %d, quero 15 (soma das zonas, sem 2º turno nem BRASIL)", got)
	}
	if got := v.ByCity["100"]["01007"]; got != 2 {
		t.Errorf("votos de 100 em 01007 = %d, quero 2 (código com zero à esquerda)", got)
	}
	c := v.Candidates["100"]
	if c.Name != "JOÃO" || c.Office != FederalDeputy || c.UF != "SE" || c.Status != "ELEITO POR QP" {
		t.Errorf("candidato 100 = %+v", c)
	}
	if v.Candidates["200"].Office != StateDeputy {
		t.Errorf("candidato 200 = %+v", v.Candidates["200"])
	}
	// 10:14:37 no horário de Brasília.
	if want := time.Date(2026, 10, 5, 13, 14, 37, 0, time.UTC); !v.GeneratedAt.Equal(want) {
		t.Errorf("GeneratedAt = %v, quero %v", v.GeneratedAt, want)
	}
}

func TestReadVotesMissingColumn(t *testing.T) {
	csv := strings.Replace(voteHeader, `"QT_VOTOS_NOMINAIS_VALIDOS"`, `"OUTRA"`, 1)
	path := writeZip(t, map[string]string{"votacao_candidato_munzona_2026_SE.csv": csv})
	if _, err := ReadVotes(path); err == nil || !strings.Contains(err.Error(), "QT_VOTOS_NOMINAIS_VALIDOS") {
		t.Fatalf("err = %v, quero erro citando a coluna ausente", err)
	}
}

func TestReadPeople(t *testing.T) {
	csv := `"SQ_CANDIDATO";"DS_GENERO";"NM_CANDIDATO";"DT_NASCIMENTO"` + "\n" +
		`1;"FEMININO";"MARIA DA CONCEI` + "\xc7\xc3" + `O";"01/02/1970"` + "\n" +
		`2;"MASCULINO";"JOSE";"03/04/1980"` + "\n" +
		`3;"N` + "\xc3O DIVULG\xc1VEL" + `";"ANA";""` + "\n"
	path := writeZip(t, map[string]string{
		"consulta_cand_2026_BRASIL.csv": csv,
		"consulta_cand_2026_SE.csv":     "lixo",
	})
	p, err := ReadPeople(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Person{
		"1": {Gender: Female, FullName: "MARIA DA CONCEIÇÃO", BirthDate: "01/02/1970"},
		"2": {Gender: Male, FullName: "JOSE", BirthDate: "03/04/1980"},
		"3": {Gender: Unknown, FullName: "ANA"},
	}
	if len(p) != len(want) {
		t.Fatalf("pessoas = %v", p)
	}
	for id, w := range want {
		if p[id] != w {
			t.Errorf("pessoa %s = %+v, quero %+v", id, p[id], w)
		}
	}
}

const municipalHeader = `"SQ_CANDIDATO";"NR_TURNO";"SG_UF";"SG_UE";"NM_UE";"CD_CARGO";"SG_PARTIDO";"DS_SIT_TOT_TURNO";"NM_URNA_CANDIDATO";"NM_CANDIDATO";"DT_NASCIMENTO";"DS_GENERO";"DT_ELEICAO";"CD_TIPO_ELEICAO"` + "\n"

func municipalRow(id, round, city, office, status string) string {
	return municipalRowOn(id, round, city, office, status, "06/10/2024", "2")
}

func municipalRowOn(id, round, city, office, status, date, kind string) string {
	return id + ";" + round + `;"SP";"` + city + `";"S` + "\xc3" + `O PAULO";` + office + `;"PL";"` + status +
		`";"URNA";"NOME";"01/01/1980";"MASCULINO";"` + date + `";` + kind + "\n"
}

func TestReadMunicipal(t *testing.T) {
	csv := municipalHeader +
		municipalRow("1", "2", "71072", "11", "ELEITO") + // 2º turno antes do 1º no arquivo
		municipalRow("1", "1", "71072", "11", "2\xba TURNO") +
		municipalRow("2", "1", "1007", "13", "ELEITO POR QP") +
		municipalRow("3", "1", "71072", "13", "SUPLENTE") +
		municipalRow("4", "1", "71072", "13", "#NULO#") + // candidatura que não valeu
		municipalRow("5", "1", "71072", "12", "N\xc3O ELEITO") +
		municipalRowOn("6", "1", "71072", "11", "ELEITO", "07/06/2026", "1") // suplementar de 2026 no arquivo de 2024
	path := writeZip(t, map[string]string{"consulta_cand_2024_BRASIL.csv": csv})
	runs, err := ReadMunicipal(path, func(r Run) bool { return r.ID != "5" })
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, r := range runs {
		ids = append(ids, r.ID)
	}
	if !slices.Equal(ids, []string{"1", "2", "3", "6"}) {
		t.Fatalf("IDs = %v, quero 1 2 3 6 em ordem (sem a indeferida 4 e sem a 5, que o keep recusou)", ids)
	}
	if r := runs[3]; r.Year != 2026 || !r.Supplementary || !r.Date.Equal(time.Date(2026, 6, 7, 3, 0, 0, 0, time.UTC)) {
		t.Errorf("suplementar = %+v, quero ano 2026 (o da eleição, não o do arquivo)", r)
	}
	got := map[string]Run{}
	for _, r := range runs {
		got[r.Status+"/"+strconv.Itoa(r.Office)+"/"+strconv.Itoa(r.Year)] = r
	}
	mayor := got["ELEITO/11/2024"]
	want := Run{ID: "1", Date: time.Date(2024, 10, 6, 3, 0, 0, 0, time.UTC), Year: 2024,
		City: "71072", CityName: "SÃO PAULO", UF: "SP", Office: Mayor, Party: "PL", Elected: true,
		Status: "ELEITO", Name: "URNA", FullName: "NOME", BirthDate: "01/01/1980", Gender: Male}
	mayor.Date = mayor.Date.UTC()
	if mayor != want {
		t.Errorf("prefeito = %+v\nquero    %+v", mayor, want)
	}
	if r := got["ELEITO POR QP/13/2024"]; !r.Elected || r.City != "01007" {
		t.Errorf("vereador eleito = %+v", r)
	}
	if r := got["SUPLENTE/13/2024"]; r.Elected {
		t.Errorf("suplente = %+v", r)
	}
}

func TestPersonKey(t *testing.T) {
	if a, b := PersonKey("José  da Conceição", "01/01/1980"), PersonKey("JOSE DA CONCEICAO", "01/01/1980"); a != b {
		t.Errorf("chaves diferentes para a mesma pessoa: %q, %q", a, b)
	}
	if k := PersonKey("JOSE", ""); k != "" {
		t.Errorf("sem data de nascimento a chave tem que ser vazia, veio %q", k)
	}
}

func TestReadCities(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mun.json")
	doc := `{"abr":[{"cd":"se","ds":"SERGIPE","mu":[{"cd":"31879","cdi":"2804409","nm":"NEÓPOLIS","z":["15"]}]},` +
		`{"cd":"zz","ds":"EXTERIOR","mu":[{"cd":"29050","cdi":"","nm":"LISBOA"}]}]}`
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := ReadCities(path)
	if err != nil {
		t.Fatal(err)
	}
	want := City{Code: "31879", IBGE: "2804409", Name: "NEÓPOLIS", UF: "SE"}
	if len(c) != 1 || c["31879"] != want {
		t.Errorf("municípios = %+v, quero só %+v (sem exterior)", c, want)
	}
}
