package credicarro

import (
	"context"
	"fmt"
	"net/url"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/transport"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

// noOptions é a ausência de ajustes de requisição.
var noOptions = transport.RequestOptions{}

// CreateWebhookRequest cadastra um webhook. Credential é obrigatória para Basic (valor em
// Base64 de usuario:senha, enviado como "Basic {credencial}") e Jwt (token, enviado como
// "Bearer {credencial}"). A autenticação não pode ser alterada depois da criação.
type CreateWebhookRequest struct {
	Type               WebhookType
	URL                string // HTTPS pública, até 2048 caracteres
	AuthenticationType WebhookAuthenticationType
	Credential         string
}

// String não inclui a Credential.
func (r CreateWebhookRequest) String() string {
	return fmt.Sprintf("CreateWebhookRequest{Type: %d, URL: %q, AuthenticationType: %d, Credential: [REDACTED]}", r.Type, r.URL, r.AuthenticationType)
}

// GoString não inclui a Credential (verbo %#v).
func (r CreateWebhookRequest) GoString() string { return r.String() }

// CreateWebhookResult é o retorno do cadastro. SigningSecret é o segredo de assinatura em
// claro, devolvido SOMENTE nesta resposta — guarde-o em um cofre de segredos.
type CreateWebhookResult struct {
	WebhookID     string
	SigningSecret string
}

// String não inclui o SigningSecret.
func (r CreateWebhookResult) String() string {
	return fmt.Sprintf("CreateWebhookResult{WebhookID: %q, SigningSecret: [REDACTED]}", r.WebhookID)
}

// GoString não inclui o SigningSecret (verbo %#v).
func (r CreateWebhookResult) GoString() string { return r.String() }

// UpdateWebhookRequest altera tipo, URL e status. Status nil mantém o status atual.
type UpdateWebhookRequest struct {
	Type   WebhookType
	URL    string
	Status *WebhookStatus
}

// Webhook é um webhook cadastrado. A credencial e o segredo de assinatura não são retornados.
// Os campos XxxRaw guardam o texto original devolvido pela API.
type Webhook struct {
	ID                    string
	Type                  WebhookType
	TypeRaw               string
	URL                   string
	AuthenticationType    WebhookAuthenticationType
	AuthenticationTypeRaw string
	Status                WebhookStatus
	StatusRaw             string
}

// WebhooksClient expõe as operações de webhooks.
type WebhooksClient struct {
	api *apiCaller
}

// Create cadastra um webhook ativo (POST v1/webhook). A rota não aceita Idempotency-Key e
// NÃO é repetida em falhas transitórias, para não cadastrar webhooks duplicados: se a
// chamada falhar por timeout, liste os webhooks antes de tentar de novo.
func (c *WebhooksClient) Create(ctx context.Context, req CreateWebhookRequest) (*CreateWebhookResult, error) {
	wireReq := wire.CreateWebhookRequest{
		Type:               int(req.Type),
		URL:                req.URL,
		AuthenticationType: int(req.AuthenticationType),
		Credential:         req.Credential,
	}

	var result wire.CreateWebhookResponse
	if err := c.api.post(ctx, "v1/webhook", wireReq, &result, noOptions); err != nil {
		return nil, err
	}
	return &CreateWebhookResult{WebhookID: result.WebhookID, SigningSecret: result.SigningSecret}, nil
}

// List lista os webhooks do parceiro, paginados (GET v1/webhook). req nil usa a página 1 com 10 itens.
func (c *WebhooksClient) List(ctx context.Context, req *ListRequest) (*PagedResult[Webhook], error) {
	query, err := req.query()
	if err != nil {
		return nil, err
	}
	var result wire.PagedWebhooks
	if err := c.api.get(ctx, "v1/webhook?"+query, &result); err != nil {
		return nil, err
	}

	items := make([]Webhook, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, Webhook{
			ID:                    item.WebhookID,
			Type:                  webhookTypeFromText(item.Type),
			TypeRaw:               item.Type,
			URL:                   item.URL,
			AuthenticationType:    webhookAuthenticationTypeFromText(item.AuthenticationType),
			AuthenticationTypeRaw: item.AuthenticationType,
			Status:                webhookStatusFromText(item.Status),
			StatusRaw:             item.Status,
		})
	}
	return &PagedResult[Webhook]{
		Items:       items,
		Page:        result.Page.Number,
		PageSize:    result.Page.Size,
		TotalCount:  result.Page.Total,
		HasNext:     result.Page.HasNext,
		HasPrevious: result.Page.HasPrevious,
	}, nil
}

// Update altera o tipo, a URL e o status de um webhook (PUT v1/webhook/{id}, 204). A
// autenticação não pode ser alterada: para trocá-la, exclua o webhook e cadastre outro.
func (c *WebhooksClient) Update(ctx context.Context, webhookID string, req UpdateWebhookRequest) error {
	if err := requireUUID("webhookID", webhookID); err != nil {
		return err
	}
	wireReq := wire.UpdateWebhookRequest{Type: int(req.Type), URL: req.URL}
	if req.Status != nil {
		status := int(*req.Status)
		wireReq.Status = &status
	}
	return c.api.put(ctx, "v1/webhook/"+url.PathEscape(webhookID), wireReq)
}

// Delete exclui um webhook (DELETE v1/webhook/{id}, 204); ele deixa de receber notificações.
func (c *WebhooksClient) Delete(ctx context.Context, webhookID string) error {
	if err := requireUUID("webhookID", webhookID); err != nil {
		return err
	}
	return c.api.delete(ctx, "v1/webhook/"+url.PathEscape(webhookID))
}
