package idempotency_test

import (
	"testing"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/idempotency"
)

const id = "bea8d060-f93d-48eb-b6c4-45f64483cdd9"

func TestRequiresKey(t *testing.T) {
	cases := []struct {
		method, path string
		expected     bool
	}{
		{"POST", "v1/customer", true},
		{"POST", "v1/customer/address", true},
		{"POST", "v1/customer/professional", true},
		{"POST", "v1/proposal/simulation/financing", true},
		{"POST", "v1/proposal/simulation/refinancing", true},
		{"POST", "v2/proposal/simulation/financing", true},
		{"POST", "v2/proposal/simulation/refinancing", true},
		{"POST", "v1/proposal", true},
		{"POST", "v2/proposal", true},
		{"POST", "v2/proposal/" + id + "/document", true},
		// a chave NUNCA é automática no unified: com ela a rota só cria
		{"POST", "v1/customer/unified", false},
		{"POST", "v1/webhook", false},
		{"POST", "v1/authentication/accesstoken", false},
		{"POST", "v2/proposal/" + id + "/vehicle-detail", false},
		{"POST", "v2/proposal/" + id + "/deposit-account", false},
		{"GET", "v2/proposal/" + id, false},
		{"GET", "v1/customer", false},
	}

	for _, c := range cases {
		if got := idempotency.RequiresKey(c.method, c.path); got != c.expected {
			t.Errorf("RequiresKey(%q, %q) = %v, esperava %v", c.method, c.path, got, c.expected)
		}
	}
}

func TestIsDocumentUpload(t *testing.T) {
	if !idempotency.IsDocumentUpload("POST", "v2/proposal/"+id+"/document") {
		t.Error("esperava true para o envio de documento")
	}
	if idempotency.IsDocumentUpload("POST", "v2/proposal/"+id+"/vehicle-detail") || idempotency.IsDocumentUpload("GET", "v2/proposal/"+id+"/document") {
		t.Error("esperava false fora do envio de documento")
	}
}

func TestIsRetrySafe(t *testing.T) {
	cases := []struct {
		method, path string
		hasKey       bool
		expected     bool
	}{
		{"GET", "v2/proposal/" + id, false, true},
		{"GET", "v1/proposal?pagina_inicial=1&limite=10", false, true},
		{"HEAD", "v1/proposal", false, true},
		{"POST", "v1/authentication/accesstoken", false, true},
		{"POST", "v1/customer", true, true},
		{"POST", "v1/customer", false, false},
		// unified: só é repetível quando o integrador informou a chave
		{"POST", "v1/customer/unified", true, true},
		{"POST", "v1/customer/unified", false, false},
		{"POST", "v2/proposal/" + id + "/document", true, true},
		// substituição: repetível mesmo sem chave
		{"POST", "v2/proposal/" + id + "/vehicle-detail", false, true},
		{"POST", "v2/proposal/" + id + "/deposit-account", false, true},
		{"POST", "v1/webhook", false, false},
		{"PUT", "v1/webhook/" + id, false, true},
		{"DELETE", "v1/webhook/" + id, false, true},
		{"PUT", "v1/customer", false, false},
		{"DELETE", "v2/proposal/" + id, false, false},
	}

	for _, c := range cases {
		if got := idempotency.IsRetrySafe(c.method, c.path, c.hasKey); got != c.expected {
			t.Errorf("IsRetrySafe(%q, %q, %v) = %v, esperava %v", c.method, c.path, c.hasKey, got, c.expected)
		}
	}
}
