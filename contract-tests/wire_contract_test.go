// Package contracttests confere os tipos de transporte do SDK com o openapi.yaml público
// da API do Credicarro.
//
// O swagger de staging exige autenticação (401), por isso os testes baixam o openapi.yaml
// publicado em https://documentacao.credicarro.com.br/openapi.yaml (sobrescreva com a
// variável CREDICARRO_CONTRACT_TESTS_OPENAPI_URL). Para cada operação, verifica que:
//
//   - a rota e o método existem no openapi.yaml;
//   - cada wire type de requisição/resposta só usa propriedades presentes no schema;
//   - os campos obrigatórios do schema da requisição existem no wire type;
//   - os parâmetros de query/header usados pelo SDK existem na operação.
//
// E, no sentido inverso, que toda operação do openapi.yaml está coberta pelo SDK.
package contracttests

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

const (
	defaultOpenAPIURL = "https://documentacao.credicarro.com.br/openapi.yaml"
	openAPIURLEnv     = "CREDICARRO_CONTRACT_TESTS_OPENAPI_URL"
)

var (
	specOnce sync.Once
	spec     map[string]interface{}
	specErr  error
)

func openAPI(t *testing.T) map[string]interface{} {
	t.Helper()

	specOnce.Do(func() {
		url := os.Getenv(openAPIURLEnv)
		if url == "" {
			url = defaultOpenAPIURL
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			specErr = err
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			specErr = fmt.Errorf("HTTP %d ao baixar %s", resp.StatusCode, url)
			return
		}
		specErr = yaml.NewDecoder(resp.Body).Decode(&spec)
	})

	if specErr != nil {
		t.Fatalf("falha ao baixar/decodificar o openapi.yaml: %v", specErr)
	}
	return spec
}

// ---------------------------------------------------------------- navegação no YAML

func asMap(v interface{}) map[string]interface{} {
	switch m := v.(type) {
	case map[string]interface{}:
		return m
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out
	default:
		return nil
	}
}

func asList(v interface{}) []interface{} {
	list, _ := v.([]interface{})
	return list
}

func lookup(root map[string]interface{}, pointer string) map[string]interface{} {
	var node interface{} = root
	for _, part := range strings.Split(strings.TrimPrefix(pointer, "#/"), "/") {
		node = asMap(node)[part]
	}
	return asMap(node)
}

// resolve segue $ref e mescla allOf, devolvendo um schema "achatado".
func resolve(root, node map[string]interface{}) map[string]interface{} {
	if node == nil {
		return nil
	}
	if ref, ok := node["$ref"].(string); ok {
		return resolve(root, lookup(root, ref))
	}
	if all := asList(node["allOf"]); len(all) > 0 {
		merged := map[string]interface{}{}
		props := map[string]interface{}{}
		var required []interface{}
		for _, part := range all {
			resolved := resolve(root, asMap(part))
			for k, v := range asMap(resolved["properties"]) {
				props[k] = v
			}
			required = append(required, asList(resolved["required"])...)
		}
		merged["properties"] = props
		merged["required"] = required
		return merged
	}
	return node
}

func schemaProperties(root, schema map[string]interface{}) map[string]interface{} {
	return asMap(resolve(root, schema)["properties"])
}

func schemaRequired(root, schema map[string]interface{}) []string {
	var required []string
	for _, r := range asList(resolve(root, schema)["required"]) {
		required = append(required, fmt.Sprint(r))
	}
	return required
}

func jsonContentSchema(root, container map[string]interface{}) map[string]interface{} {
	content := asMap(container["content"])
	media := asMap(content["application/json"])
	return resolve(root, asMap(media["schema"]))
}

func operation(root map[string]interface{}, method, path string) map[string]interface{} {
	return asMap(asMap(asMap(root["paths"])[path])[strings.ToLower(method)])
}

func requestSchema(root map[string]interface{}, method, path string) map[string]interface{} {
	op := operation(root, method, path)
	return jsonContentSchema(root, asMap(op["requestBody"]))
}

func responseSchema(root map[string]interface{}, method, path, status string) map[string]interface{} {
	op := operation(root, method, path)
	response := resolve(root, asMap(asMap(op["responses"])[status]))
	return jsonContentSchema(root, response)
}

func parameterNames(root, op map[string]interface{}) map[string]bool {
	names := map[string]bool{}
	for _, p := range asList(op["parameters"]) {
		param := resolve(root, asMap(p))
		names[fmt.Sprint(param["name"])] = true
	}
	return names
}

// ---------------------------------------------------------------- reflexão no wire

type wireField struct {
	name string
	typ  reflect.Type
}

func wireFields(t reflect.Type) []wireField {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	var fields []wireField
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			continue
		}
		fields = append(fields, wireField{name: name, typ: t.Field(i).Type})
	}
	return fields
}

// checkSubset garante que todo campo do wire type existe no schema (recursivamente).
func checkSubset(t *testing.T, root map[string]interface{}, where string, typ reflect.Type, schema map[string]interface{}) {
	t.Helper()
	props := schemaProperties(root, schema)

	for _, f := range wireFields(typ) {
		prop, ok := props[f.name]
		if !ok {
			t.Errorf("%s: o campo %q do wire type não existe no schema do openapi.yaml", where, f.name)
			continue
		}

		fieldType := f.typ
		for fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		propSchema := resolve(root, asMap(prop))
		switch fieldType.Kind() {
		case reflect.Struct:
			checkSubset(t, root, where+"."+f.name, fieldType, propSchema)
		case reflect.Slice:
			elem := fieldType.Elem()
			for elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Struct {
				checkSubset(t, root, where+"."+f.name+"[]", elem, resolve(root, asMap(propSchema["items"])))
			}
		}
	}
}

// checkRequired garante que todo campo obrigatório do schema existe no wire type.
func checkRequired(t *testing.T, root map[string]interface{}, where string, typ reflect.Type, schema map[string]interface{}) {
	t.Helper()
	have := map[string]reflect.Type{}
	for _, f := range wireFields(typ) {
		have[f.name] = f.typ
	}

	for _, name := range schemaRequired(root, schema) {
		field, ok := have[name]
		if !ok {
			t.Errorf("%s: o campo obrigatório %q do schema não existe no wire type", where, name)
			continue
		}

		prop := resolve(root, asMap(schemaProperties(root, schema)[name]))
		fieldType := field
		for fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct && len(schemaRequired(root, prop)) > 0 {
			checkRequired(t, root, where+"."+name, fieldType, prop)
		}
	}
}

// ---------------------------------------------------------------- tabela de operações

type op struct {
	method, path string
	request      interface{}            // wire type do corpo (nil se não há)
	responses    map[string]interface{} // status -> wire type
	query        []string               // parâmetros de query usados pelo SDK
	idempotency  bool                   // o SDK pode enviar Idempotency-Key
}

var operations = []op{
	{method: "POST", path: "/v1/authentication/accesstoken", request: wire.GenerateAccessTokenRequest{}, responses: map[string]interface{}{"200": wire.GenerateAccessTokenResponse{}}},

	{method: "POST", path: "/v1/customer/unified", request: wire.UnifiedCustomerRequest{}, responses: map[string]interface{}{"200": wire.CustomerIDResponse{}}, idempotency: true},
	{method: "POST", path: "/v1/customer", request: wire.CreateCustomerRequest{}, responses: map[string]interface{}{"200": wire.CustomerIDResponse{}}, idempotency: true},
	{method: "POST", path: "/v1/customer/address", request: wire.CustomerAddressRequest{}, idempotency: true},
	{method: "POST", path: "/v1/customer/professional", request: wire.CustomerProfessionalRequest{}, idempotency: true},

	{method: "GET", path: "/v2/proposal/simulation/range/financing", responses: map[string]interface{}{"200": wire.FinancingRangeResponse{}}, query: []string{"placa", "cnpjLoja"}},
	{method: "GET", path: "/v2/proposal/simulation/range/refinancing", responses: map[string]interface{}{"200": wire.RefinancingRangeResponse{}}, query: []string{"placa", "cnpjLoja"}},
	{method: "POST", path: "/v2/proposal/simulation/financing", request: wire.FinancingSimulationRequest{}, responses: map[string]interface{}{"201": wire.SimulationListResponse{}}, idempotency: true},
	{method: "POST", path: "/v2/proposal/simulation/refinancing", request: wire.RefinancingSimulationRequest{}, responses: map[string]interface{}{"201": wire.SimulationListResponse{}}, idempotency: true},

	{method: "GET", path: "/v1/proposal/simulation/range/financing", responses: map[string]interface{}{"200": wire.FinancingRangeResponse{}}, query: []string{"placa"}},
	{method: "GET", path: "/v1/proposal/simulation/range/refinancing", responses: map[string]interface{}{"200": wire.RefinancingRangeResponse{}}, query: []string{"placa"}},
	{method: "POST", path: "/v1/proposal/simulation/financing", request: wire.FinancingSimulationRequestV1{}, responses: map[string]interface{}{"201": wire.SimulationListResponse{}}, idempotency: true},
	{method: "POST", path: "/v1/proposal/simulation/refinancing", request: wire.RefinancingSimulationRequestV1{}, responses: map[string]interface{}{"201": wire.SimulationListResponse{}}, idempotency: true},
	{method: "GET", path: "/v1/proposal/simulation/{simulacaoId}", responses: map[string]interface{}{"200": wire.Simulation{}}},
	{method: "GET", path: "/v1/proposal/simulation", responses: map[string]interface{}{"200": wire.PagedSimulations{}}, query: []string{"pagina_inicial", "limite"}},

	{method: "POST", path: "/v2/proposal", request: wire.CreateProposalRequest{}, responses: map[string]interface{}{"201": wire.CreateProposalResponse{}}, idempotency: true},
	{method: "GET", path: "/v2/proposal/{propostaId}", responses: map[string]interface{}{"200": wire.ProposalV2{}}},
	{method: "POST", path: "/v2/proposal/{propostaId}/vehicle-detail", request: wire.VehicleDetailsRequest{}},
	{method: "POST", path: "/v2/proposal/{propostaId}/deposit-account", request: wire.DepositAccountRequest{}},
	{method: "POST", path: "/v2/proposal/{propostaId}/document", request: wire.DocumentUploadRequest{}, idempotency: true},

	{method: "POST", path: "/v1/proposal", request: wire.CreateProposalRequest{}, responses: map[string]interface{}{"201": wire.CreateProposalResponse{}}, idempotency: true},
	{method: "GET", path: "/v1/proposal/{propostaId}", responses: map[string]interface{}{"200": wire.ProposalV1{}}},
	{method: "GET", path: "/v1/proposal", responses: map[string]interface{}{"200": wire.PagedProposals{}}, query: []string{"pagina_inicial", "limite"}},

	{method: "POST", path: "/v1/webhook", request: wire.CreateWebhookRequest{}, responses: map[string]interface{}{"201": wire.CreateWebhookResponse{}}},
	{method: "GET", path: "/v1/webhook", responses: map[string]interface{}{"200": wire.PagedWebhooks{}}, query: []string{"pagina_inicial", "limite"}},
	{method: "PUT", path: "/v1/webhook/{webhookId}", request: wire.UpdateWebhookRequest{}},
	{method: "DELETE", path: "/v1/webhook/{webhookId}"},
}

func (o op) name() string { return o.method + " " + o.path }

func TestEveryOperationExistsInTheOpenAPI(t *testing.T) {
	root := openAPI(t)
	for _, o := range operations {
		o := o
		t.Run(o.name(), func(t *testing.T) {
			if operation(root, o.method, o.path) == nil {
				t.Fatalf("a operação %s não existe no openapi.yaml — o SDK e a API divergiram", o.name())
			}
		})
	}
}

func TestEveryOpenAPIOperationIsCoveredByTheSDK(t *testing.T) {
	root := openAPI(t)

	covered := map[string]bool{}
	for _, o := range operations {
		covered[o.name()] = true
	}

	var missing []string
	for path, item := range asMap(root["paths"]) {
		for method := range asMap(item) {
			switch strings.ToUpper(method) {
			case "GET", "POST", "PUT", "DELETE", "PATCH":
				name := strings.ToUpper(method) + " " + path
				if !covered[name] {
					missing = append(missing, name)
				}
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("operações do openapi.yaml sem método no SDK: %s", strings.Join(missing, ", "))
	}
}

func TestRequestWireTypesMatchTheSchema(t *testing.T) {
	root := openAPI(t)
	for _, o := range operations {
		o := o
		if o.request == nil {
			continue
		}
		t.Run(o.name(), func(t *testing.T) {
			schema := requestSchema(root, o.method, o.path)
			if schema == nil {
				t.Fatalf("a operação %s não declara corpo JSON no openapi.yaml", o.name())
			}
			typ := reflect.TypeOf(o.request)
			checkSubset(t, root, typ.Name(), typ, schema)
			checkRequired(t, root, typ.Name(), typ, schema)
		})
	}
}

func TestResponseWireTypesMatchTheSchema(t *testing.T) {
	root := openAPI(t)
	for _, o := range operations {
		for status, response := range o.responses {
			o, status, response := o, status, response
			t.Run(o.name()+" "+status, func(t *testing.T) {
				schema := responseSchema(root, o.method, o.path, status)
				if schema == nil {
					t.Fatalf("a operação %s não declara resposta JSON %s no openapi.yaml", o.name(), status)
				}
				typ := reflect.TypeOf(response)
				checkSubset(t, root, typ.Name(), typ, schema)
			})
		}
	}
}

func TestSharedSchemasMatch(t *testing.T) {
	root := openAPI(t)
	schemas := asMap(asMap(root["components"])["schemas"])

	cases := map[string]interface{}{
		"ProblemDetails": wire.ProblemDetails{},
		"Page":           wire.Page{},
	}
	for name, typ := range cases {
		schema, ok := schemas[name]
		if !ok {
			t.Errorf("o schema %s não existe no openapi.yaml", name)
			continue
		}
		checkSubset(t, root, name, reflect.TypeOf(typ), asMap(schema))
	}
}

func TestOperationParametersExist(t *testing.T) {
	root := openAPI(t)
	for _, o := range operations {
		o := o
		if len(o.query) == 0 && !o.idempotency {
			continue
		}
		t.Run(o.name(), func(t *testing.T) {
			names := parameterNames(root, operation(root, o.method, o.path))
			for _, q := range o.query {
				if !names[q] {
					t.Errorf("o parâmetro %q usado pelo SDK não existe na operação", q)
				}
			}
			if o.idempotency && !names["Idempotency-Key"] {
				t.Error("a operação deveria aceitar o header Idempotency-Key")
			}
		})
	}
}

func TestDocumentUploadRequiresIdempotencyKeyInTheSpec(t *testing.T) {
	root := openAPI(t)
	op := operation(root, "POST", "/v2/proposal/{propostaId}/document")
	for _, p := range asList(op["parameters"]) {
		param := resolve(root, asMap(p))
		if param["name"] == "Idempotency-Key" {
			if param["required"] != true {
				t.Fatal("o openapi.yaml deveria marcar a Idempotency-Key do envio de documento como obrigatória")
			}
			return
		}
	}
	t.Fatal("o envio de documento não declara o header Idempotency-Key")
}

func TestUnifiedCustomerAcceptsIdempotencyKeyButSDKNeverAddsItAutomatically(t *testing.T) {
	root := openAPI(t)
	names := parameterNames(root, operation(root, "POST", "/v1/customer/unified"))
	if !names["Idempotency-Key"] {
		t.Fatal("o openapi.yaml deveria declarar Idempotency-Key no unified (o integrador pode informar)")
	}
}

func TestWebhookRoutesDoNotUseIdempotencyKey(t *testing.T) {
	root := openAPI(t)
	for _, o := range []op{{method: "POST", path: "/v1/webhook"}, {method: "PUT", path: "/v1/webhook/{webhookId}"}, {method: "DELETE", path: "/v1/webhook/{webhookId}"}} {
		if parameterNames(root, operation(root, o.method, o.path))["Idempotency-Key"] {
			t.Errorf("%s passou a aceitar Idempotency-Key — revise o retry do SDK", o.name())
		}
	}
}

func TestEnumsDeclaredInTheSpecAreKnownToTheSDK(t *testing.T) {
	root := openAPI(t)

	enumValues := func(schema map[string]interface{}, prop string) []string {
		var values []string
		for _, v := range asList(asMap(schemaProperties(root, schema)[prop])["enum"]) {
			values = append(values, fmt.Sprint(v))
		}
		return values
	}

	t.Run("participante e tipoConta", func(t *testing.T) {
		participants := enumValues(requestSchema(root, "POST", "/v2/proposal/{propostaId}/document"), "participante")
		sdk := []string{string(credicarro.ProposalParticipantBuyer), string(credicarro.ProposalParticipantSeller), string(credicarro.ProposalParticipantStore)}
		assertSameSet(t, "participante", participants, sdk)

		accounts := enumValues(requestSchema(root, "POST", "/v2/proposal/{propostaId}/deposit-account"), "tipoConta")
		sdk = []string{string(credicarro.DepositAccountTypeChecking), string(credicarro.DepositAccountTypeSavings), string(credicarro.DepositAccountTypePayment)}
		assertSameSet(t, "tipoConta", accounts, sdk)
	})

	t.Run("status da proposta v2 são reconhecidos", func(t *testing.T) {
		schema := responseSchema(root, "GET", "/v2/proposal/{propostaId}", "200")
		names := enumValues(schema, "status")
		if len(names) == 0 {
			t.Fatal("o openapi.yaml não declara os nomes de status da proposta")
		}
		for _, name := range names {
			body := fmt.Sprintf(`{"id_proposta":"bea8d060-f93d-48eb-b6c4-45f64483cdd9","status":%q}`, name)
			event, err := credicarro.ParseWebhookEventUnverified([]byte(body))
			if err != nil || event.Status == credicarro.ProposalStatusUnknown {
				t.Errorf("o status %q do openapi.yaml não é reconhecido pelo SDK (%v)", name, err)
			}
		}
	})

	t.Run("status da proposta v1 em inglês são reconhecidos", func(t *testing.T) {
		schema := responseSchema(root, "GET", "/v1/proposal/{propostaId}", "200")
		for _, name := range enumValues(schema, "status") {
			body := fmt.Sprintf(`{"id_proposta":"bea8d060-f93d-48eb-b6c4-45f64483cdd9","status":%q}`, name)
			event, err := credicarro.ParseWebhookEventUnverified([]byte(body))
			if err != nil || event.Status == credicarro.ProposalStatusUnknown {
				t.Errorf("o status %q do openapi.yaml não é reconhecido pelo SDK (%v)", name, err)
			}
		}
	})
}

func assertSameSet(t *testing.T, what string, spec, sdk []string) {
	t.Helper()
	sort.Strings(spec)
	sort.Strings(sdk)
	if !reflect.DeepEqual(spec, sdk) {
		t.Errorf("%s: o openapi.yaml declara %v e o SDK conhece %v", what, spec, sdk)
	}
}
