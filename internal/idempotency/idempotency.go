// Package idempotency decide quais rotas recebem Idempotency-Key automática e quais
// requisições podem ser repetidas com segurança pelo pipeline de resiliência.
package idempotency

import (
	"strings"
)

// autoKeyPaths são as rotas POST que recebem uma Idempotency-Key automática (uma por
// chamada lógica, a mesma em todas as tentativas).
//
// POST v1/customer/unified NÃO está aqui de propósito: com a chave a rota só cria
// (CPF existente => 409 Cpf.CpfAlreadyAdded); sem a chave ela é um upsert. Quem quiser
// o comportamento "só cria" precisa informar a chave explicitamente.
var autoKeyPaths = map[string]bool{
	"v1/customer":                        true,
	"v1/customer/address":                true,
	"v1/customer/professional":           true,
	"v1/proposal/simulation/financing":   true,
	"v1/proposal/simulation/refinancing": true,
	"v2/proposal/simulation/financing":   true,
	"v2/proposal/simulation/refinancing": true,
	"v1/proposal":                        true,
	"v2/proposal":                        true,
}

// TokenPath é a rota de geração do token de acesso.
const TokenPath = "v1/authentication/accesstoken"

// normalize remove a query string e barras nas pontas.
func normalize(path string) string {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	return strings.Trim(path, "/")
}

// proposalSubPath reconhece "v2/proposal/{id}/<suffix>".
func proposalSubPath(path, suffix string) bool {
	const prefix = "v2/proposal/"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, "/"+suffix) {
		return false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/"+suffix)
	return id != "" && !strings.Contains(id, "/")
}

// IsDocumentUpload indica POST v2/proposal/{id}/document, que exige Idempotency-Key.
func IsDocumentUpload(method, path string) bool {
	return strings.ToUpper(method) == "POST" && proposalSubPath(normalize(path), "document")
}

// RequiresKey indica se o SDK gera uma Idempotency-Key automática para a requisição.
func RequiresKey(method, path string) bool {
	if strings.ToUpper(method) != "POST" {
		return false
	}
	p := normalize(path)
	return autoKeyPaths[p] || proposalSubPath(p, "document")
}

// IsRetrySafe indica se a requisição pode ser repetida sem risco de duplicar efeitos.
//
// Seguras: GET/HEAD/OPTIONS; POST de token; POST com Idempotency-Key; POST de dados do
// veículo e de conta de recebimento (semântica de substituição); PUT/DELETE de webhook.
// Não seguras: POST v1/webhook e POST v1/customer/unified sem chave.
func IsRetrySafe(method, path string, hasKey bool) bool {
	p := normalize(path)

	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS":
		return true
	case "PUT", "DELETE":
		return strings.HasPrefix(p, "v1/webhook/")
	case "POST":
		if hasKey || p == TokenPath {
			return true
		}
		return proposalSubPath(p, "vehicle-detail") || proposalSubPath(p, "deposit-account")
	default:
		return false
	}
}
