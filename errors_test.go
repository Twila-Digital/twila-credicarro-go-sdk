package credicarro_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

func asAPIError(err error, target **credicarro.APIError) bool { return errors.As(err, target) }

func asBusinessRule(err error, target **credicarro.BusinessRuleError) bool {
	return errors.As(err, target)
}

func asTimeoutError(err error, target **credicarro.TimeoutError) bool { return errors.As(err, target) }

// errorClient devolve um cliente sem retry cujo recurso sempre responde status/body.
func errorClient(t *testing.T, status int, body string, headers map[string]string) *credicarro.Client {
	t.Helper()
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		if body == "" {
			w.WriteHeader(status)
			return
		}
		writeRaw(w, status, body)
	})
	return api.client(withResilience(func(r *credicarro.ResilienceOptions) { r.MaxRetryAttempts = 1 }))
}

func getProposal(client *credicarro.Client) error {
	_, err := client.Proposals.Get(context.Background(), testUUID)
	return err
}

func TestBusinessRuleErrorFor422(t *testing.T) {
	body := `{"tipo":"Store.Integration.Intermediation","titulo":"Regra de negócio não atendida","status":422,"detalhe":"A loja possui contrato de intermediação e não é elegível para a integração."}`
	err := getProposal(errorClient(t, http.StatusUnprocessableEntity, body, nil))

	var rule *credicarro.BusinessRuleError
	if !errors.As(err, &rule) {
		t.Fatalf("esperava *BusinessRuleError, veio %T: %v", err, err)
	}
	if rule.StatusCode != 422 || rule.ErrorCode != "Store.Integration.Intermediation" || rule.Title != "Regra de negócio não atendida" ||
		!strings.Contains(rule.Detail, "intermediação") || rule.Field != "" || rule.Body != body {
		t.Fatalf("campos incorretos: %+v", rule.APIError)
	}

	var api *credicarro.APIError
	if !errors.As(err, &api) || api != rule.APIError {
		t.Fatal("o BusinessRuleError deveria ser desembrulhável como *APIError")
	}
}

func TestValidationErrorFor400WithFieldAndErrorsList(t *testing.T) {
	body := `{"tipo":"Validation.Cpf.Invalid","titulo":"Requisição inválida","status":400,"detalhe":"O CPF informado em 'cpf' é inválido.","campo":"cpf","erros":[{"codigo":"Validation.InvalidFormat","mensagem":"Valor com tipo errado.","campo":"valorVeiculo"}]}`
	err := getProposal(errorClient(t, http.StatusBadRequest, body, nil))

	var validation *credicarro.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("esperava *ValidationError, veio %T: %v", err, err)
	}
	if validation.ErrorCode != "Validation.Cpf.Invalid" || validation.Field != "cpf" || len(validation.Errors) != 1 {
		t.Fatalf("campos incorretos: %+v", validation.APIError)
	}
	item := validation.Errors[0]
	if item.Code != "Validation.InvalidFormat" || item.Message != "Valor com tipo errado." || item.Field != "valorVeiculo" {
		t.Fatalf("item de erros incorreto: %+v", item)
	}
}

func TestRateLimitErrorFor429WithRetryAfter(t *testing.T) {
	err := getProposal(errorClient(t, http.StatusTooManyRequests, `{"tipo":"RateLimit.Exceeded"}`, map[string]string{"Retry-After": "7"}))

	var limit *credicarro.RateLimitError
	if !errors.As(err, &limit) {
		t.Fatalf("esperava *RateLimitError, veio %T: %v", err, err)
	}
	if limit.StatusCode != 429 || limit.RetryAfter != 7*time.Second || limit.ErrorCode != "RateLimit.Exceeded" {
		t.Fatalf("campos incorretos: %+v (retryAfter=%v)", limit.APIError, limit.RetryAfter)
	}
}

func TestRateLimitErrorWithoutRetryAfterHeader(t *testing.T) {
	err := getProposal(errorClient(t, http.StatusTooManyRequests, ``, nil))
	var limit *credicarro.RateLimitError
	if !errors.As(err, &limit) || limit.RetryAfter != 0 {
		t.Fatalf("esperava *RateLimitError com RetryAfter zero, veio %T: %v", err, err)
	}
}

func TestPlainAPIErrorFor404And409And403(t *testing.T) {
	cases := []struct {
		status int
		code   string
	}{
		{404, "Proposal.Access.NotFound"},
		{409, "Proposal.IdempotencyKeyReused"},
		{403, "Integration.Access.Inactive"},
		{502, "Document.UploadFailed"},
	}
	for _, c := range cases {
		err := getProposal(errorClient(t, c.status, `{"tipo":"`+c.code+`","titulo":"T"}`, nil))
		var apiErr *credicarro.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != c.status || apiErr.ErrorCode != c.code {
			t.Errorf("status %d: esperava *APIError com código %s, veio %T: %v", c.status, c.code, err, err)
		}
		var validation *credicarro.ValidationError
		var rule *credicarro.BusinessRuleError
		if errors.As(err, &validation) || errors.As(err, &rule) {
			t.Errorf("status %d não deveria virar ValidationError/BusinessRuleError", c.status)
		}
	}
}

func TestStandardProblemDetailsFallback(t *testing.T) {
	body := `{"type":"https://tools.ietf.org/html/rfc9110#section-15.6.1","title":"Erro inesperado","status":500,"detail":"Falha ao processar CPF 12345678909."}`
	err := getProposal(errorClient(t, http.StatusInternalServerError, body, nil))

	var apiErr *credicarro.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("esperava *APIError, veio %T: %v", err, err)
	}
	if apiErr.ErrorCode != "https://tools.ietf.org/html/rfc9110#section-15.6.1" || apiErr.Title != "Erro inesperado" || !strings.Contains(apiErr.Detail, "12345678909") {
		t.Fatalf("o fallback type/title/detail não foi aplicado: %+v", apiErr)
	}
}

func TestPortugueseFieldsTakePrecedenceOverStandardOnes(t *testing.T) {
	body := `{"tipo":"Codigo.Pt","type":"codigo.en","titulo":"Titulo PT","title":"Title EN","detalhe":"Detalhe PT","detail":"Detail EN"}`
	err := getProposal(errorClient(t, http.StatusConflict, body, nil))

	var apiErr *credicarro.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode != "Codigo.Pt" || apiErr.Title != "Titulo PT" || apiErr.Detail != "Detalhe PT" {
		t.Fatalf("campos em português deveriam vencer: %+v", apiErr)
	}
}

func TestUnauthorizedWithoutBodyIsAuthenticationError(t *testing.T) {
	err := getProposal(errorClient(t, http.StatusUnauthorized, ``, nil))

	var authErr *credicarro.AuthenticationError
	if !errors.As(err, &authErr) || authErr.StatusCode != 401 || authErr.Body != "" {
		t.Fatalf("esperava *AuthenticationError 401 sem corpo, veio %T: %v", err, err)
	}
}

func TestNonJSONErrorBodyIsKeptRawWithoutFailing(t *testing.T) {
	err := getProposal(errorClient(t, http.StatusBadGateway, `<html>Bad Gateway</html>`, nil))

	var apiErr *credicarro.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 502 || apiErr.ErrorCode != "" || apiErr.Body != `<html>Bad Gateway</html>` {
		t.Fatalf("esperava *APIError com o corpo bruto, veio %T: %+v", err, apiErr)
	}
}

func TestErrorMessageHasStatusCodeAndTitleButNeverDetailOrBody(t *testing.T) {
	body := `{"tipo":"Customer.Account.NotFound","titulo":"Cliente não encontrado","status":404,"detalhe":"Nenhum cliente com o CPF 12345678909 e e-mail maria@exemplo.com.br."}`
	err := getProposal(errorClient(t, http.StatusNotFound, body, nil))

	msg := err.Error()
	for _, want := range []string{"404", "Customer.Account.NotFound", "Cliente não encontrado"} {
		if !strings.Contains(msg, want) {
			t.Errorf("a mensagem %q deveria conter %q", msg, want)
		}
	}
	for _, forbidden := range []string{"12345678909", "maria@exemplo.com.br", "Nenhum cliente"} {
		if strings.Contains(msg, forbidden) {
			t.Errorf("a mensagem %q não pode conter %q (dado pessoal)", msg, forbidden)
		}
	}
}

func TestMessagesWithoutCodeOrTitle(t *testing.T) {
	err := getProposal(errorClient(t, http.StatusServiceUnavailable, ``, nil))
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("a mensagem deveria trazer o status: %v", err)
	}
}

func TestMalformedProblemDetailsAreIgnored(t *testing.T) {
	// "erros" como string em vez de lista: o corpo bruto continua disponível
	err := getProposal(errorClient(t, http.StatusBadRequest, `{"tipo":"Validation.Field.Required","erros":"texto"}`, nil))
	var validation *credicarro.ValidationError
	if !errors.As(err, &validation) || validation.Body == "" {
		t.Fatalf("esperava *ValidationError, veio %T: %v", err, err)
	}
}

func TestNetworkFailureIsReturnedWithoutLeakingQueryString(t *testing.T) {
	api := newFakeAPI(t, nil)
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) { r.MaxRetryAttempts = 1 }))
	// derruba o servidor depois de obter o token
	if _, err := client.Simulations.GetFinancingRange(context.Background(), "ABC1D23", "11222333000181"); err == nil {
		t.Fatal("esperava erro 404 do servidor sem rotas")
	}
	api.server.Close()

	_, err := client.Simulations.GetFinancingRange(context.Background(), "ABC1D23", "11222333000181")
	if err == nil {
		t.Fatal("esperava erro de rede")
	}
	if strings.Contains(err.Error(), "ABC1D23") || strings.Contains(err.Error(), "11222333000181") {
		t.Fatalf("a mensagem não pode vazar a query string: %q", err.Error())
	}
}
