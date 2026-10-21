// sample-http expõe, com net/http puro, uma consulta de faixa de entrada e o endpoint que
// recebe os webhooks de proposta do Credicarro.
//
// Variáveis de ambiente:
//
//	CREDICARRO_CLIENT_ID, CREDICARRO_CLIENT_SECRET  credenciais da integração (staging)
//	CREDICARRO_WEBHOOK_SECRET                       (obrigatório) segredo_assinatura devolvido no cadastro do webhook
//	CREDICARRO_WEBHOOK_AUTHORIZATION                (opcional) valor esperado do cabeçalho Authorization
//	                                                ("Basic ..." ou "Bearer ..."), conforme o cadastro do webhook
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

// maxWebhookBodyBytes limita o corpo aceito no endpoint de webhook (1 MiB).
const maxWebhookBodyBytes = 1 << 20

type webhookJob struct {
	ProposalID     string
	IdempotencyKey string
}

type server struct {
	client               *credicarro.Client
	webhookSecret        string
	webhookAuthorization string
	jobs                 chan webhookJob
	seen                 sync.Map // deduplicação em memória; em produção use Redis ou banco com expiração
}

func main() {
	client, err := credicarro.NewClient(credicarro.ClientOptions{
		ClientID:     os.Getenv("CREDICARRO_CLIENT_ID"),
		ClientSecret: os.Getenv("CREDICARRO_CLIENT_SECRET"),
		Environment:  credicarro.EnvironmentStaging,
	})
	if err != nil {
		log.Fatal(err)
	}

	s := &server{
		client:               client,
		webhookSecret:        os.Getenv("CREDICARRO_WEBHOOK_SECRET"),
		webhookAuthorization: os.Getenv("CREDICARRO_WEBHOOK_AUTHORIZATION"),
		jobs:                 make(chan webhookJob, 1000),
	}
	if s.webhookSecret == "" {
		log.Fatal("CREDICARRO_WEBHOOK_SECRET é obrigatório")
	}
	go s.runWorker()

	mux := http.NewServeMux()
	mux.HandleFunc("/faixa", s.handleRange)
	mux.HandleFunc("/webhooks/credicarro", s.handleWebhook)

	httpServer := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Println("Ouvindo em :8080")
	log.Fatal(httpServer.ListenAndServe())
}

// handleRange: GET /faixa?placa=ABC1D23&cnpj=11222333000181
func (s *server) handleRange(w http.ResponseWriter, r *http.Request) {
	options, err := s.client.Simulations.GetFinancingRange(r.Context(), r.URL.Query().Get("placa"), r.URL.Query().Get("cnpj"))
	if err != nil {
		status := http.StatusBadGateway
		var rule *credicarro.BusinessRuleError
		var validation *credicarro.ValidationError
		switch {
		case errors.As(err, &rule):
			status = http.StatusUnprocessableEntity
		case errors.As(err, &validation):
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"erro": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, options)
}

// handleWebhook valida, deduplica, enfileira e só então responde 2xx (limite de entrega de 15 s).
func (s *server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Authorization (Basic/Jwt) cadastrado no webhook, comparado em tempo constante.
	if s.webhookAuthorization != "" && !credicarro.VerifyWebhookAuthorization(r.Header.Get("Authorization"), s.webhookAuthorization) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 2. Corpo bruto, limitado a 1 MiB, antes de qualquer desserialização.
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			http.Error(w, "corpo grande demais", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "corpo ilegível", http.StatusBadRequest)
		return
	}

	// 3. Assinatura HMAC (X-CrediCarro-Signature) com tolerância de 5 minutos (0 = padrão) e decodificação.
	event, err := credicarro.ParseWebhookEvent(rawBody, r.Header.Get("X-CrediCarro-Signature"), s.webhookSecret, 0)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 4. Deduplicação pelo Idempotency-Key ({id_mensagem}:{id_webhook}, igual em todas as retentativas).
	key := r.Header.Get("Idempotency-Key")
	if key != "" {
		if _, duplicate := s.seen.LoadOrStore(key, struct{}{}); duplicate {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// 5. Fila cheia: 503 na hora, desfazendo a deduplicação para a retentativa da Credicarro.
	select {
	case s.jobs <- webhookJob{ProposalID: event.ProposalID, IdempotencyKey: key}:
		w.WriteHeader(http.StatusOK)
	default:
		if key != "" {
			s.seen.Delete(key)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

// runWorker consulta a proposta (as entregas chegam fora de ordem) e lista o que falta do integrador.
func (s *server) runWorker() {
	for job := range s.jobs {
		s.process(job)
	}
}

func (s *server) process(job webhookJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("webhook | Proposta: %s | Pânico: %T", job.ProposalID, r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	proposal, err := s.client.Proposals.Get(ctx, job.ProposalID)
	if err != nil {
		log.Printf("webhook | Proposta: %s | Erro: %T", job.ProposalID, err) // sem payload nem dados pessoais
		return
	}
	for _, action := range proposal.PendingActions() {
		log.Printf("webhook | Proposta: %s | %s -> %s", proposal.ID, action.Participant, action.Kind)
	}
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
