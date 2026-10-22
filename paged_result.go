package credicarro

import "fmt"

const (
	defaultPage     = 1
	defaultPageSize = 10
	maxPageSize     = 100
)

// PagedResult é o retorno de qualquer listagem paginada. Sem auto-paginação — o
// consumidor avança de página explicitamente via HasNext e ListRequest.Page.
type PagedResult[T any] struct {
	Items       []T
	Page        int
	PageSize    int
	TotalCount  int64
	HasNext     bool
	HasPrevious bool
}

// ListRequest parametriza as listagens (pagina_inicial e limite na API).
// Valores zero usam o padrão: Page = 1 e PageSize = 10. PageSize vai de 1 a 100.
type ListRequest struct {
	Page     int
	PageSize int
}

// query devolve a query string (sem "?") já validada.
func (r *ListRequest) query() (string, error) {
	page, size := defaultPage, defaultPageSize
	if r != nil {
		if r.Page != 0 {
			page = r.Page
		}
		if r.PageSize != 0 {
			size = r.PageSize
		}
	}
	if page < 1 {
		return "", &ConfigurationError{Message: "ListRequest.Page deve ser maior ou igual a 1."}
	}
	if size < 1 || size > maxPageSize {
		return "", &ConfigurationError{Message: fmt.Sprintf("ListRequest.PageSize deve estar entre 1 e %d.", maxPageSize)}
	}
	return fmt.Sprintf("pagina_inicial=%d&limite=%d", page, size), nil
}
