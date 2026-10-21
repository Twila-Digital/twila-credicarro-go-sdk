package credicarro_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

// Vetor da documentação (pages/webhooks/security.mdx).
const (
	docSecret    = "segredo-de-exemplo"
	docTimestamp = 1715792400
	docBody      = `{"id_proposta":"bea8d060-f93d-48eb-b6c4-45f64483cdd9","enum_status":2,"status":"Approved"}`
	docSignature = "db66c554eb116ef2577deb63f72619793502a31520f622b2d59864b074eb8df2"

	tenYears = 10 * 365 * 24 * time.Hour
)

func signedHeader(secret string, timestamp int64, body string) string {
	return fmt.Sprintf("t=%d,v1=%s", timestamp, credicarro.ComputeWebhookSignature(secret, timestamp, []byte(body)))
}

func v2Body() string {
	return `{
 "id_proposta":"bea8d060-f93d-48eb-b6c4-45f64483cdd9","enum_status":2,"status":"APROVADA","motivo_evento":"DOCUMENTOS_ALTERADOS",
 "motivo_recusa":null,"data_evento":"2026-10-12T17:20:31.512+00:00",
 "comprador":{"cpf":"12345678909","enum_biometria":3,"biometria":"APROVADA",
   "status_ativo":{"enum_etapa":7,"etapa":"CORRECAO_DOCUMENTOS","enum_situacao":4,"situacao":"PENDENTE_CORRECAO","enum_responsavel_acao":2,"responsavel_acao":"INTEGRADOR"},
   "etapas_concluidas":[{"enum_etapa":1,"etapa":"PROPOSTA_CRIADA","data":"2026-10-12T14:05:08-03:00"}],
   "documentos":[{"enum_tipo":1,"tipo":"CNH","enum_status":2,"status":"INVALIDO","motivo":"Documento ilegível"}]},
 "loja":{"cnpj":"11222333000181",
   "status_ativo":{"enum_etapa":6,"etapa":"ENVIO_DOCUMENTOS","enum_situacao":1,"situacao":"AGUARDANDO_ENVIO","enum_responsavel_acao":2,"responsavel_acao":"INTEGRADOR"},
   "etapas_concluidas":[],"documentos":[]},
 "vendedor":null}`
}

func TestSignatureMatchesTheDocumentationVector(t *testing.T) {
	if got := credicarro.ComputeWebhookSignature(docSecret, docTimestamp, []byte(docBody)); got != docSignature {
		t.Fatalf("v1 calculado = %s, esperava %s", got, docSignature)
	}

	header := fmt.Sprintf("t=%d,v1=%s", docTimestamp, docSignature)
	if !credicarro.VerifyWebhookSignature([]byte(docBody), header, docSecret, tenYears) {
		t.Fatal("a assinatura do vetor da documentação deveria ser aceita")
	}

	event, err := credicarro.ParseWebhookEvent([]byte(docBody), header, docSecret, tenYears)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if event.ProposalID != "bea8d060-f93d-48eb-b6c4-45f64483cdd9" || event.Status != credicarro.ProposalStatusApproved || event.StatusRaw != 2 || event.StatusName != "Approved" {
		t.Fatalf("evento incorreto: %+v", event)
	}
}

func TestSignatureAcceptsUppercaseHexAndExtraSpaces(t *testing.T) {
	header := fmt.Sprintf("t=%d , v1=%s", docTimestamp, strings.ToUpper(docSignature))
	if !credicarro.VerifyWebhookSignature([]byte(docBody), header, docSecret, tenYears) {
		t.Fatal("hex em maiúsculas e espaços ao redor deveriam ser aceitos")
	}
}

func TestSignatureRejectsTamperedBodySecretAndHeader(t *testing.T) {
	good := fmt.Sprintf("t=%d,v1=%s", docTimestamp, docSignature)

	cases := map[string]struct {
		body, header, secret string
	}{
		"corpo alterado":     {strings.Replace(docBody, `"enum_status":2`, `"enum_status":3`, 1), good, docSecret},
		"corpo reformatado":  {strings.Replace(docBody, `,"`, `, "`, 1), good, docSecret},
		"segredo errado":     {docBody, good, "outro-segredo"},
		"segredo vazio":      {docBody, good, ""},
		"timestamp alterado": {docBody, fmt.Sprintf("t=%d,v1=%s", docTimestamp+1, docSignature), docSecret},
		"assinatura zerada":  {docBody, fmt.Sprintf("t=%d,v1=%s", docTimestamp, strings.Repeat("0", 64)), docSecret},
		"cabeçalho vazio":    {docBody, "", docSecret},
		"sem t":              {docBody, "v1=" + docSignature, docSecret},
		"sem v1":             {docBody, fmt.Sprintf("t=%d", docTimestamp), docSecret},
		"t não numérico":     {docBody, "t=abc,v1=" + docSignature, docSecret},
		"t com 11 dígitos":   {docBody, "t=17157924000,v1=" + docSignature, docSecret},
		"t negativo":         {docBody, "t=-1715792400,v1=" + docSignature, docSecret},
		"t repetido":         {docBody, fmt.Sprintf("t=%d,t=%d,v1=%s", docTimestamp, docTimestamp, docSignature), docSecret},
		"lixo":               {docBody, "garbage-header", docSecret},
		"v1 repetido":        {docBody, fmt.Sprintf("t=%d,v1=%s,v1=%s", docTimestamp, docSignature, docSignature), docSecret},
		"v1 vazio":           {docBody, fmt.Sprintf("t=%d,v1=", docTimestamp), docSecret},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			if credicarro.VerifyWebhookSignature([]byte(c.body), c.header, c.secret, tenYears) {
				t.Fatal("a assinatura deveria ser rejeitada")
			}
			_, err := credicarro.ParseWebhookEvent([]byte(c.body), c.header, c.secret, tenYears)
			var sigErr *credicarro.WebhookSignatureError
			if !errors.As(err, &sigErr) {
				t.Fatalf("esperava *WebhookSignatureError, veio %T: %v", err, err)
			}
		})
	}
}

func TestEmptySecretIsAlwaysInvalidEvenWithAMatchingSignature(t *testing.T) {
	// HMAC com chave vazia é calculável; o SDK precisa recusar mesmo assim.
	header := signedHeader("", time.Now().Unix(), docBody)
	if credicarro.VerifyWebhookSignature([]byte(docBody), header, "", 0) {
		t.Fatal("segredo vazio nunca deve validar")
	}
}

func TestToleranceAppliesInBothDirections(t *testing.T) {
	now := time.Now().Unix()
	cases := map[string]struct {
		timestamp int64
		tolerance time.Duration
		valid     bool
	}{
		"agora":                     {now, 0, true},
		"4 minutos atrás":           {now - 240, 0, true},
		"6 minutos atrás":           {now - 360, 0, false},
		"4 minutos no futuro":       {now + 240, 0, true},
		"6 minutos no futuro":       {now + 360, 0, false},
		"tolerância customizada ok": {now - 600, 15 * time.Minute, true},
		"tolerância customizada":    {now - 600, time.Minute, false},
		"tolerância negativa":       {now - 240, -time.Hour, false},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			header := signedHeader(docSecret, c.timestamp, docBody)
			if got := credicarro.VerifyWebhookSignature([]byte(docBody), header, docSecret, c.tolerance); got != c.valid {
				t.Fatalf("esperava %v, veio %v", c.valid, got)
			}
			_, err := credicarro.ParseWebhookEvent([]byte(docBody), header, docSecret, c.tolerance)
			if (err == nil) != c.valid {
				t.Fatalf("ParseWebhookEvent: esperava válido=%v, veio erro %v", c.valid, err)
			}
		})
	}
}

func TestParseWebhookEventRejectsInvalidPayloads(t *testing.T) {
	for _, body := range []string{``, `not json`, `[]`, `{}`, `{"enum_status":2}`, `{"id_proposta":"abc","enum_status":2}`, `{"id_proposta":"","enum_status":2}`} {
		_, err := credicarro.ParseWebhookEventUnverified([]byte(body))
		var sigErr *credicarro.WebhookSignatureError
		if !errors.As(err, &sigErr) {
			t.Errorf("corpo %q: esperava *WebhookSignatureError, veio %T: %v", body, err, err)
		}
	}
}

func TestParseWebhookEventV1Payload(t *testing.T) {
	event, err := credicarro.ParseWebhookEventUnverified([]byte(docBody))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if event.IsDetailed || event.EventReason != nil || event.EventReasonRaw != "" || event.EventDate != "" || event.RefusalReason != "" ||
		event.Buyer != nil || event.Seller != nil || event.Store != nil {
		t.Fatalf("payload v1 não deveria trazer detalhes: %+v", event)
	}
}

func TestParseWebhookEventV1StatusByNameWhenEnumIsAbsent(t *testing.T) {
	cases := map[string]credicarro.ProposalStatus{
		"Analysing": credicarro.ProposalStatusAnalysing, "Approved": credicarro.ProposalStatusApproved, "Refused": credicarro.ProposalStatusRefused,
		"Canceled": credicarro.ProposalStatusCanceled, "ManualAnalysis": credicarro.ProposalStatusManualAnalysis, "Completed": credicarro.ProposalStatusCompleted,
		"InNegotiation": credicarro.ProposalStatusInNegotiation, "Expired": credicarro.ProposalStatusExpired, "Pending": credicarro.ProposalStatusPending,
		"EM_ANALISE": credicarro.ProposalStatusAnalysing, "APROVADA": credicarro.ProposalStatusApproved, "RECUSADA": credicarro.ProposalStatusRefused,
		"CANCELADA": credicarro.ProposalStatusCanceled, "ANALISE_MANUAL": credicarro.ProposalStatusManualAnalysis, "FINALIZADA": credicarro.ProposalStatusCompleted,
		"EM_NEGOCIACAO": credicarro.ProposalStatusInNegotiation, "EXPIRADA": credicarro.ProposalStatusExpired, "PENDENTE": credicarro.ProposalStatusPending,
		"NOVO_STATUS": credicarro.ProposalStatusUnknown,
	}
	for name, expected := range cases {
		body := fmt.Sprintf(`{"id_proposta":"%s","status":%q}`, testUUID, name)
		event, err := credicarro.ParseWebhookEventUnverified([]byte(body))
		if err != nil || event.Status != expected || event.StatusName != name || event.StatusRaw != 0 {
			t.Errorf("status %q: esperava %d, veio %+v (%v)", name, expected, event, err)
		}
	}
}

func TestParseWebhookEventUnknownEnumKeepsRawValue(t *testing.T) {
	event, err := credicarro.ParseWebhookEventUnverified([]byte(`{"id_proposta":"` + testUUID + `","enum_status":42,"status":"NOVO"}`))
	if err != nil || event.Status != credicarro.ProposalStatusUnknown || event.StatusRaw != 42 || event.StatusName != "NOVO" {
		t.Fatalf("evento incorreto: %+v (%v)", event, err)
	}
}

func TestParseWebhookEventV2Payload(t *testing.T) {
	body := v2Body()
	header := signedHeader(docSecret, time.Now().Unix(), body)

	event, err := credicarro.ParseWebhookEvent([]byte(body), header, docSecret, 0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if !event.IsDetailed || event.Status != credicarro.ProposalStatusApproved || event.StatusRaw != 2 || event.StatusName != "APROVADA" {
		t.Fatalf("cabeçalho incorreto: %+v", event)
	}
	if event.EventReason == nil || *event.EventReason != credicarro.ProposalEventReasonDocumentsChanged || event.EventReasonRaw != "DOCUMENTOS_ALTERADOS" {
		t.Fatalf("motivo do evento incorreto: %v %q", event.EventReason, event.EventReasonRaw)
	}
	if event.RefusalReason != "" || event.EventDate != "2026-10-12T17:20:31.512+00:00" || event.Seller != nil {
		t.Fatalf("campos opcionais incorretos: %+v", event)
	}

	buyer := event.Buyer
	if buyer == nil || buyer.Document != "12345678909" || buyer.ActiveStep == nil || buyer.ActiveStep.Step != credicarro.ProposalStepDocumentCorrection ||
		buyer.ActiveStep.ActionOwner != credicarro.ActionOwnerIntegrator || len(buyer.Documents) != 1 ||
		buyer.Documents[0].Status != credicarro.DocumentStatusInvalid || buyer.Documents[0].Reason != "Documento ilegível" {
		t.Fatalf("comprador incorreto: %+v", buyer)
	}
	store := event.Store
	if store == nil || store.Document != "11222333000181" || store.ActiveStep.Step != credicarro.ProposalStepDocumentSubmission {
		t.Fatalf("loja incorreta: %+v", store)
	}

	details := credicarro.ProposalDetails{Buyer: event.Buyer, Store: event.Store}
	if got := len(details.PendingActions()); got != 2 {
		t.Fatalf("o mesmo tipo de participante deveria alimentar PendingActions: %d ações", got)
	}
}

func TestEventReasonMapping(t *testing.T) {
	cases := map[string]credicarro.ProposalEventReason{
		"STATUS_ALTERADO": credicarro.ProposalEventReasonStatusChanged, "ETAPAS_ALTERADAS": credicarro.ProposalEventReasonStepsChanged,
		"DOCUMENTOS_ALTERADOS": credicarro.ProposalEventReasonDocumentsChanged, "NOVO_MOTIVO": credicarro.ProposalEventReasonUnknown,
	}
	for raw, expected := range cases {
		body := fmt.Sprintf(`{"id_proposta":%q,"enum_status":1,"status":"EM_ANALISE","motivo_evento":%q}`, testUUID, raw)
		event, err := credicarro.ParseWebhookEventUnverified([]byte(body))
		if err != nil || event.EventReason == nil || *event.EventReason != expected || event.EventReasonRaw != raw {
			t.Errorf("motivo %q: esperava %d, veio %+v (%v)", raw, expected, event, err)
		}
	}
}

func TestVerifyWebhookAuthorizationIsExactAndRejectsEmptyExpected(t *testing.T) {
	cases := []struct {
		received, expected string
		valid              bool
	}{
		{"Basic dXN1YXJpbzpzZW5oYQ==", "Basic dXN1YXJpbzpzZW5oYQ==", true},
		{"Bearer abc.def.ghi", "Bearer abc.def.ghi", true},
		{"Basic dXN1YXJpbzpzZW5oYQ==", "Basic outra", false},
		{"basic dXN1YXJpbzpzZW5oYQ==", "Basic dXN1YXJpbzpzZW5oYQ==", false},
		{"", "Basic dXN1YXJpbzpzZW5oYQ==", false},
		{"Basic x", "", false},
		{"", "", false},
		{"Basic dXN1YXJpbzpzZW5oYQ== ", "Basic dXN1YXJpbzpzZW5oYQ==", false},
	}
	for _, c := range cases {
		if got := credicarro.VerifyWebhookAuthorization(c.received, c.expected); got != c.valid {
			t.Errorf("VerifyWebhookAuthorization(%q, %q) = %v, esperava %v", c.received, c.expected, got, c.valid)
		}
	}
}
