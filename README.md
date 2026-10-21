<p align="center">
  <img src="https://raw.githubusercontent.com/Twila-Digital/twila-credicarro-go-sdk/production/assets/logo-light.svg" alt="Credicarro" width="180" style="max-width: 100%;">
</p>

<p align="center">
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/Twila-Digital/twila-credicarro-go-sdk"></a>
  <a href="https://github.com/Twila-Digital/twila-credicarro-go-sdk/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/Twila-Digital/twila-credicarro-go-sdk/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/Twila-Digital/twila-credicarro-go-sdk/actions/workflows/quality.yml"><img alt="Quality" src="https://github.com/Twila-Digital/twila-credicarro-go-sdk/actions/workflows/quality.yml/badge.svg"></a>
  <a href="https://github.com/Twila-Digital/twila-credicarro-go-sdk/security/code-scanning"><img alt="Security" src="https://github.com/Twila-Digital/twila-credicarro-go-sdk/actions/workflows/security.yml/badge.svg"></a>
  <a href="https://codecov.io/gh/Twila-Digital/twila-credicarro-go-sdk"><img alt="Coverage" src="https://codecov.io/gh/Twila-Digital/twila-credicarro-go-sdk/branch/production/graph/badge.svg"></a>
  <a href="https://pkg.go.dev/github.com/Twila-Digital/twila-credicarro-go-sdk"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/Twila-Digital/twila-credicarro-go-sdk.svg"></a>
</p>

# twila-credicarro-go-sdk

SDK oficial em Go para a API de integração do [Credicarro](https://documentacao.credicarro.com.br) — financiamento e refinanciamento de veículos: cadastro de clientes, simulações, propostas, envio de documentos e webhooks.

> Uso restrito a server-side. O `ClientSecret` nunca deve ser embarcado em um app mobile, frontend ou qualquer código que rode no navegador/dispositivo do usuário final.

## Compatibilidade

| Runtime | Versões aceitas |
| --- | --- |
| Go | 1.18 ou superior (CI cobre 1.18, 1.20, 1.21, 1.22, 1.23, 1.24, 1.25, 1.26 e 1.27) |

**Zero dependências externas** — só biblioteca padrão (`net/http`, `encoding/json`, `crypto/hmac`, `testing`). Seguro para uso concorrente por múltiplas goroutines.

## Instalação

```bash
go get github.com/Twila-Digital/twila-credicarro-go-sdk
```

## Configuração

```go
import credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"

client, err := credicarro.NewClient(credicarro.ClientOptions{
    ClientID:     "<seu-client-id>",
    ClientSecret: "<seu-client-secret>",
    Environment:  credicarro.EnvironmentStaging, // padrão: EnvironmentProduction
})
```

| Ambiente | URL base |
| --- | --- |
| `EnvironmentStaging` | `https://api.staging.credicarro.com.br/integration/` |
| `EnvironmentProduction` (padrão) | `https://api.credicarro.com.br/integration/` |

`BaseURL` sobrescreve o ambiente e precisa ser `https` (`http` só é aceito para `localhost` e `127.0.0.1`, para testes). O `ClientSecret` nunca aparece em `String()`, `%v`, `%+v`, `%#v` nem em mensagens de erro.

`ClientOptions.HTTPClient` é opcional (proxy, transporte próprio): o SDK usa uma cópia dele e **sempre desliga os redirects** — um `3xx` vira `*APIError` e o token nunca é enviado a outro destino. Sem ele, o SDK usa um transporte próprio (TLS 1.2+, 64 conexões ociosas por host).

`*credicarro.Client` deve ser reaproveitado como singleton na sua aplicação (não crie uma instância por requisição) — ele mantém o cache do token de acesso e o estado do circuit breaker, e é seguro para uso concorrente.

## Fluxo de uma integração

```go
ctx := context.Background()

// 1. Cadastro completo do cliente (sem Idempotency-Key: cria ou atualiza)
_, err = client.Customers.Upsert(ctx, credicarro.UpsertCustomerRequest{
    Cpf:         "12345678909",
    FirstName:   "Maria",
    LastName:    "Souza",
    Email:       "maria.souza@exemplo.com.br",
    PhoneNumber: "+55 (11) 99876-5432",
    DateOfBirth: "1990-05-20T00:00:00-03:00",
    Address: credicarro.CustomerAddress{
        Street: "Av. Paulista", Number: "1578", Neighborhood: "Bela Vista",
        City: "São Paulo", State: "SP", PostalCode: "01311000",
    },
    ProfessionalData: credicarro.ProfessionalData{Occupation: credicarro.OccupationTypeEmployee, MonthlyIncome: 6500},
})

// 2. Simulação (uma opção por prazo)
options, err := client.Simulations.CreateFinancing(ctx, credicarro.CreateFinancingSimulationRequest{
    Cpf: "12345678909", LicensePlate: "ABC1D23", VehicleAmount: 45000, DownPayment: 5000,
    State: "SP", IsZeroKm: false, StoreCnpj: "11222333000181",
})

// 3. Proposta a partir da opção escolhida
proposalID, err := client.Proposals.Create(ctx, credicarro.CreateProposalRequest{
    Cpf: "12345678909", SimulationID: options[0].SimulationID,
})

// 4. Acompanhamento: o que a proposta espera do seu sistema?
proposal, err := client.Proposals.Get(ctx, proposalID)
for _, action := range proposal.PendingActions() {
    fmt.Println(action.Participant, action.Kind, action.DocumentTypes)
}
```

O client expõe um struct por recurso:

| Cliente | Métodos |
| --- | --- |
| `client.Customers` | `Upsert`, `Create`, `UpsertAddress`, `UpsertProfessionalData` |
| `client.Simulations` | `GetFinancingRange`, `GetRefinancingRange`, `CreateFinancing`, `CreateRefinancing`, `Get`, `List` |
| `client.Proposals` | `Create`, `Get`, `List`, `SendVehicleDetails`, `SendDepositAccount`, `UploadDocument` |
| `client.Webhooks` | `Create`, `List`, `Update`, `Delete` |
| `client.V1.Simulations` | `GetFinancingRange`, `GetRefinancingRange`, `CreateFinancing`, `CreateRefinancing` (rotas v1, sem `cnpjLoja`) |
| `client.V1.Proposals` | `Create`, `Get` (rotas v1) |

Todo método recebe `context.Context` como primeiro argumento (idiomático em Go, permite cancelamento/timeout por chamada além do timeout total configurado no client). Os identificadores (`SimulationID`, ids de proposta e de webhook) são UUIDs em `string` e são validados antes de qualquer chamada.

### O que a proposta espera de você: `PendingActions`

`ProposalDetails.PendingActions()` devolve só as etapas ativas em que o responsável é o integrador (`ActionOwnerIntegrator`):

| Etapa | Ação (`Kind`) | `DocumentTypes` |
| --- | --- | --- |
| `ENVIO_DOCUMENTOS` (6) | `PendingActionSendDocuments` | documentos com status `Pending` |
| `CORRECAO_DOCUMENTOS` (7) | `PendingActionCorrectDocuments` | documentos com status `Invalid` |
| `DADOS_VEICULO` (8) | `PendingActionSendVehicleDetails` | — |
| `CONTA_RECEBIMENTO` (5) | `PendingActionSendDepositAccount` | — |

## Idempotência e retentativas

O SDK envia a `Idempotency-Key` automaticamente — **uma por chamada lógica, a mesma em todas as tentativas** — em `Customers.Create`, `UpsertAddress`, `UpsertProfessionalData`, nas criações de simulação e proposta (v1 e v2) e em `Proposals.UploadDocument`. Para usar a sua própria chave (1 a 200 caracteres ASCII imprimíveis; chave vazia ou inválida é `*ConfigurationError`; recomendado quando você persiste uma por operação):

```go
id, err := client.Proposals.Create(ctx, req, credicarro.WithIdempotencyKey("minha-chave-por-operacao"))
```

Regras que valem a pena conhecer:

- **`Customers.Upsert` nunca recebe chave automática.** Sem chave, `POST /v1/customer/unified` é um upsert (cria ou atualiza); com chave, só cria (CPF já cadastrado => `409 Cpf.CpfAlreadyAdded`). Passe `WithIdempotencyKey` apenas se quiser o comportamento "só cria". Sem chave, a chamada **não é repetida** automaticamente.
- **`Webhooks.Create` não aceita chave e não é repetido**, para não duplicar webhooks. Se der timeout, liste os webhooks antes de tentar de novo.
- **`Proposals.UploadDocument`**: a API exige a chave. Persista uma por envio e reuse a mesma ao repetir esse envio; use uma chave **nova** ao reenviar a correção de um documento recusado.
- São repetidas automaticamente: `GET`, a geração de token, `POST` com `Idempotency-Key`, `SendVehicleDetails` e `SendDepositAccount` (substituição), `Webhooks.Update` e `Webhooks.Delete`.
- `Retry-After` (segundos ou data HTTP) é respeitado; inválido ou absurdo é ignorado, e se for maior que o orçamento restante o SDK não espera e devolve já o `*RateLimitError` com `RetryAfter`.
- Status repetidos: `408`, `429` (respeitando `Retry-After`), `500` (`RetryOn500`, padrão `true`), `502`, `503`, `504`, falha de rede e timeout de tentativa. Outros `4xx` nunca são repetidos.

### Resiliência

```go
resilience := credicarro.DefaultResilienceOptions()
resilience.MaxRetryAttempts = 4

client, err := credicarro.NewClient(credicarro.ClientOptions{
    ClientID: id, ClientSecret: secret, Resilience: &resilience,
})
```

| Campo | Padrão |
| --- | --- |
| `TotalTimeout` | 30 s |
| `AttemptTimeout` | 10 s |
| `DocumentUploadAttemptTimeout` | 60 s (orçamento total do envio: 60 s × `MaxRetryAttempts` = 180 s) |
| `MaxRetryAttempts` | 3 |
| `RetryBaseDelay` | 500 ms (backoff exponencial com jitter) |
| Circuit breaker | taxa de falha 0,5, janela de 30 s, mínimo de 10 chamadas, pausa de 15 s |
| `RetryOn500` | `true` |
| `DisableAutomaticIdempotencyKey` | `false` |

## Autenticação

O token é obtido em `POST /v1/authentication/accesstoken`, mantido em cache até `expira_em` (ou `expira_em_segundos` menos 60 s) e renovado automaticamente. A renovação é *single-flight*: chamadas concorrentes compartilham uma única requisição de token, e cada uma respeita o próprio `context.Context` enquanto espera. Se uma rota responder `401`, o SDK renova o token e repete a chamada **uma** vez.

## Paginação

`Simulations.List`, `Proposals.List` e `Webhooks.List` retornam `*PagedResult[T]` (genérico) — sem auto-paginação, você controla o avanço de página. Passe `nil` para a primeira página com 10 itens, ou um `*ListRequest` (`Page` ≥ 1, `PageSize` de 1 a 100):

```go
page, err := client.Proposals.List(ctx, &credicarro.ListRequest{Page: 1, PageSize: 20})
for _, proposal := range page.Items {
    fmt.Println(proposal.ID, proposal.StatusName)
}
if page.HasNext {
    next, err := client.Proposals.List(ctx, &credicarro.ListRequest{Page: page.Page + 1, PageSize: 20})
}
```

## Envio de documentos

```go
file, err := credicarro.NewDocumentFileFromFile("/caminho/cnh.pdf", "") // ou ...FromBytes / ...FromReader
if err != nil { /* *ConfigurationError: nome, extensão ou tamanho inválidos */ }

err = client.Proposals.UploadDocument(ctx, proposalID, credicarro.ProposalDocumentUpload{
    Participant:  credicarro.ProposalParticipantBuyer,
    DocumentType: credicarro.DocumentTypeCnh,
    File:         file,
}, credicarro.WithIdempotencyKey(chaveDoEnvio))
```

`DocumentFile` valida o nome (sem caminho, até 255 caracteres, extensão `.pdf`, `.jpg`, `.jpeg` ou `.png`) e o conteúdo (1 byte a 10 MiB) e o codifica em Base64 puro, sem prefixo `data:`. A extensão precisa corresponder ao conteúdo real do arquivo.

## Logs e dados pessoais

`String()`, `GoString()` e `%v`/`%+v`/`%#v` dos modelos com CPF, e-mail, celular, renda, conta bancária, credencial, segredo ou conteúdo de documento mostram valores mascarados (CPF `***.456.789-**`, demais `***`). `Detail` e `Body` dos erros ficam só nos campos, fora da mensagem e do `%#v`. A máscara vale só para depuração/log (`String()`, `GoString()`, `%v`, `%+v`, `%#v`). A serialização (`json.Marshal`) dos modelos de dados traz os **valores reais**, para persistir em fila ou cache (`DocumentFile` inclusive, com round-trip validado). `ClientOptions` **recusa** `json.Marshal`, por guardar o `ClientSecret`.

## Enums

Os enums de resposta (`ProposalStatus`, `ProposalStep`, `StepSituation`, `ActionOwner`, `BiometryStatus`, `DocumentStatus`, `DocumentType`, `ProposalType`, `SimulationType`, `ProposalEventReason`, `WebhookType`, `WebhookAuthenticationType`, `WebhookStatus`) têm o fallback `...Unknown` (`-1`): um valor novo da API nunca gera erro, e o valor original fica nos campos `...Raw` e `...Name`.

## Tratamento de erros

Segue o idioma Go — erros são valores retornados. Use `errors.As` para verificar o tipo:

| Erro | Quando |
| --- | --- |
| `*credicarro.ConfigurationError` | Configuração ou argumento inválido detectado no cliente (ex.: `ClientID` ausente, `ListRequest` fora do intervalo, `DocumentFile` inválido) |
| `*credicarro.AuthenticationError` | Falha ao gerar o token com `400` ou `401` (`Integration.Authentication.InvalidCredentials`) ou `401` que persiste após renovar. Outros status do endpoint de token (3xx, 5xx...) vêm como `*APIError`; falha de rede, como `*TimeoutError` |
| `*credicarro.ValidationError` | `400` |
| `*credicarro.BusinessRuleError` | `422` — regra de negócio; trate pelo `ErrorCode` |
| `*credicarro.RateLimitError` | `429`, com `RetryAfter` |
| `*credicarro.APIError` | Qualquer outro erro de API (`403`, `404`, `409`, `5xx`) |
| `*credicarro.TimeoutError` | Falha de rede ou timeout (causa sanitizada, sem URL nem cabeçalhos), timeout total ou circuit breaker aberto |
| `*credicarro.WebhookSignatureError` | Assinatura de webhook inválida, expirada, ou payload inválido |

`ValidationError`, `BusinessRuleError` e `RateLimitError` embutem `*APIError` e também respondem a `errors.As(err, &apiErr)`.

```go
_, err := client.Proposals.Create(ctx, req)

var rule *credicarro.BusinessRuleError
switch {
case errors.As(err, &rule) && rule.ErrorCode == "Proposal.SimulationAlreadyExpired":
    // refazer a simulação
case errors.As(err, &rule):
    log.Printf("regra não atendida: %s", rule.ErrorCode)
}
```

Os campos expostos são `StatusCode`, `ErrorCode` (o `tipo` estável, ex.: `Store.Integration.NotFound`), `Title`, `Detail`, `Field`, `Errors` e `Body`. **A mensagem do erro traz só o status, o `tipo` e o `titulo`**: `Detail` e `Body` podem conter dados pessoais — não os registre em log. Quando a API responde no formato padrão (`type`/`title`/`detail`, como em falhas `500`), o SDK usa esses campos como fallback; um `401` sem corpo vira `AuthenticationError`.

Cancelamento e expiração do `context.Context` do chamador são devolvidos como estão (`errors.Is(err, context.Canceled)`), nunca como `TimeoutError`.

## Webhooks

Cadastre o webhook e **guarde o `SigningSecret`**, que só é devolvido nessa resposta:

```go
result, err := client.Webhooks.Create(ctx, credicarro.CreateWebhookRequest{
    Type:               credicarro.WebhookTypeProposal,
    URL:                "https://parceiro.com.br/webhooks/credicarro",
    AuthenticationType: credicarro.WebhookAuthenticationTypeBasic,
    Credential:         "dXN1YXJpbzpzZW5oYQ==", // Base64 de usuario:senha
})
```

No endpoint que recebe as notificações, leia o **corpo bruto** (limite sugerido de 1 MiB), verifique e decodifique:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
rawBody, _ := io.ReadAll(r.Body)

event, err := credicarro.ParseWebhookEvent(rawBody, r.Header.Get("X-CrediCarro-Signature"), signingSecret, 0) // 0 = tolerância de 5 min
if err != nil {
    http.Error(w, "assinatura inválida", http.StatusUnauthorized)
    return
}
// responda 2xx e processe de forma assíncrona; deduplique pelo cabeçalho Idempotency-Key
```

A assinatura é `t={timestamp},v1={hmac}`: HMAC-SHA256 de `"{t}.{corpo bruto}"` com o segredo em UTF-8, em hexadecimal minúsculo, comparado em tempo constante. A tolerância vale nos dois sentidos, e segredo vazio é sempre inválido. `ParseWebhookEvent` aceita o payload v1 (`id_proposta`, `enum_status`, `status` em inglês) e o v2 (retrato completo, em português; `IsDetailed == true`). Também existem `VerifyWebhookSignature` (retorna `bool`), `VerifyWebhookAuthorization` (compara o cabeçalho `Authorization` em tempo constante) e `ParseWebhookEventUnverified` (só para testes).

## Samples

- `samples/sample-cli` — script standalone: faixa de entrada de uma placa e pendências de uma proposta
- `samples/sample-http` — servidor `net/http` puro, com a consulta de faixa e o endpoint de webhook (`MaxBytesReader` de 1 MiB, assinatura e autenticação)

Cada sample é um módulo Go separado (`cd samples/sample-cli && go run . faixa ABC1D23 11222333000181`).

## Qualidade, segurança e cobertura

- **Build/Test** (`ci.yml`) — `go vet` + `golangci-lint` + `gofmt -l` no job build; testes (`go test -race`) em Go 1.18–1.27.
- **Contract tests** (`contract-tests.yml`) — confere os tipos de transporte do SDK com o [`openapi.yaml` público](https://documentacao.credicarro.com.br/openapi.yaml) (módulo próprio, `gopkg.in/yaml.v3` só ali).
- **Quality** (`quality.yml`) — análise estática via Codacy CLI, resultados publicados na aba **Security → Code scanning**.
- **Security** (`security.yml`) — [CodeQL](https://codeql.github.com/) para Go, a cada PR/push e semanalmente.
- **Coverage** — cobertura de testes coletada via `go test -coverprofile` e publicada no [Codecov](https://codecov.io/gh/Twila-Digital/twila-credicarro-go-sdk).

## Documentação completa

[documentacao.credicarro.com.br](https://documentacao.credicarro.com.br) — referência de todos os endpoints, autenticação, erros, webhooks e mais.

## Contribuindo

Veja [CONTRIBUTING.md](CONTRIBUTING.md).

## Código de conduta

Este projeto segue o [Código de Conduta](CODE_OF_CONDUCT.md).

## Licença

[MIT](LICENSE)
