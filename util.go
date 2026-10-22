package credicarro

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// requireUUID valida um identificador (UUID) antes de interpolá-lo na rota.
func requireUUID(name, value string) error {
	if !uuidPattern.MatchString(value) {
		return &ConfigurationError{Message: name + " deve ser um UUID válido."}
	}
	return nil
}

func requireNonEmpty(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return &ConfigurationError{Message: name + " é obrigatório."}
	}
	return nil
}

func base64Encode(content []byte) string {
	return base64.StdEncoding.EncodeToString(content)
}

func unmarshalJSON(body []byte, target interface{}) error {
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, target)
}

// rangeQuery monta a query das consultas de faixa (placa e, na v2, cnpjLoja).
func rangeQuery(licensePlate, storeCnpj string, withStore bool) url.Values {
	q := url.Values{}
	q.Set("placa", licensePlate)
	if withStore {
		q.Set("cnpjLoja", storeCnpj)
	}
	return q
}
