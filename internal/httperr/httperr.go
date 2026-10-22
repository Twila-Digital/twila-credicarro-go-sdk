// Package httperr representa uma resposta HTTP não-2xx como error, sem depender de
// tipos do pacote raiz — usado pelos pacotes internos (auth, transport) pra propagar
// o status/corpo da resposta pra cima, onde o pacote raiz reconstrói (via errors.As)
// o erro tipado público correto (AuthenticationError, ValidationError, etc.).
package httperr

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Response carrega status, corpo e cabeçalhos de uma resposta de erro. O corpo pode
// conter dados pessoais — por isso Error() nunca o inclui.
type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

func (r *Response) Error() string {
	return fmt.Sprintf("credicarro: http status %d", r.StatusCode)
}

// HeaderValue devolve o primeiro valor do cabeçalho name (case-insensitive) ou "".
func (r *Response) HeaderValue(name string) string {
	for k, v := range r.Header {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

// MaxRetryAfter limita o valor aceito do cabeçalho Retry-After (1 ano). Valores maiores
// são reduzidos a ele antes de qualquer conversão para time.Duration, evitando estouro.
const MaxRetryAfter = 365 * 24 * time.Hour

// ParseRetryAfter lê o cabeçalho Retry-After (segundos ou data HTTP). Valores ausentes,
// inválidos, não finitos ou menores/iguais a zero resultam em 0; valores absurdos são
// limitados a MaxRetryAfter.
func ParseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		if math.IsNaN(seconds) || seconds <= 0 {
			return 0
		}
		if math.IsInf(seconds, 1) || seconds >= MaxRetryAfter.Seconds() {
			return MaxRetryAfter
		}
		return time.Duration(seconds * float64(time.Second))
	}

	if when, err := http.ParseTime(value); err == nil {
		d := when.Sub(now)
		if d <= 0 {
			return 0
		}
		if d > MaxRetryAfter {
			return MaxRetryAfter
		}
		return d
	}
	return 0
}
