package credicarro_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

func TestCustomersUpsertSendsUnifiedBodyAndReturnsID(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"id_cliente":"`+testUUID+`"}`)
	})

	id, err := client.Customers.Upsert(context.Background(), upsertRequest())
	if err != nil || id != testUUID {
		t.Fatalf("esperava %s, veio %q (%v)", testUUID, id, err)
	}

	req := api.recorded()[0]
	if req.Method != http.MethodPost || req.Path != "v1/customer/unified" {
		t.Fatalf("rota incorreta: %s %s", req.Method, req.Path)
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type incorreto: %q", req.Header.Get("Content-Type"))
	}

	expected := map[string]interface{}{
		"cpf":              "12345678909",
		"nome":             "Maria",
		"sobrenome":        "Souza",
		"email":            "maria@exemplo.com.br",
		"celular":          "+55 (11) 99876-5432",
		"dataDeNascimento": "1990-05-20T00:00:00-03:00",
		"endereco": map[string]interface{}{
			"rua": "Av. Paulista", "bairro": "Bela Vista", "cidade": "São Paulo", "estado": "SP", "cep": "01311000",
		},
		"dadosProfissionais": map[string]interface{}{"tipoOcupacao": float64(3), "renda": float64(6500)},
	}
	if got := req.JSON(t); !reflect.DeepEqual(got, expected) {
		t.Fatalf("corpo incorreto:\n got: %v\nwant: %v", got, expected)
	}
}

func TestCustomersUpsertSendsOptionalAddressFieldsWhenPresent(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"id_cliente":"`+testUUID+`"}`)
	})
	req := upsertRequest()
	req.Address.Number = "1578"
	req.Address.Complement = "Bloco B, Apto 1203"

	if _, err := client.Customers.Upsert(context.Background(), req); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	address := api.recorded()[0].JSON(t)["endereco"].(map[string]interface{})
	if address["numero"] != "1578" || address["complemento"] != "Bloco B, Apto 1203" {
		t.Fatalf("endereço incorreto: %v", address)
	}
}

func TestCustomersCreateSendsOnlyPersonalFields(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"id_cliente":"`+testUUID+`"}`)
	})

	id, err := client.Customers.Create(context.Background(), credicarro.CreateCustomerRequest{
		Cpf: "12345678909", FirstName: "Maria", LastName: "Souza", Email: "maria@exemplo.com.br",
		PhoneNumber: "+55 (11) 99876-5432", DateOfBirth: "1990-05-20T00:00:00-03:00",
	})
	if err != nil || id != testUUID {
		t.Fatalf("esperava %s, veio %q (%v)", testUUID, id, err)
	}

	req := api.recorded()[0]
	if req.Method != http.MethodPost || req.Path != "v1/customer" {
		t.Fatalf("rota incorreta: %s %s", req.Method, req.Path)
	}
	body := req.JSON(t)
	if len(body) != 6 || body["nome"] != "Maria" || body["celular"] != "+55 (11) 99876-5432" {
		t.Fatalf("corpo incorreto: %v", body)
	}
	if req.Header.Get("Idempotency-Key") == "" {
		t.Fatal("esperava Idempotency-Key automática")
	}
}

func TestCustomersUpsertAddressAccepts202(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("Accepted"))
	})

	err := client.Customers.UpsertAddress(context.Background(), "12345678909", credicarro.CustomerAddress{
		Street: "Av. Paulista", Number: "1578", Neighborhood: "Bela Vista", City: "São Paulo", State: "SP", PostalCode: "01311000",
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	req := api.recorded()[0]
	if req.Path != "v1/customer/address" {
		t.Fatalf("rota incorreta: %s", req.Path)
	}
	body := req.JSON(t)
	address := body["endereco"].(map[string]interface{})
	if body["cpf"] != "12345678909" || address["rua"] != "Av. Paulista" || address["numero"] != "1578" || address["cep"] != "01311000" {
		t.Fatalf("corpo incorreto: %v", body)
	}
	if _, present := address["complemento"]; present {
		t.Fatal("complemento vazio não deve ser enviado")
	}
}

func TestCustomersUpsertProfessionalDataAccepts202(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	err := client.Customers.UpsertProfessionalData(context.Background(), "12345678909", credicarro.ProfessionalData{
		Occupation: credicarro.OccupationTypeBusinessOwner, MonthlyIncome: 6500.5,
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	req := api.recorded()[0]
	body := req.JSON(t)
	if req.Path != "v1/customer/professional" || body["cpf"] != "12345678909" || body["tipoOcupacao"] != float64(7) || body["renda"] != 6500.5 {
		t.Fatalf("requisição incorreta: %s %v", req.Path, body)
	}
}

func TestOccupationTypeValuesMatchTheContract(t *testing.T) {
	expected := map[credicarro.OccupationType]int{
		credicarro.OccupationTypeRetired: 1, credicarro.OccupationTypePensioner: 2, credicarro.OccupationTypeEmployee: 3,
		credicarro.OccupationTypeSelfEmployed: 4, credicarro.OccupationTypeFreelancer: 6, credicarro.OccupationTypeBusinessOwner: 7,
	}
	for occupation, value := range expected {
		if int(occupation) != value {
			t.Errorf("OccupationType %d deveria valer %d", occupation, value)
		}
	}
}

func TestCustomersRejectEmptyCpf(t *testing.T) {
	client, api := newTestClient(t, nil)
	assertConfigurationError(t, client.Customers.UpsertAddress(context.Background(), " ", credicarro.CustomerAddress{}))
	assertConfigurationError(t, client.Customers.UpsertProfessionalData(context.Background(), "", credicarro.ProfessionalData{}))
	if len(api.recorded()) != 0 {
		t.Fatal("nenhuma requisição deveria ser enviada")
	}
}

func TestCustomersConflictMapsToAPIErrorWithStableCode(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusConflict, `{"tipo":"Cpf.CpfAlreadyAdded","titulo":"Conflito","status":409}`)
	})

	_, err := client.Customers.Create(context.Background(), credicarro.CreateCustomerRequest{Cpf: "12345678909"})
	var apiErr *credicarro.APIError
	if !asAPIError(err, &apiErr) || apiErr.StatusCode != 409 || apiErr.ErrorCode != "Cpf.CpfAlreadyAdded" {
		t.Fatalf("esperava APIError 409 Cpf.CpfAlreadyAdded, veio %T: %v", err, err)
	}
}
