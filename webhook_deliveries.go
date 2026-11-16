package credicarro

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/transport"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

// resendAttemptTimeout cobre a espera pela URL do integrador no reenvio (até 15 s do lado da Credicarro).
const resendAttemptTimeout = 30 * time.Second

// WebhookDeliveryFilter filtra o histórico e o gráfico de envios. Campos vazios (ou nil) não
// são enviados. As datas são texto ISO-8601: uma data sem horário ("2026-11-05") vale pelo dia
// inteiro; com horário, informe o fuso (ex.: "2026-11-05T14:00:00-03:00").
type WebhookDeliveryFilter struct {
	InitialDate    string // data_inicial
	FinalDate      string // data_final
	ProposalID     string // id_proposta
	SimulationID   string // id_simulacao
	ProposalNumber string // numero_proposta, até 50 caracteres
	StatusCode     *int   // status_http devolvido pela sua URL (100 a 599)
}

// ListWebhookDeliveriesRequest parametriza o histórico de envios: filtros opcionais e
// paginação (zero usa a página 1 com 10 itens).
type ListWebhookDeliveriesRequest struct {
	WebhookDeliveryFilter
	Page     int
	PageSize int
}

// WebhookDeliveryChartRequest parametriza o gráfico de envios. InitialDate e FinalDate são
// obrigatórios, e o período pode ter até 31 dias.
type WebhookDeliveryChartRequest struct {
	WebhookDeliveryFilter
}

// WebhookDelivery é um envio de webhook registrado no histórico. Use ID (id_auditoria) em
// Resend. StatusCode é nil quando a sua URL não respondeu (ex.: timeout).
type WebhookDelivery struct {
	ID         string
	WebhookID  string
	Type       WebhookType
	TypeRaw    string
	Request    string // corpo enviado, como texto JSON
	Response   string // corpo devolvido pela sua URL; vazio quando não houve resposta
	StatusCode *int
	CreatedAt  string
}

// String não inclui os corpos (podem conter dados pessoais).
func (d WebhookDelivery) String() string {
	return "WebhookDelivery{ID: " + strconv.Quote(d.ID) + ", Type: " + strconv.Quote(d.TypeRaw) +
		", StatusCode: " + optionalStatus(d.StatusCode) + ", Request: [REDACTED], Response: [REDACTED]}"
}

// GoString não inclui os corpos (verbo %#v).
func (d WebhookDelivery) GoString() string { return d.String() }

// WebhookDeliveryChartPoint é a contagem de envios em um intervalo do gráfico, no horário de
// Brasília. Label é "10m" (intervalos de 10 minutos), "14h" (hora) ou "05" (dia), conforme o período.
type WebhookDeliveryChartPoint struct {
	Label        string
	SuccessCount int
	ErrorCount   int
}

// WebhookResendResult é o resultado imediato do reenvio. StatusCode é nil quando a sua URL
// não respondeu.
type WebhookResendResult struct {
	StatusCode *int
	Response   string
}

// ListDeliveries consulta o histórico de envios, com o corpo enviado, a resposta da sua URL e
// o status HTTP (GET v1/webhook/audit). req nil lista a página 1 sem filtros.
func (c *WebhooksClient) ListDeliveries(ctx context.Context, req *ListWebhookDeliveriesRequest) (*PagedResult[WebhookDelivery], error) {
	var filter WebhookDeliveryFilter
	var paging *ListRequest
	if req != nil {
		filter = req.WebhookDeliveryFilter
		paging = &ListRequest{Page: req.Page, PageSize: req.PageSize}
	}
	pageQuery, err := paging.query()
	if err != nil {
		return nil, err
	}
	path := "v1/webhook/audit?" + pageQuery
	if filterQuery := filter.values().Encode(); filterQuery != "" {
		path += "&" + filterQuery
	}

	var result wire.PagedWebhookDeliveries
	if err := c.api.get(ctx, path, &result); err != nil {
		return nil, err
	}
	items := make([]WebhookDelivery, 0, len(result.Items))
	for _, item := range result.Items {
		delivery := WebhookDelivery{
			ID:         item.AuditID,
			Type:       webhookTypeFromText(item.Type),
			TypeRaw:    item.Type,
			Request:    item.Request,
			Response:   item.Response,
			StatusCode: item.StatusCode,
			CreatedAt:  item.CreatedAt,
		}
		if item.WebhookID != nil {
			delivery.WebhookID = *item.WebhookID
		}
		items = append(items, delivery)
	}
	return &PagedResult[WebhookDelivery]{
		Items:       items,
		Page:        result.Page.Number,
		PageSize:    result.Page.Size,
		TotalCount:  result.Page.Total,
		HasNext:     result.Page.HasNext,
		HasPrevious: result.Page.HasPrevious,
	}, nil
}

// GetDeliveryChart conta os envios com sucesso e com erro por intervalo, no horário de
// Brasília (GET v1/webhook/audit/chart). O período é obrigatório e pode ter até 31 dias.
func (c *WebhooksClient) GetDeliveryChart(ctx context.Context, req WebhookDeliveryChartRequest) ([]WebhookDeliveryChartPoint, error) {
	if err := requireNonEmpty("InitialDate", req.InitialDate); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("FinalDate", req.FinalDate); err != nil {
		return nil, err
	}

	var result []wire.WebhookDeliveryChartPoint
	if err := c.api.get(ctx, "v1/webhook/audit/chart?"+req.values().Encode(), &result); err != nil {
		return nil, err
	}
	points := make([]WebhookDeliveryChartPoint, 0, len(result))
	for _, point := range result {
		points = append(points, WebhookDeliveryChartPoint{Label: point.Label, SuccessCount: point.SuccessCount, ErrorCount: point.ErrorCount})
	}
	return points, nil
}

// Resend reenvia para a sua URL o mesmo corpo de um envio do histórico
// (POST v1/webhook/audit/{id}/resend). É uma única tentativa, NÃO repetida em falhas
// transitórias (repetir dispararia o webhook de novo). O reenvio chega com uma
// Idempotency-Key nova ({id_mensagem}:{id_webhook}:{id_tentativa}).
func (c *WebhooksClient) Resend(ctx context.Context, deliveryID string) (*WebhookResendResult, error) {
	if err := requireUUID("deliveryID", deliveryID); err != nil {
		return nil, err
	}
	var result wire.WebhookResendResponse
	path := "v1/webhook/audit/" + url.PathEscape(deliveryID) + "/resend"
	if err := c.api.post(ctx, path, nil, &result, transport.RequestOptions{AttemptTimeout: resendAttemptTimeout}); err != nil {
		return nil, err
	}
	return &WebhookResendResult{StatusCode: result.StatusCode, Response: result.Response}, nil
}

func (f WebhookDeliveryFilter) values() url.Values {
	q := url.Values{}
	setIfNotEmpty(q, "data_inicial", f.InitialDate)
	setIfNotEmpty(q, "data_final", f.FinalDate)
	setIfNotEmpty(q, "id_proposta", f.ProposalID)
	setIfNotEmpty(q, "id_simulacao", f.SimulationID)
	setIfNotEmpty(q, "numero_proposta", f.ProposalNumber)
	if f.StatusCode != nil {
		q.Set("status_http", strconv.Itoa(*f.StatusCode))
	}
	return q
}

func setIfNotEmpty(q url.Values, name, value string) {
	if value != "" {
		q.Set(name, value)
	}
}

func optionalStatus(status *int) string {
	if status == nil {
		return "nil"
	}
	return strconv.Itoa(*status)
}
