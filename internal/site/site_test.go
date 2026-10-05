package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RafaelPanisset/reduto/internal/tse"
)

func TestSummarize(t *testing.T) {
	got := summarize(map[string]int{"20000": 7, "10000": 7, "30000": 1, "40000": 0})
	want := summary{Total: 15, Cities: 3, Top: "10000", TopVotes: 7}
	if got != want {
		t.Errorf("summarize = %+v, quero %+v (empate fica com o menor código; cidade com 0 não conta)", got, want)
	}
}

func TestTitleCase(t *testing.T) {
	for in, want := range map[string]string{
		"CAMPOS DOS GOYTACAZES":  "Campos dos Goytacazes",
		"PINGO-D'ÁGUA":           "Pingo-d'Água",
		"MIRASSOL D'OESTE":       "Mirassol d'Oeste",
		"SANT'ANA DO LIVRAMENTO": "Sant'Ana do Livramento",
		"SÃO JOÃO DEL REI":       "São João Del Rei",
		"PEDRO II":               "Pedro II",
		"MARIA E SILVA":          "Maria e Silva",
		"DR. ÍTALO":              "Dr. Ítalo",
		"SARGENTO MELLO-CASAL":   "Sargento Mello-Casal",
		"DA SILVA":               "Da Silva",
	} {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, quero %q", in, got, want)
		}
	}
}

// fixture: SE com duas cidades; Ana (federal) concentrada em A, Bia (federal) espalhada,
// Caio (estadual) e Davi (federal, sem votos).
func fixture(t *testing.T) (Input, string) {
	t.Helper()
	geo := t.TempDir()
	topo := `{"type":"Topology","objects":{"UF28MU":{"type":"GeometryCollection","geometries":[` +
		`{"type":"Polygon","arcs":[[0]],"properties":{"codarea":"2800001"}},` +
		`{"type":"Polygon","arcs":[[1]],"properties":{"codarea":"2800002"}}]}},"arcs":[]}`
	if err := os.WriteFile(filepath.Join(geo, "SE.json"), []byte(topo), 0o644); err != nil {
		t.Fatal(err)
	}
	return Input{
		Votes: &tse.Votes{
			Candidates: map[string]tse.Candidate{
				"1": {ID: "1", UF: "SE", Office: tse.FederalDeputy, Number: "1301", Name: "ANA DA SILVA", Party: "PT", Status: "ELEITO POR QP"},
				"2": {ID: "2", UF: "SE", Office: tse.FederalDeputy, Number: "2201", Name: "BIA", Party: "PL", Status: "SUPLENTE"},
				"3": {ID: "3", UF: "SE", Office: tse.StateDeputy, Number: "13001", Name: "CAIO", Party: "PT", Status: "ELEITO POR MÉDIA"},
				"4": {ID: "4", UF: "SE", Office: tse.FederalDeputy, Number: "2202", Name: "DAVI", Party: "PL", Status: "NÃO ELEITO"},
			},
			ByCity: map[string]map[string]int{
				"1": {"00001": 90, "00002": 10},
				"2": {"00001": 30, "00002": 30},
				"3": {"00002": 5},
				"4": {"00001": 0},
			},
			GeneratedAt: time.Date(2026, 10, 5, 10, 14, 37, 0, time.UTC),
		},
		People: map[string]tse.Person{
			"1": {Gender: tse.Female, FullName: "ANA DA SILVA", BirthDate: "01/01/1980"},
			"2": {Gender: tse.Female, FullName: "BIA", BirthDate: "02/02/1990"},
			"3": {Gender: tse.Male, FullName: "CAIO", BirthDate: ""}, // sem data: não dá para cruzar
		},
		Runs: []tse.Run{
			// Ana: acento e espaço a mais no nome não podem atrapalhar.
			{Year: 2020, City: "00001", CityName: "CIDADE A", UF: "SE", Office: tse.Councilor, Party: "PT", Elected: true, Status: "ELEITO POR QP", FullName: "ÁNA  DA SILVA", BirthDate: "01/01/1980"},
			{Year: 2024, City: "00001", CityName: "CIDADE A", UF: "SE", Office: tse.Mayor, Party: "PT", Status: "NÃO ELEITO", FullName: "ANA DA SILVA", BirthDate: "01/01/1980"},
			// Homônima com outra data de nascimento: não é a Ana.
			{Year: 2024, City: "00002", CityName: "CIDADE B", UF: "SE", Office: tse.Mayor, Party: "PL", Elected: true, Status: "ELEITO", FullName: "ANA DA SILVA", BirthDate: "09/09/1999"},
			{Year: 2024, City: "00001", CityName: "CIDADE A", UF: "SE", Office: tse.Mayor, Party: "PT", Elected: true, Status: "ELEITO", Name: "ZÉ", FullName: "JOSÉ PREFEITO FILHO", BirthDate: "03/03/1970", Gender: tse.Male},
			// Prefeito de uma eleição mais antiga não conta como prefeito atual.
			{Year: 2020, City: "00003", CityName: "CIDADE C", UF: "SE", Office: tse.Mayor, Party: "PL", Elected: true, Status: "ELEITO", Name: "VELHO", FullName: "VELHO", BirthDate: "04/04/1950"},
			{Year: 2020, City: "00001", CityName: "CIDADE A", UF: "SE", Office: tse.Mayor, Party: "PL", Elected: true, Status: "ELEITO", Name: "ANTIGO", FullName: "ANTIGO", BirthDate: "05/05/1955"},
		},
		Cities: map[string]tse.City{
			"00001": {Code: "00001", IBGE: "2800001", Name: "CIDADE A", UF: "SE"},
			"00002": {Code: "00002", IBGE: "2800002", Name: "CIDADE B", UF: "SE"},
		},
		GeoDir: geo,
	}, filepath.Join(t.TempDir(), "data")
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func TestBuild(t *testing.T) {
	in, out := fixture(t)
	// Um resto de build anterior tem que sumir.
	if err := os.MkdirAll(filepath.Join(out, "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "c", "velho.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Build(in, out)
	if err != nil {
		t.Fatal(err)
	}
	if rep != (Report{Candidates: 3, WithHistory: 1}) {
		t.Errorf("relatório = %+v, quero 3 candidatos (Davi não teve votos), 1 com histórico (Ana)", rep)
	}

	var idx indexOut
	readJSON(t, filepath.Join(out, "candidates.json"), &idx)
	if len(idx.Candidates) != 3 || idx.Candidates[0].ID != "1" || idx.Candidates[2].ID != "3" {
		t.Fatalf("candidatos fora da ordem de votos: %+v", idx.Candidates)
	}
	want := candidateOut{
		ID: "1", Name: "Ana da Silva", Gender: "F", Party: "PT", UF: "SE", Office: 6, Number: "1301",
		Elected: true, Status: "ELEITO POR QP", Votes: 100, Cities: 2,
		Top: "00001", TopName: "Cidade A", TopVotes: 90, TopTotal: 120, // 90 da Ana + 30 da Bia; Caio é de outro cargo
		MayorParty: true, // o prefeito eleito em 2024 na Cidade A é do PT, como ela
	}
	if l := idx.Candidates[0].Local; l == nil || *l != (localOut{Year: 2020, Office: tse.Councilor, Elected: true}) {
		t.Errorf("Ana.Local = %+v, quero vereadora eleita em 2020 (eleita vence prefeita não eleita)", l)
	}
	idx.Candidates[0].Local = nil
	if idx.Candidates[0] != want {
		t.Errorf("Ana = %+v\nquero  %+v", idx.Candidates[0], want)
	}
	if idx.Totals["SE"]["6"] != 160 || idx.Totals["SE"]["7"] != 5 || len(idx.Totals) != 1 {
		t.Errorf("totais por UF = %v", idx.Totals)
	}
	if !idx.Generated.Equal(in.Votes.GeneratedAt) {
		t.Errorf("generated = %v", idx.Generated)
	}

	var ana struct {
		Votes   [][2]any
		History []runOut
	}
	readJSON(t, filepath.Join(out, "c", "1.json"), &ana)
	if len(ana.Votes) != 2 || ana.Votes[0][0] != "00001" || ana.Votes[0][1] != 90.0 {
		t.Errorf("votos da Ana = %v, quero a cidade A primeiro", ana.Votes)
	}
	wantHist := []runOut{
		{Year: 2020, City: "00001", CityName: "Cidade A", UF: "SE", Office: tse.Councilor, Party: "PT", Elected: true},
		{Year: 2024, City: "00001", CityName: "Cidade A", UF: "SE", Office: tse.Mayor, Party: "PT"},
	}
	if !slices.Equal(ana.History, wantHist) {
		t.Errorf("histórico da Ana = %+v\nquero %+v", ana.History, wantHist)
	}

	var uf ufOut
	readJSON(t, filepath.Join(out, "uf", "SE.json"), &uf)
	if uf.Totals["6"] != 160 || uf.Totals["7"] != 5 || len(uf.Cities) != 2 || uf.Cities[1].Totals["6"] != 40 {
		t.Errorf("UF = %+v", uf)
	}
	wantMayor := mayorOut{Year: 2024, Name: "Zé", FullName: "José Prefeito Filho", Party: "PT", Gender: tse.Male}
	if m := uf.Cities[0].Mayor; m == nil || *m != wantMayor {
		t.Errorf("prefeito da Cidade A = %+v, quero %+v (o de 2024, não o de 2020)", m, wantMayor)
	}
	if m := uf.Cities[1].Mayor; m == nil || m.Party != "PL" {
		t.Errorf("prefeito da Cidade B = %+v", m)
	}
	if _, err := os.Stat(filepath.Join(out, "geo", "SE.json")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(out, "c", "velho.json")); !os.IsNotExist(err) {
		t.Errorf("arquivo do build anterior continua lá (err = %v)", err)
	}
	if _, err := os.Stat(out + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("diretório temporário ficou para trás (err = %v)", err)
	}
}

func TestBuildMissingPolygon(t *testing.T) {
	in, out := fixture(t)
	in.Cities["00002"] = tse.City{Code: "00002", IBGE: "2899999", Name: "CIDADE B", UF: "SE"}
	_, err := Build(in, out)
	if err == nil || !strings.Contains(err.Error(), "2899999") {
		t.Fatalf("err = %v, quero erro citando o município sem polígono", err)
	}
	for _, dir := range []string{out, out + ".tmp"} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("build com erro deixou %s (err = %v)", dir, err)
		}
	}
}

func TestBuildUnknownCity(t *testing.T) {
	in, out := fixture(t)
	in.Votes.ByCity["2"]["99999"] = 1
	if _, err := Build(in, out); err == nil || !strings.Contains(err.Error(), "99999") {
		t.Fatalf("err = %v, quero erro citando o município desconhecido", err)
	}
}

func TestLocal(t *testing.T) {
	runs := []runOut{
		{Year: 2024, City: "1", Office: tse.Mayor},                    // perdeu para prefeito
		{Year: 2020, City: "1", Office: tse.Councilor, Elected: true}, // vereador eleito
		{Year: 2024, City: "1", Office: tse.Councilor, Elected: true}, // reeleito
		{Year: 2024, City: "2", Office: tse.Mayor, Elected: true},     // outra cidade
	}
	if got := local(runs, "1"); got == nil || *got != (localOut{Year: 2024, Office: tse.Councilor, Elected: true}) {
		t.Errorf("local = %+v, quero vereador eleito em 2024", got)
	}
	if got := local(runs[:1], "1"); got == nil || *got != (localOut{Year: 2024, Office: tse.Mayor}) {
		t.Errorf("local = %+v, quero a candidatura derrotada quando é a única", got)
	}
	if got := local(runs, "3"); got != nil {
		t.Errorf("local = %+v, quero nil para cidade sem candidatura", got)
	}
}

func TestPersonKey(t *testing.T) {
	if a, b := personKey("José  da Conceição", "01/01/1980"), personKey("JOSE DA CONCEICAO", "01/01/1980"); a != b {
		t.Errorf("chaves diferentes para a mesma pessoa: %q, %q", a, b)
	}
	if k := personKey("JOSE", ""); k != "" {
		t.Errorf("sem data de nascimento a chave tem que ser vazia, veio %q", k)
	}
}
