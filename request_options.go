package credicarro

import (
	"strings"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/transport"
)

// RequestOption ajusta uma chamada individual (hoje: a Idempotency-Key explícita).
type RequestOption func(*requestOptions)

type requestOptions struct {
	idempotencyKey string
	keySet         bool
}

// WithIdempotencyKey define a Idempotency-Key (1 a 200 caracteres) da chamada, no lugar
// da chave automática. Aceita pelos métodos POST que enviam o header.
//
// Atenção em Customers.Upsert: sem chave a rota é um upsert (cria ou atualiza); COM chave
// ela só cria (CPF já cadastrado => 409 Cpf.CpfAlreadyAdded). O SDK nunca gera a chave
// automaticamente nessa rota.
//
// Em Proposals.UploadDocument, persista uma chave por envio e reuse a mesma nas
// retentativas; use uma chave nova ao reenviar a correção de um documento recusado.
func WithIdempotencyKey(key string) RequestOption {
	return func(o *requestOptions) { o.idempotencyKey, o.keySet = key, true }
}

func applyRequestOptions(opts []RequestOption) (transport.RequestOptions, error) {
	var o requestOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.keySet && !validIdempotencyKey(o.idempotencyKey) {
		return transport.RequestOptions{}, &ConfigurationError{Message: "A Idempotency-Key deve ter de 1 a 200 caracteres ASCII imprimíveis e não pode ser só espaços."}
	}
	return transport.RequestOptions{IdempotencyKey: o.idempotencyKey}, nil
}

// validIdempotencyKey exige 1 a 200 caracteres ASCII imprimíveis (0x20 a 0x7E), não só espaços.
func validIdempotencyKey(key string) bool {
	if len(key) < 1 || len(key) > 200 || strings.TrimSpace(key) == "" {
		return false
	}
	for i := 0; i < len(key); i++ {
		if key[i] < 0x20 || key[i] > 0x7E {
			return false
		}
	}
	return true
}
