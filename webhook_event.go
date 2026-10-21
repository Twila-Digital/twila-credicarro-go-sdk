package credicarro

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

// DefaultWebhookTolerance é a tolerância padrão (nos dois sentidos) entre o timestamp
// da assinatura e o relógio local.
const DefaultWebhookTolerance = 5 * time.Minute

var signatureTimestampPattern = regexp.MustCompile(`^[0-9]{1,10}$`)

// ProposalWebhookEvent é a notificação de proposta. Aceita os dois formatos de payload:
// o v1 (id_proposta, enum_status e status em inglês) e o v2 (retrato completo da proposta,
// com nomes em português). IsDetailed indica o payload v2.
type ProposalWebhookEvent struct {
	ProposalID string
	// Status vem de enum_status (ou, na falta dele, do nome); StatusRaw é o código bruto
	// (0 quando ausente) e StatusName o nome recebido (ex.: APROVADA ou Approved).
	Status     ProposalStatus
	StatusRaw  int
	StatusName string
	// EventReason é nil quando o payload não traz motivo_evento (payload v1).
	EventReason    *ProposalEventReason
	EventReasonRaw string
	RefusalReason  string
	// EventDate é o momento em que o retrato foi montado (ISO-8601, como veio); vazio no payload v1.
	EventDate string
	Buyer     *ProposalParticipantState
	Seller    *ProposalParticipantState
	Store     *ProposalParticipantState
	// IsDetailed é true no payload v2 (retrato completo da proposta).
	IsDetailed bool
}

// PendingActions lista o que o evento detalhado (payload v2) espera do integrador, com as
// mesmas regras de ProposalDetails.PendingActions. No payload v1 não há participantes:
// consulte a proposta com Proposals.Get.
func (e *ProposalWebhookEvent) PendingActions() []PendingAction {
	if e == nil {
		return nil
	}
	details := ProposalDetails{Buyer: e.Buyer, Seller: e.Seller, Store: e.Store}
	return details.PendingActions()
}

// ParseWebhookEvent verifica a assinatura X-CrediCarro-Signature e decodifica o corpo
// bruto do webhook de proposta. rawBody precisa ser o corpo exatamente como recebido, antes
// de qualquer desserialização. tolerance zero usa DefaultWebhookTolerance.
//
// Segredo vazio, cabeçalho ausente/malformado, assinatura divergente ou timestamp fora da
// tolerância devolvem *WebhookSignatureError.
func ParseWebhookEvent(rawBody []byte, signatureHeader, signingSecret string, tolerance time.Duration) (*ProposalWebhookEvent, error) {
	if err := verifySignatureAt(time.Now(), rawBody, signatureHeader, signingSecret, tolerance); err != nil {
		return nil, err
	}
	return ParseWebhookEventUnverified(rawBody)
}

// ParseWebhookEventUnverified decodifica o corpo SEM verificar a assinatura. Use apenas
// em testes; em produção use ParseWebhookEvent.
func ParseWebhookEventUnverified(rawBody []byte) (*ProposalWebhookEvent, error) {
	var w wire.ProposalWebhookEvent
	if err := json.Unmarshal(rawBody, &w); err != nil {
		return nil, &WebhookSignatureError{Message: "O corpo do webhook está vazio ou não é um JSON válido."}
	}
	if !uuidPattern.MatchString(w.ProposalID) {
		return nil, &WebhookSignatureError{Message: "O corpo do webhook não contém um id_proposta válido."}
	}
	return eventFromWire(w), nil
}

// VerifyWebhookSignature informa se a assinatura é válida. Veja ParseWebhookEvent.
func VerifyWebhookSignature(rawBody []byte, signatureHeader, signingSecret string, tolerance time.Duration) bool {
	return verifySignatureAt(time.Now(), rawBody, signatureHeader, signingSecret, tolerance) == nil
}

// VerifyWebhookAuthorization compara, em tempo constante, o cabeçalho Authorization
// recebido com o esperado (Basic ou Bearer, conforme o cadastro do webhook). Um valor
// esperado vazio nunca é aceito.
func VerifyWebhookAuthorization(receivedHeader, expectedHeader string) bool {
	if expectedHeader == "" {
		return false
	}
	received := sha256.Sum256([]byte(receivedHeader))
	expected := sha256.Sum256([]byte(expectedHeader))
	return subtle.ConstantTimeCompare(received[:], expected[:]) == 1
}

// ComputeWebhookSignature calcula o v1 da assinatura: HMAC-SHA256 em hexadecimal
// minúsculo de "{timestamp}.{corpo}", com o segredo em UTF-8.
func ComputeWebhookSignature(signingSecret string, timestampSeconds int64, payload []byte) string {
	return computeSignature(signingSecret, strconv.FormatInt(timestampSeconds, 10), payload)
}

func computeSignature(signingSecret, rawTimestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(rawTimestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func verifySignatureAt(now time.Time, rawBody []byte, signatureHeader, signingSecret string, tolerance time.Duration) error {
	if signingSecret == "" {
		return &WebhookSignatureError{Message: "O segredo de assinatura do webhook não foi informado."}
	}
	if tolerance < 0 {
		return &WebhookSignatureError{Message: "A tolerância do webhook não pode ser negativa."}
	}
	if tolerance == 0 {
		tolerance = DefaultWebhookTolerance
	}

	rawTimestamp, signatures, ok := parseSignatureHeader(signatureHeader)
	if !ok {
		return &WebhookSignatureError{Message: "O cabeçalho de assinatura do webhook está ausente ou malformado."}
	}

	expected := []byte(computeSignature(signingSecret, rawTimestamp, rawBody))
	matched := 0
	for _, candidate := range signatures {
		matched |= subtle.ConstantTimeCompare(expected, []byte(strings.ToLower(candidate)))
	}
	if matched != 1 {
		return &WebhookSignatureError{Message: "A assinatura do webhook não confere."}
	}

	timestamp, err := strconv.ParseInt(rawTimestamp, 10, 64)
	if err != nil {
		return &WebhookSignatureError{Message: "O cabeçalho de assinatura do webhook está ausente ou malformado."}
	}
	diff := now.Unix() - timestamp
	if diff < 0 {
		diff = -diff
	}
	if diff > int64(tolerance/time.Second) {
		return &WebhookSignatureError{Message: "O timestamp do webhook está fora da janela de tolerância — possível replay."}
	}
	return nil
}

// parseSignatureHeader lê "t={1..10 dígitos},v1={hex}" e devolve o t bruto e os v1.
func parseSignatureHeader(header string) (string, []string, bool) {
	var timestamp string
	var signatures []string

	for _, part := range strings.Split(header, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "t":
			if timestamp != "" {
				return "", nil, false
			}
			timestamp = strings.TrimSpace(value)
		case "v1":
			if len(signatures) > 0 {
				return "", nil, false
			}
			signatures = append(signatures, strings.TrimSpace(value))
		}
	}

	if !signatureTimestampPattern.MatchString(timestamp) || len(signatures) == 0 {
		return "", nil, false
	}
	return timestamp, signatures, true
}

func eventFromWire(w wire.ProposalWebhookEvent) *ProposalWebhookEvent {
	event := &ProposalWebhookEvent{
		ProposalID:     w.ProposalID,
		StatusName:     w.StatusName,
		EventReasonRaw: w.EventReason,
		RefusalReason:  w.RefusalReason,
		EventDate:      w.EventDate,
		Buyer:          participantFromWire(w.Buyer),
		Seller:         participantFromWire(w.Seller),
		Store:          storeFromWire(w.Store),
	}

	event.Status = ProposalStatusUnknown
	if w.Status != nil {
		event.StatusRaw = *w.Status
		event.Status = proposalStatusFromInt(*w.Status)
	}
	if event.Status == ProposalStatusUnknown {
		if byName := proposalStatusFromName(w.StatusName); byName != ProposalStatusUnknown {
			event.Status = byName
		}
	}

	if w.EventReason != "" {
		reason := proposalEventReasonFromName(w.EventReason)
		event.EventReason = &reason
	}

	event.IsDetailed = w.EventReason != "" || w.EventDate != "" || w.Buyer != nil || w.Seller != nil || w.Store != nil
	return event
}
