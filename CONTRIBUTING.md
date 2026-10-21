# Contribuindo

## Pré-requisitos

- Go 1.18+ (o módulo principal não pode usar APIs mais novas que o Go 1.18: sem `min`/`max` embutidos, `slices`, `errors.Join`, `atomic.Int64` etc.)
- [golangci-lint](https://golangci-lint.run/) (opcional, mas usado no CI)

## Build e testes

```bash
go build ./...
go vet ./...
gofmt -l .          # deve ficar vazio; use `gofmt -w .` pra corrigir
golangci-lint run ./...
go test ./... -race
```

Os contract tests (`contract-tests/`) baixam o `openapi.yaml` público (`https://documentacao.credicarro.com.br/openapi.yaml`, ou o endereço da variável `CREDICARRO_CONTRACT_TESTS_OPENAPI_URL`) e conferem os tipos de transporte do SDK com ele. Precisam de rede e não rodam por padrão — é um módulo Go separado (próprio `go.mod`), pra `gopkg.in/yaml.v3` não entrar na árvore de dependências do módulo principal:

```bash
cd contract-tests
go test ./...
```

Os samples (`samples/sample-cli` e `samples/sample-http`) também são módulos separados:

```bash
cd samples/sample-cli && go build ./...
cd samples/sample-http && go build ./...
```

## Instalando a partir do código-fonte

Enquanto uma tag ainda não foi publicada, referencie o commit diretamente:

```bash
go get github.com/Twila-Digital/twila-credicarro-go-sdk@<commit-sha>
```

Ou, num checkout local, use uma diretiva `replace` no seu `go.mod`:

```
replace github.com/Twila-Digital/twila-credicarro-go-sdk => /caminho/local/twila-credicarro-go-sdk
```

## Abrindo um PR

1. Crie uma branch a partir de `production`
2. Adicione testes para qualquer mudança de comportamento
3. Rode `go vet ./... && golangci-lint run ./... && go test ./... -race` localmente antes de abrir o PR
4. Abra o PR contra `production` — o CI roda build + testes automaticamente

Regras do projeto: nenhum dado pessoal, `ClientSecret` ou `SigningSecret` em log, mensagem de erro ou `String()`; mensagens para o integrador em português do Brasil; tipos de transporte (`internal/wire`) só com propriedades do `openapi.yaml`.

## Release (módulos Go não têm "publish")

Diferente de todos os outros registries deste ecossistema, **um módulo Go não tem passo de publicação nenhum** — não existe conta pra criar, token pra gerar, nem webhook pra configurar. O [Go module proxy](https://proxy.golang.org/) resolve qualquer tag `vX.Y.Z` sob demanda, na primeira vez que alguém rodar `go get` apontando pra ela.

`git tag vX.Y.Z && git push origin vX.Y.Z` é o release inteiro. O `release.yml` deste repositório serve só como gate de qualidade (CI + contract tests) e pra criar o GitHub Release com changelog automático — não há `environment`/aprovação manual porque não existe nenhuma credencial de publish pra proteger.

**Atenção ao versionamento semântico de módulos Go**: a partir de `v2.0.0`, o caminho de import precisa incluir o major version (`github.com/Twila-Digital/twila-credicarro-go-sdk/v2`) — isso está fora do escopo de uma tag `v1.x.y` comum, mas vale lembrar antes de um eventual breaking change futuro.

## Reportando problemas

Abra uma [issue](https://github.com/Twila-Digital/twila-credicarro-go-sdk/issues) com passos para reproduzir, versão do módulo/Go e o comportamento esperado vs. observado. Nunca inclua `ClientID`/`ClientSecret` reais, CPFs ou outros dados pessoais no relato.
