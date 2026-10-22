package credicarro_test

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

func TestWebhooksCreateReturnsIDAndSigningSecret(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusCreated, `{"id_webhook":"`+testUUID+`","segredo_assinatura":"segredo-gerado"}`)
	})

	result, err := client.Webhooks.Create(context.Background(), credicarro.CreateWebhookRequest{
		Type: credicarro.WebhookTypeProposal, URL: "https://parceiro.com.br/webhooks/credicarro",
		AuthenticationType: credicarro.WebhookAuthenticationTypeBasic, Credential: "dXN1YXJpbzpzZW5oYQ==",
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if result.WebhookID != testUUID || result.SigningSecret != "segredo-gerado" {
		t.Fatalf("resultado incorreto: %+v", result)
	}

	req := api.recorded()[0]
	expected := map[string]interface{}{
		"tipo": float64(3), "url": "https://parceiro.com.br/webhooks/credicarro", "tipo_autenticacao": float64(2), "credencial": "dXN1YXJpbzpzZW5oYQ==",
	}
	if req.Method != http.MethodPost || req.Path != "v1/webhook" || !reflect.DeepEqual(req.JSON(t), expected) {
		t.Fatalf("requisição incorreta: %s %s %v", req.Method, req.Path, req.JSON(t))
	}
}

func TestWebhooksCreateOmitsCredentialWhenEmpty(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusCreated, `{"id_webhook":"`+testUUID+`","segredo_assinatura":"s"}`)
	})
	_, err := client.Webhooks.Create(context.Background(), credicarro.CreateWebhookRequest{
		Type: credicarro.WebhookTypeCustomer, URL: "https://parceiro.com.br/h", AuthenticationType: credicarro.WebhookAuthenticationTypeNone,
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if _, present := api.recorded()[0].JSON(t)["credencial"]; present {
		t.Fatal("credencial vazia não deve ser enviada")
	}
}

func TestWebhooksCreateMapsInvalidEndpointAsBusinessRule(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusUnprocessableEntity, `{"tipo":"Organization.Webhook.InvalidEndpoint","titulo":"Regra de negócio não atendida","status":422}`)
	})
	_, err := client.Webhooks.Create(context.Background(), credicarro.CreateWebhookRequest{Type: credicarro.WebhookTypeProposal, URL: "http://localhost"})
	var rule *credicarro.BusinessRuleError
	if !asBusinessRule(err, &rule) || rule.ErrorCode != "Organization.Webhook.InvalidEndpoint" {
		t.Fatalf("esperava BusinessRuleError, veio %T: %v", err, err)
	}
}

func TestSecretsNeverAppearWhenFormattingWebhookTypes(t *testing.T) {
	result := credicarro.CreateWebhookResult{WebhookID: testUUID, SigningSecret: "segredo-gerado"}
	request := credicarro.CreateWebhookRequest{Type: credicarro.WebhookTypeProposal, URL: "https://x.com.br/h", Credential: "credencial-secreta"}

	for _, text := range []string{
		fmt.Sprint(result), fmt.Sprintf("%+v", result), fmt.Sprintf("%#v", result), fmt.Sprintf("%v", &result),
		fmt.Sprint(request), fmt.Sprintf("%+v", request), fmt.Sprintf("%#v", request),
	} {
		if strings.Contains(text, "segredo-gerado") || strings.Contains(text, "credencial-secreta") {
			t.Errorf("segredo vazou em %q", text)
		}
	}
}

func TestWebhooksListParsesTextEnums(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"itens":[
 {"id_webhook":"`+testUUID+`","tipo":"Proposal","url":"https://parceiro.com.br/a","tipo_autenticacao":"Basic","status":"Active"},
 {"id_webhook":"`+testUUID2+`","tipo":"customer","url":"https://parceiro.com.br/b","tipo_autenticacao":"JWT","status":"inactive"},
 {"id_webhook":"`+testUUID+`","tipo":"Simulation","url":"https://parceiro.com.br/c","tipo_autenticacao":"None","status":"Pausado"},
 {"id_webhook":"`+testUUID+`","tipo":"Futuro","url":"https://parceiro.com.br/d","tipo_autenticacao":"Mtls","status":"Active"}
],"pagina":{"tem_proximo":false,"tem_anterior":false,"numero":1,"tamanho":10,"total":4}}`)
	})

	page, err := client.Webhooks.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if req := api.recorded()[0]; req.Path != "v1/webhook" || req.RawQuery != "pagina_inicial=1&limite=10" {
		t.Fatalf("requisição incorreta: %s ?%s", req.Path, req.RawQuery)
	}
	if page.TotalCount != 4 || len(page.Items) != 4 {
		t.Fatalf("página incorreta: %+v", page)
	}

	a, b, c, d := page.Items[0], page.Items[1], page.Items[2], page.Items[3]
	if a.ID != testUUID || a.Type != credicarro.WebhookTypeProposal || a.TypeRaw != "Proposal" || a.AuthenticationType != credicarro.WebhookAuthenticationTypeBasic ||
		a.Status != credicarro.WebhookStatusActive || a.URL != "https://parceiro.com.br/a" {
		t.Fatalf("webhook A incorreto: %+v", a)
	}
	if b.Type != credicarro.WebhookTypeCustomer || b.AuthenticationType != credicarro.WebhookAuthenticationTypeJwt || b.Status != credicarro.WebhookStatusInactive {
		t.Fatalf("webhook B incorreto: %+v", b)
	}
	if c.Type != credicarro.WebhookTypeSimulation || c.AuthenticationType != credicarro.WebhookAuthenticationTypeNone ||
		c.Status != credicarro.WebhookStatusUnknown || c.StatusRaw != "Pausado" {
		t.Fatalf("webhook C incorreto: %+v", c)
	}
	if d.Type != credicarro.WebhookTypeUnknown || d.TypeRaw != "Futuro" || d.AuthenticationType != credicarro.WebhookAuthenticationTypeUnknown || d.AuthenticationTypeRaw != "Mtls" {
		t.Fatalf("webhook D incorreto: %+v", d)
	}
}

func TestWebhooksUpdateSendsPutWithOptionalStatus(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	inactive := credicarro.WebhookStatusInactive
	if err := client.Webhooks.Update(context.Background(), testUUID, credicarro.UpdateWebhookRequest{
		Type: credicarro.WebhookTypeProposal, URL: "https://parceiro.com.br/novo", Status: &inactive,
	}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if err := client.Webhooks.Update(context.Background(), testUUID, credicarro.UpdateWebhookRequest{
		Type: credicarro.WebhookTypeProposal, URL: "https://parceiro.com.br/novo",
	}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	reqs := api.recorded()
	withStatus := map[string]interface{}{"tipo": float64(3), "url": "https://parceiro.com.br/novo", "status": float64(2)}
	withoutStatus := map[string]interface{}{"tipo": float64(3), "url": "https://parceiro.com.br/novo"}
	if reqs[0].Method != http.MethodPut || reqs[0].Path != "v1/webhook/"+testUUID || !reflect.DeepEqual(reqs[0].JSON(t), withStatus) {
		t.Fatalf("PUT com status incorreto: %s %v", reqs[0].Path, reqs[0].JSON(t))
	}
	if !reflect.DeepEqual(reqs[1].JSON(t), withoutStatus) {
		t.Fatalf("PUT sem status deveria omitir o campo: %v", reqs[1].JSON(t))
	}
}

func TestWebhooksUpdateConcurrencyConflict(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusConflict, `{"tipo":"Organization.Webhook.ConcurrencyConflict","titulo":"Conflito","status":409}`)
	})
	err := client.Webhooks.Update(context.Background(), testUUID, credicarro.UpdateWebhookRequest{Type: credicarro.WebhookTypeProposal, URL: "https://x.com.br/h"})
	var apiErr *credicarro.APIError
	if !asAPIError(err, &apiErr) || apiErr.StatusCode != 409 || apiErr.ErrorCode != "Organization.Webhook.ConcurrencyConflict" {
		t.Fatalf("esperava APIError 409, veio %T: %v", err, err)
	}
}

func TestWebhooksDeleteSendsDelete(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	if err := client.Webhooks.Delete(context.Background(), testUUID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if req := api.recorded()[0]; req.Method != http.MethodDelete || req.Path != "v1/webhook/"+testUUID {
		t.Fatalf("requisição incorreta: %s %s", req.Method, req.Path)
	}
}

func TestWebhooksNotFound(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusNotFound, `{"tipo":"Webhook.Registration.NotFound","titulo":"Não encontrado","status":404}`)
	})
	err := client.Webhooks.Delete(context.Background(), testUUID)
	var apiErr *credicarro.APIError
	if !asAPIError(err, &apiErr) || apiErr.StatusCode != 404 || apiErr.ErrorCode != "Webhook.Registration.NotFound" {
		t.Fatalf("esperava APIError 404, veio %T: %v", err, err)
	}
}

func TestWebhooksRejectInvalidID(t *testing.T) {
	client, api := newTestClient(t, nil)
	assertConfigurationError(t, client.Webhooks.Delete(context.Background(), "../v1/customer"))
	assertConfigurationError(t, client.Webhooks.Update(context.Background(), "", credicarro.UpdateWebhookRequest{}))
	if len(api.recorded()) != 0 {
		t.Fatal("nenhuma requisição deveria ser enviada")
	}
}
