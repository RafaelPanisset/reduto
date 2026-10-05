package tse

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type City struct {
	Code string // código TSE, 5 dígitos
	IBGE string // código IBGE, 7 dígitos: é o que aparece nas malhas do IBGE
	Name string // como o TSE escreve: em maiúsculas
	UF   string
}

// ReadCities lê o cadastro de municípios da apuração (mun-e<eleição>-cm.json). É a única
// fonte do TSE que traz o código IBGE ao lado do código TSE, e sem ele não há como pôr os
// votos no mapa.
func ReadCities(path string) (map[string]City, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Abr []struct {
			UF string `json:"cd"`
			Mu []struct {
				Code string `json:"cd"`
				IBGE string `json:"cdi"`
				Name string `json:"nm"`
			} `json:"mu"`
		} `json:"abr"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := map[string]City{}
	for _, a := range doc.Abr {
		uf := strings.ToUpper(a.UF)
		if uf == "ZZ" { // exterior: não tem município no mapa nem deputado
			continue
		}
		for _, m := range a.Mu {
			out[m.Code] = City{Code: m.Code, IBGE: m.IBGE, Name: m.Name, UF: uf}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: nenhum município", path)
	}
	return out, nil
}
