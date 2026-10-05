// Comando reduto gera os dados do site a partir dos arquivos do TSE e do IBGE baixados por
// scripts/fetch.sh.
package main

import (
	"flag"
	"log"
	"strings"
	"time"

	"github.com/RafaelPanisset/reduto/internal/site"
	"github.com/RafaelPanisset/reduto/internal/tse"
)

func main() {
	votes := flag.String("votos", "raw/votacao.zip", "zip votacao_candidato_munzona do TSE")
	cands := flag.String("candidatos", "raw/candidatos.zip", "zip consulta_cand do TSE da eleição dos votos")
	municipal := flag.String("municipais", "raw/candidatos_2024.zip,raw/candidatos_2020.zip", "zips consulta_cand das eleições municipais, separados por vírgula")
	cities := flag.String("municipios", "raw/municipios.json", "cadastro de municípios da apuração (mun-e*-cm.json)")
	geo := flag.String("geo", "raw/geo", "diretório com a malha do IBGE de cada UF (<UF>.json)")
	out := flag.String("out", "site/data", "onde gravar os dados do site")
	flag.Parse()
	log.SetFlags(0)

	start := time.Now()
	v, err := tse.ReadVotes(*votes)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("votos: %d candidatos a deputado (%s)", len(v.Candidates), time.Since(start).Round(time.Millisecond))

	people, err := tse.ReadPeople(*cands)
	if err != nil {
		log.Fatal(err)
	}
	// Das eleições municipais só interessam as candidaturas de quem disputou em 2026 e os
	// prefeitos eleitos. Descartar o resto já na leitura poupa memória (são ~1 milhão).
	keys := map[string]bool{}
	for _, p := range people {
		if k := tse.PersonKey(p.FullName, p.BirthDate); k != "" {
			keys[k] = true
		}
	}
	keep := func(r tse.Run) bool {
		return (r.Office == tse.Mayor && r.Elected) || keys[tse.PersonKey(r.FullName, r.BirthDate)]
	}
	var runs []tse.Run
	for _, path := range strings.Split(*municipal, ",") {
		r, err := tse.ReadMunicipal(path, keep)
		if err != nil {
			log.Fatal(err)
		}
		runs = append(runs, r...)
	}
	log.Printf("eleições municipais: %d candidaturas guardadas (%s)", len(runs), time.Since(start).Round(time.Millisecond))
	c, err := tse.ReadCities(*cities)
	if err != nil {
		log.Fatal(err)
	}
	rep, err := site.Build(site.Input{Votes: v, People: people, Runs: runs, Cities: c, GeoDir: *geo}, *out)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s: %d candidatos com voto (%d com histórico municipal), %d municípios (%s no total)",
		*out, rep.Candidates, rep.WithHistory, len(c), time.Since(start).Round(time.Millisecond))
}
