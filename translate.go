package credicarro

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/auth"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/httperr"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/resilience"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/transport"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

// problem é o ProblemDetails já normalizado (campos em português, com fallback para o
// formato padrão type/title/detail).
type problem struct {
	code, title, detail, field string
	errors                     []FieldError
}

func parseProblem(body []byte) problem {
	var p problem
	if len(strings.TrimSpace(string(body))) == 0 {
		return p
	}

	var pt wire.ProblemDetails
	if err := json.Unmarshal(body, &pt); err == nil {
		p.code, p.title, p.detail, p.field = pt.Type, pt.Title, pt.Detail, pt.Field
		for _, item := range pt.Errors {
			p.errors = append(p.errors, FieldError{Code: item.Code, Message: item.Message, Field: item.Field})
		}
	}

	// Falhas inesperadas (500 e alguns 502) vêm no formato padrão type/title/detail.
	var std wire.StdProblemDetails
	if err := json.Unmarshal(body, &std); err == nil {
		if p.code == "" {
			p.code = std.Type
		}
		if p.title == "" {
			p.title = std.Title
		}
		if p.detail == "" {
			p.detail = std.Detail
		}
	}
	return p
}

func apiErrorFromHTTP(resp *httperr.Response) *APIError {
	p := parseProblem(resp.Body)
	return &APIError{
		StatusCode: resp.StatusCode,
		ErrorCode:  p.code,
		Title:      p.title,
		Detail:     p.detail,
		Field:      p.field,
		Errors:     p.errors,
		Body:       string(resp.Body),
	}
}

func exceptionFromHTTPError(resp *httperr.Response) error {
	apiErr := apiErrorFromHTTP(resp)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &AuthenticationError{
			Message:    formatStatusMessage("Autenticação recusada pela API do Credicarro com status", 401, apiErr.ErrorCode, apiErr.Title),
			StatusCode: 401,
			ErrorCode:  apiErr.ErrorCode,
			Title:      apiErr.Title,
			Detail:     apiErr.Detail,
			Field:      apiErr.Field,
			Errors:     apiErr.Errors,
			Body:       apiErr.Body,
		}
	case http.StatusBadRequest:
		return &ValidationError{APIError: apiErr}
	case http.StatusUnprocessableEntity:
		return &BusinessRuleError{APIError: apiErr}
	case http.StatusTooManyRequests:
		return &RateLimitError{APIError: apiErr, RetryAfter: retryAfterFromHeader(resp.HeaderValue("Retry-After"))}
	default:
		return apiErr
	}
}

// authenticationErrorFromTokenFailure converte uma falha da rota de token em AuthenticationError
// (401 credenciais inválidas, 400, rede ou resposta vazia).
func authenticationErrorFromTokenFailure(authErr *auth.Error) error {
	switch authErr.Kind {
	case auth.KindRejected:
		apiErr := apiErrorFromHTTP(authErr.Response)
		if apiErr.StatusCode != http.StatusBadRequest && apiErr.StatusCode != http.StatusUnauthorized {
			return apiErr
		}
		return &AuthenticationError{
			Message:    formatStatusMessage("Geração do token de acesso recusada pela API do Credicarro com status", apiErr.StatusCode, apiErr.ErrorCode, apiErr.Title),
			StatusCode: apiErr.StatusCode,
			ErrorCode:  apiErr.ErrorCode,
			Title:      apiErr.Title,
			Detail:     apiErr.Detail,
			Field:      apiErr.Field,
			Errors:     apiErr.Errors,
			Body:       apiErr.Body,
		}
	case auth.KindNetwork:
		return &TimeoutError{Message: networkFailureMessage, Cause: authErr.Cause}
	default:
		return &AuthenticationError{Message: "A API do Credicarro retornou uma resposta vazia ao gerar o token de acesso."}
	}
}

func retryAfterFromHeader(value string) time.Duration {
	return httperr.ParseRetryAfter(value, time.Now())
}

// translateExecutorError converte os erros de baixo nível (internal/resilience,
// internal/httperr, internal/auth) nos tipos públicos deste pacote.
//
// Cancelamento ou expiração do contexto do chamador é devolvido como está (compatível
// com errors.Is(err, context.Canceled / context.DeadlineExceeded)).
func translateExecutorError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	var httpErr *httperr.Response
	if errors.As(err, &httpErr) {
		return exceptionFromHTTPError(httpErr)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	var authErr *auth.Error
	if errors.As(err, &authErr) {
		return authenticationErrorFromTokenFailure(authErr)
	}

	var exhausted *resilience.TimeoutExceededError
	if errors.As(err, &exhausted) {
		return &TimeoutError{Message: exhausted.Error(), Cause: err}
	}

	var networkErr *transport.NetworkError
	if errors.As(err, &networkErr) {
		return &TimeoutError{Message: networkFailureMessage, Cause: networkErr.Cause}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return &TimeoutError{Message: "A tentativa excedeu o tempo limite configurado.", Cause: err}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &TimeoutError{Message: "A tentativa excedeu o tempo limite configurado.", Cause: err}
	}

	return err
}

const networkFailureMessage = "Falha de rede ou timeout ao chamar a API do Credicarro."
