package credicarro

// Environment seleciona o ambiente padrão da API do Credicarro quando BaseURL não é informado.
type Environment string

const (
	// EnvironmentStaging é o ambiente de testes.
	EnvironmentStaging Environment = "staging"
	// EnvironmentProduction é o ambiente de produção (padrão).
	EnvironmentProduction Environment = "production"
)

var environmentBaseURLs = map[Environment]string{
	EnvironmentStaging:    "https://api.staging.credicarro.com.br/integration/",
	EnvironmentProduction: "https://api.credicarro.com.br/integration/",
}
