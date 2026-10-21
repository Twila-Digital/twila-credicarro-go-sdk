package credicarro_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

func TestTokenEndpointStatusMapping(t *testing.T) {
	cases := []struct {
		status   int
		wantAuth bool
	}{{400, true}, {401, true}, {302, false}, {403, false}, {404, false}, {409, false}, {422, false}, {429, false}, {500, false}, {502, false}}
	for _, c := range cases {
		c := c
		t.Run(fmt.Sprint(c.status), func(t *testing.T) {
			api := newFakeAPI(t, okProposal)
			api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
				if c.status >= 300 && c.status < 400 {
					w.Header().Set("Location", "http://127.0.0.1:1/destino") // um 3xx real sempre traz Location
				}
				writeRaw(w, c.status, `{"tipo":"Token.Falha","titulo":"Falha"}`)
			}
			client := api.client(withResilience(func(r *credicarro.ResilienceOptions) { r.MaxRetryAttempts = 1 }))
			err := getProposal(client)

			var authErr *credicarro.AuthenticationError
			var apiErr *credicarro.APIError
			isAuth := errors.As(err, &authErr)
			if isAuth != c.wantAuth {
				t.Fatalf("status %d: AuthenticationError=%v, esperava %v (%T: %v)", c.status, isAuth, c.wantAuth, err, err)
			}
			if !c.wantAuth && (!errors.As(err, &apiErr) || apiErr.StatusCode != c.status || apiErr.ErrorCode != "Token.Falha") {
				t.Fatalf("status %d: esperava *APIError com o tipo, veio %T: %v", c.status, err, err)
			}
		})
	}
}

func TestNetworkBackoffNeverExceedsTheRemainingBudget(t *testing.T) {
	api := newFakeAPI(t, okProposal)
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) {
		r.RetryBaseDelay = 30 * time.Second // backoff enorme
		r.TotalTimeout = 400 * time.Millisecond
		r.AttemptTimeout = 200 * time.Millisecond
	}))
	if err := getProposal(client); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	api.server.Close()

	start := time.Now()
	err := getProposal(client)
	var timeoutErr *credicarro.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("esperava *TimeoutError, veio %T: %v", err, err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("o backoff deveria ser limitado ao orçamento restante: %v", time.Since(start))
	}
}

func TestJSONMarshalCarriesRealValuesWhileFormattingIsMasked(t *testing.T) {
	buyer := &credicarro.ProposalParticipantState{
		Document:  "12345678909",
		Documents: []credicarro.ProposalDocumentState{{TypeRaw: 1, Reason: "Documento ilegível de Maria"}},
	}
	file, _ := credicarro.NewDocumentFileFromBytes([]byte("conteudo"), "cnh.pdf")

	cases := []struct {
		want  string
		value interface{}
	}{
		{"12345678909", upsertRequest()},
		{"maria@exemplo.com.br", upsertRequest()},
		{"6500", upsertRequest()},
		{"Documento ilegível de Maria", credicarro.ProposalDetails{Buyer: buyer}},
		{"123456", credicarro.DepositAccount{HolderCpf: "12345678909", AccountNumber: "123456"}},
		{"motivo real", credicarro.ProposalWebhookEvent{RefusalReason: "motivo real"}},
		{"Y29udGV1ZG8=", file},
		{"credencial-real", credicarro.CreateWebhookRequest{Credential: "credencial-real"}},
		{"segredo-real", credicarro.CreateWebhookResult{WebhookID: testUUID, SigningSecret: "segredo-real"}},
		{"98765432100", credicarro.ProposalSummary{SellerCpf: "98765432100"}},
	}
	for _, c := range cases {
		data, err := json.Marshal(c.value)
		if err != nil || !strings.Contains(string(data), c.want) {
			t.Errorf("%T: json.Marshal deveria trazer %q (err=%v): %s", c.value, c.want, err, data)
		}
		if c.want == "6500" {
			continue // número comum pode aparecer em outro campo; renda mascarada é coberta em outro teste
		}
		if text := fmt.Sprintf("%v %+v %#v", c.value, c.value, c.value); strings.Contains(text, c.want) {
			t.Errorf("%T: a formatação não pode trazer %q: %s", c.value, c.want, text)
		}
	}
}

func TestDocumentFileJSONRoundTripKeepsContent(t *testing.T) {
	file, _ := credicarro.NewDocumentFileFromBytes([]byte("conteudo"), "cnh.pdf")
	data, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	var back credicarro.DocumentFile
	if err := json.Unmarshal(data, &back); err != nil || back.FileName() != "cnh.pdf" || back.Size() != 8 {
		t.Fatalf("round-trip incorreto: %v (%v)", back, err)
	}
	for _, bad := range []string{`{"fileName":"a.exe","base64Content":"eA=="}`, `{"fileName":"a.pdf","base64Content":"@@@"}`, `{"fileName":"a.pdf","base64Content":""}`} {
		var f credicarro.DocumentFile
		if err := json.Unmarshal([]byte(bad), &f); err == nil {
			t.Errorf("%s deveria ser recusado", bad)
		}
	}
}

func TestClientOptionsRefuseSerialization(t *testing.T) {
	_, err := json.Marshal(credicarro.ClientOptions{ClientID: "id", ClientSecret: testSecret})
	if err == nil || strings.Contains(err.Error(), testSecret) {
		t.Fatalf("a serialização das opções deveria ser recusada sem vazar o segredo: %v", err)
	}
}

func TestValueReceiversMaskWhenPrintedByValueOrPointer(t *testing.T) {
	buyer := credicarro.ProposalParticipantState{
		Document:  "12345678909",
		Documents: []credicarro.ProposalDocumentState{{Reason: "motivo sensível"}},
	}
	details := credicarro.ProposalDetails{Buyer: &buyer, RefusalReason: "motivo sensível"}
	event := credicarro.ProposalWebhookEvent{Buyer: &buyer, RefusalReason: "motivo sensível"}

	for _, v := range []interface{}{buyer, &buyer, details, &details, event, &event, buyer.Documents, buyer.Documents[0]} {
		for _, text := range []string{fmt.Sprint(v), fmt.Sprintf("%+v", v), fmt.Sprintf("%#v", v)} {
			if strings.Contains(text, "12345678909") || strings.Contains(text, "motivo sensível") {
				t.Errorf("%T vazou dado em %q", v, text)
			}
		}
	}
}

func TestClientStillWorksWithOwnTransport(t *testing.T) {
	client, _ := newTestClient(t, okProposal)
	if _, err := client.V1.Proposals.Get(context.Background(), testUUID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
}
