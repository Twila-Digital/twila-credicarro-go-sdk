package credicarro_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"reflect"
	"testing"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

const proposalV2JSON = `{
 "id_proposta":"bea8d060-f93d-48eb-b6c4-45f64483cdd9","numero":1234,"id_simulacao":"123e4567-e89b-12d3-a456-426614174000",
 "enum_status":2,"status":"APROVADA","tipo":"Financing","data_criacao":"2026-10-12T14:05:08-03:00",
 "motivo_recusa":null,"link_biometria":"https://biometria.exemplo.com.br/abc",
 "comprador":{
   "cpf":"12345678909","enum_biometria":3,"biometria":"APROVADA",
   "status_ativo":{"enum_etapa":7,"etapa":"CORRECAO_DOCUMENTOS","enum_situacao":4,"situacao":"PENDENTE_CORRECAO","enum_responsavel_acao":2,"responsavel_acao":"INTEGRADOR"},
   "etapas_concluidas":[{"enum_etapa":1,"etapa":"PROPOSTA_CRIADA","data":"2026-10-12T14:05:08-03:00"},{"enum_etapa":4,"etapa":"ANALISE_BIOMETRIA","data":"2026-10-12T14:20:00-03:00"}],
   "documentos":[
     {"enum_tipo":1,"tipo":"CNH","enum_status":2,"status":"INVALIDO","motivo":"Documento ilegível"},
     {"enum_tipo":5,"tipo":"COMPROVANTE_RESIDENCIA","enum_status":1,"status":"VALIDO"},
     {"enum_tipo":4,"tipo":"COMPROVANTE_RENDA","enum_status":5,"status":"PENDENTE"}]},
 "loja":{
   "cnpj":"11222333000181",
   "status_ativo":{"enum_etapa":6,"etapa":"ENVIO_DOCUMENTOS","enum_situacao":1,"situacao":"AGUARDANDO_ENVIO","enum_responsavel_acao":2,"responsavel_acao":"INTEGRADOR"},
   "etapas_concluidas":[],
   "documentos":[{"enum_tipo":8,"tipo":"LAUDO_ECV","enum_status":5,"status":"PENDENTE"},{"enum_tipo":6,"tipo":"DOCUMENTO_VEICULO","enum_status":1,"status":"VALIDO"},{"enum_tipo":99,"tipo":"NOVO_TIPO","enum_status":5,"status":"PENDENTE"}]}
}`

func TestProposalsCreateV2(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusCreated, `{"id_proposta":"`+testUUID+`"}`)
	})

	id, err := client.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2})
	if err != nil || id != testUUID {
		t.Fatalf("esperava %s, veio %q (%v)", testUUID, id, err)
	}
	req := api.recorded()[0]
	if req.Method != http.MethodPost || req.Path != "v2/proposal" {
		t.Fatalf("rota incorreta: %s %s", req.Method, req.Path)
	}
	expected := map[string]interface{}{"cpf": "12345678909", "simulacaoId": testUUID2}
	if !reflect.DeepEqual(req.JSON(t), expected) {
		t.Fatalf("corpo incorreto: %v", req.JSON(t))
	}
}

func TestProposalsCreateRejectsInvalidSimulationID(t *testing.T) {
	client, api := newTestClient(t, nil)
	_, err := client.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: "nao-e-uuid"})
	assertConfigurationError(t, err)
	if len(api.recorded()) != 0 {
		t.Fatal("nenhuma requisição deveria ser enviada")
	}
}

func TestProposalsCreateMapsBusinessRuleCodes(t *testing.T) {
	for _, code := range []string{"Customer.Account.Incomplete", "Proposal.SimulationAlreadyExpired", "Proposal.SimulationAlreadyActivated", "Proposal.BuyerAgeNotAllowed", "Simulation.Access.NotFromStore"} {
		code := code
		client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			writeRaw(w, http.StatusUnprocessableEntity, `{"tipo":"`+code+`","titulo":"Regra de negócio não atendida","status":422}`)
		})
		_, err := client.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2})
		var rule *credicarro.BusinessRuleError
		if !asBusinessRule(err, &rule) || rule.ErrorCode != code {
			t.Errorf("%s: esperava BusinessRuleError, veio %T: %v", code, err, err)
		}
	}
}

func TestProposalsGetMapsTheFullSnapshot(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, proposalV2JSON)
	})

	p, err := client.Proposals.Get(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if req := api.recorded()[0]; req.Method != http.MethodGet || req.Path != "v2/proposal/"+testUUID {
		t.Fatalf("rota incorreta: %s %s", req.Method, req.Path)
	}

	if p.ID != testUUID || p.Number != 1234 || p.SimulationID != testUUID2 || p.Status != credicarro.ProposalStatusApproved ||
		p.StatusRaw != 2 || p.StatusName != "APROVADA" || p.Type != credicarro.ProposalTypeFinancing || p.TypeRaw != "Financing" ||
		p.CreatedAt != "2026-10-12T14:05:08-03:00" || p.RefusalReason != "" || p.BiometryLink != "https://biometria.exemplo.com.br/abc" {
		t.Fatalf("cabeçalho incorreto: %+v", p)
	}
	if p.Seller != nil {
		t.Fatal("sem vendedor na resposta, Seller deve ser nil")
	}

	buyer := p.Buyer
	if buyer == nil || buyer.Document != "12345678909" || buyer.Biometry == nil || *buyer.Biometry != credicarro.BiometryStatusApproved ||
		buyer.BiometryRaw != 3 || buyer.BiometryName != "APROVADA" {
		t.Fatalf("comprador incorreto: %+v", buyer)
	}
	step := buyer.ActiveStep
	if step == nil || step.Step != credicarro.ProposalStepDocumentCorrection || step.StepRaw != 7 || step.StepName != "CORRECAO_DOCUMENTOS" ||
		step.Situation != credicarro.StepSituationPendingCorrection || step.SituationRaw != 4 || step.SituationName != "PENDENTE_CORRECAO" ||
		step.ActionOwner != credicarro.ActionOwnerIntegrator || step.ActionOwnerRaw != 2 || step.ActionOwnerName != "INTEGRADOR" {
		t.Fatalf("etapa ativa incorreta: %+v", step)
	}
	if len(buyer.CompletedSteps) != 2 || buyer.CompletedSteps[1].Step != credicarro.ProposalStepBiometryAnalysis || buyer.CompletedSteps[1].Date != "2026-10-12T14:20:00-03:00" {
		t.Fatalf("etapas concluídas incorretas: %+v", buyer.CompletedSteps)
	}
	if len(buyer.Documents) != 3 {
		t.Fatalf("documentos incorretos: %+v", buyer.Documents)
	}
	cnh := buyer.Documents[0]
	if cnh.Type != credicarro.DocumentTypeCnh || cnh.TypeName != "CNH" || cnh.Status != credicarro.DocumentStatusInvalid || cnh.StatusRaw != 2 || cnh.Reason != "Documento ilegível" {
		t.Fatalf("documento incorreto: %+v", cnh)
	}

	store := p.Store
	if store == nil || store.Document != "11222333000181" || store.Biometry != nil {
		t.Fatalf("loja incorreta: %+v", store)
	}
	unknownDoc := store.Documents[2]
	if unknownDoc.Type != credicarro.DocumentTypeUnknown || unknownDoc.TypeRaw != 99 || unknownDoc.TypeName != "NOVO_TIPO" {
		t.Fatalf("tipo de documento desconhecido deveria preservar o valor bruto: %+v", unknownDoc)
	}
}

func TestProposalsGetUnknownEnumValuesFallBackToUnknownPreservingRawValues(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{
 "id_proposta":"`+testUUID+`","enum_status":77,"status":"NOVO_STATUS","tipo":"Leasing",
 "comprador":{"cpf":"12345678909","enum_biometria":42,"biometria":"NOVA",
   "status_ativo":{"enum_etapa":99,"etapa":"NOVA_ETAPA","enum_situacao":88,"situacao":"NOVA_SITUACAO","enum_responsavel_acao":55,"responsavel_acao":"NOVO"},
   "documentos":[{"enum_tipo":500,"tipo":"X","enum_status":66,"status":"Y"}]}}`)
	})

	p, err := client.Proposals.Get(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("valores desconhecidos não podem gerar erro: %v", err)
	}
	if p.Status != credicarro.ProposalStatusUnknown || p.StatusRaw != 77 || p.StatusName != "NOVO_STATUS" {
		t.Fatalf("status incorreto: %+v", p)
	}
	if p.Type != credicarro.ProposalTypeUnknown || p.TypeRaw != "Leasing" {
		t.Fatalf("tipo incorreto: %v %q", p.Type, p.TypeRaw)
	}
	b := p.Buyer
	if *b.Biometry != credicarro.BiometryStatusUnknown || b.BiometryRaw != 42 {
		t.Fatalf("biometria incorreta: %v %d", *b.Biometry, b.BiometryRaw)
	}
	s := b.ActiveStep
	if s.Step != credicarro.ProposalStepUnknown || s.StepRaw != 99 || s.Situation != credicarro.StepSituationUnknown || s.SituationRaw != 88 ||
		s.ActionOwner != credicarro.ActionOwnerUnknown || s.ActionOwnerRaw != 55 {
		t.Fatalf("etapa incorreta: %+v", s)
	}
	d := b.Documents[0]
	if d.Type != credicarro.DocumentTypeUnknown || d.TypeRaw != 500 || d.Status != credicarro.DocumentStatusUnknown || d.StatusRaw != 66 {
		t.Fatalf("documento incorreto: %+v", d)
	}
}

func TestEnumValuesMatchTheContract(t *testing.T) {
	check := func(name string, got, want int) {
		t.Helper()
		if got != want {
			t.Errorf("%s: esperava %d, veio %d", name, want, got)
		}
	}
	check("ProposalStatusUnknown", int(credicarro.ProposalStatusUnknown), -1)
	check("ProposalStatusAnalysing", int(credicarro.ProposalStatusAnalysing), 1)
	check("ProposalStatusPending", int(credicarro.ProposalStatusPending), 9)
	check("ProposalStepUndefined", int(credicarro.ProposalStepUndefined), 0)
	check("ProposalStepProposalCompleted", int(credicarro.ProposalStepProposalCompleted), 12)
	check("StepSituationAwaitingSubmission", int(credicarro.StepSituationAwaitingSubmission), 1)
	check("StepSituationCanceled", int(credicarro.StepSituationCanceled), 7)
	check("ActionOwnerNone", int(credicarro.ActionOwnerNone), 0)
	check("ActionOwnerCredicarro", int(credicarro.ActionOwnerCredicarro), 3)
	check("BiometryStatusNotStarted", int(credicarro.BiometryStatusNotStarted), 0)
	check("BiometryStatusRejected", int(credicarro.BiometryStatusRejected), 4)
	check("DocumentStatusReceived", int(credicarro.DocumentStatusReceived), 0)
	check("DocumentStatusPending", int(credicarro.DocumentStatusPending), 5)
	check("DocumentTypeCnh", int(credicarro.DocumentTypeCnh), 1)
	check("DocumentTypeCnpjCard", int(credicarro.DocumentTypeCnpjCard), 11)
	check("DocumentTypeAtpv", int(credicarro.DocumentTypeAtpv), 57)
	check("WebhookTypeProposal", int(credicarro.WebhookTypeProposal), 3)
	check("WebhookAuthenticationTypeJwt", int(credicarro.WebhookAuthenticationTypeJwt), 3)
	check("WebhookStatusInactive", int(credicarro.WebhookStatusInactive), 2)

	if credicarro.DepositAccountTypeChecking != "CORRENTE" || credicarro.DepositAccountTypeSavings != "POUPANCA" || credicarro.DepositAccountTypePayment != "PAGAMENTO" {
		t.Error("DepositAccountType fora do contrato")
	}
	if credicarro.ProposalParticipantBuyer != "COMPRADOR" || credicarro.ProposalParticipantSeller != "VENDEDOR" || credicarro.ProposalParticipantStore != "LOJA" {
		t.Error("ProposalParticipant fora do contrato")
	}
}

func TestProposalsGetRejectsInvalidID(t *testing.T) {
	client, api := newTestClient(t, nil)
	_, err := client.Proposals.Get(context.Background(), "../v1/customer")
	assertConfigurationError(t, err)
	if len(api.recorded()) != 0 {
		t.Fatal("nenhuma requisição deveria ser enviada")
	}
}

func TestProposalsGetNotIntegrationV2IsBusinessRule(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusUnprocessableEntity, `{"tipo":"Proposal.Access.NotIntegrationV2","titulo":"Regra de negócio não atendida","status":422}`)
	})
	_, err := client.Proposals.Get(context.Background(), testUUID)
	var rule *credicarro.BusinessRuleError
	if !asBusinessRule(err, &rule) || rule.ErrorCode != "Proposal.Access.NotIntegrationV2" {
		t.Fatalf("esperava BusinessRuleError, veio %T: %v", err, err)
	}
}

func TestProposalsListMapsSummariesFromV1(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"itens":[
 {"id_proposta":"`+testUUID+`","numero":77,"id_simulacao":"`+testUUID2+`","tipo":"Refinancing","status":"ManualAnalysis","comprador":{"cpf":"12345678909"},"vendedor":{"cpf":"98765432100"},"data_criacao":"2026-10-12T14:05:08-03:00"},
 {"id_proposta":"`+testUUID2+`","numero":78,"id_simulacao":"`+testUUID+`","tipo":"Futuro","status":"Novo","comprador":{"cpf":"12345678909"},"data_criacao":"2026-10-13T10:00:00-03:00"}
],"pagina":{"tem_proximo":false,"tem_anterior":true,"numero":2,"tamanho":2,"total":4}}`)
	})

	page, err := client.Proposals.List(context.Background(), &credicarro.ListRequest{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if req := api.recorded()[0]; req.Path != "v1/proposal" || req.RawQuery != "pagina_inicial=2&limite=2" {
		t.Fatalf("requisição incorreta: %s ?%s", req.Path, req.RawQuery)
	}
	if page.Page != 2 || page.PageSize != 2 || page.TotalCount != 4 || page.HasNext || !page.HasPrevious || len(page.Items) != 2 {
		t.Fatalf("página incorreta: %+v", page)
	}

	first := page.Items[0]
	if first.ID != testUUID || first.Number != 77 || first.SimulationID != testUUID2 || first.Type != credicarro.ProposalTypeRefinancing ||
		first.Status != credicarro.ProposalStatusManualAnalysis || first.StatusName != "ManualAnalysis" || first.BuyerCpf != "12345678909" ||
		first.SellerCpf != "98765432100" || first.CreatedAt != "2026-10-12T14:05:08-03:00" {
		t.Fatalf("resumo incorreto: %+v", first)
	}
	second := page.Items[1]
	if second.SellerCpf != "" || second.Status != credicarro.ProposalStatusUnknown || second.StatusName != "Novo" || second.Type != credicarro.ProposalTypeUnknown || second.TypeRaw != "Futuro" {
		t.Fatalf("resumo com valores desconhecidos incorreto: %+v", second)
	}
}

func TestListRequestValidation(t *testing.T) {
	client, api := newTestClient(t, nil)
	for _, req := range []*credicarro.ListRequest{{Page: -1}, {PageSize: 101}, {PageSize: -5}} {
		_, err := client.Proposals.List(context.Background(), req)
		assertConfigurationError(t, err)
	}
	if len(api.recorded()) != 0 {
		t.Fatal("nenhuma requisição deveria ser enviada")
	}
}

func TestProposalsSendVehicleDetails(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	err := client.Proposals.SendVehicleDetails(context.Background(), testUUID, credicarro.VehicleDetails{
		Renavam: "12345678901", Chassis: "9BD11111111111111", LicensePlate: "ABC1D23", Color: "Branco",
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	req := api.recorded()[0]
	expected := map[string]interface{}{"renavam": "12345678901", "chassi": "9BD11111111111111", "placa": "ABC1D23", "cor": "Branco"}
	if req.Method != http.MethodPost || req.Path != "v2/proposal/"+testUUID+"/vehicle-detail" || !reflect.DeepEqual(req.JSON(t), expected) {
		t.Fatalf("requisição incorreta: %s %s %v", req.Method, req.Path, req.JSON(t))
	}
}

func TestProposalsSendDepositAccount(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	err := client.Proposals.SendDepositAccount(context.Background(), testUUID, credicarro.DepositAccount{
		HolderCpf: "12345678909", BankCode: "341", Branch: "1234", BranchDigit: "X", AccountNumber: "123456", AccountDigit: "7",
		AccountType: credicarro.DepositAccountTypeSavings,
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	req := api.recorded()[0]
	expected := map[string]interface{}{
		"cpfTitular": "12345678909", "codigoBanco": "341", "agencia": "1234", "digitoAgencia": "X",
		"conta": "123456", "digitoConta": "7", "tipoConta": "POUPANCA",
	}
	if req.Path != "v2/proposal/"+testUUID+"/deposit-account" || !reflect.DeepEqual(req.JSON(t), expected) {
		t.Fatalf("requisição incorreta: %s %v", req.Path, req.JSON(t))
	}
}

func TestProposalsUploadDocumentSendsBase64AndRequiredHeader(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	content := []byte("%PDF-1.7\x00\x01\x02 conteúdo binário")
	file, err := credicarro.NewDocumentFileFromBytes(content, "cnh-comprador.PDF")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	err = client.Proposals.UploadDocument(context.Background(), testUUID, credicarro.ProposalDocumentUpload{
		Participant: credicarro.ProposalParticipantBuyer, DocumentType: credicarro.DocumentTypeCnh, File: file,
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	req := api.recorded()[0]
	if req.Method != http.MethodPost || req.Path != "v2/proposal/"+testUUID+"/document" {
		t.Fatalf("rota incorreta: %s %s", req.Method, req.Path)
	}
	body := req.JSON(t)
	if body["participante"] != "COMPRADOR" || body["tipoDocumento"] != float64(1) || body["nomeArquivo"] != "cnh-comprador.PDF" || len(body) != 4 {
		t.Fatalf("corpo incorreto: %v", body)
	}
	decoded, err := base64.StdEncoding.DecodeString(body["conteudoBase64"].(string))
	if err != nil || string(decoded) != string(content) {
		t.Fatalf("conteudoBase64 incorreto: %v", err)
	}
	if req.Header.Get("Idempotency-Key") == "" {
		t.Fatal("o header Idempotency-Key é obrigatório")
	}
}

func TestProposalsUploadDocumentValidatesBeforeSending(t *testing.T) {
	client, api := newTestClient(t, nil)
	file, _ := credicarro.NewDocumentFileFromBytes([]byte("x"), "a.pdf")

	cases := map[string]credicarro.ProposalDocumentUpload{
		"participante vazio":    {DocumentType: credicarro.DocumentTypeCnh, File: file},
		"participante inválido": {Participant: "OUTRO", DocumentType: credicarro.DocumentTypeCnh, File: file},
		"tipo vazio":            {Participant: credicarro.ProposalParticipantBuyer, File: file},
		"tipo desconhecido":     {Participant: credicarro.ProposalParticipantBuyer, DocumentType: credicarro.DocumentTypeUnknown, File: file},
		"arquivo vazio":         {Participant: credicarro.ProposalParticipantBuyer, DocumentType: credicarro.DocumentTypeCnh},
	}
	for name, upload := range cases {
		upload := upload
		t.Run(name, func(t *testing.T) {
			assertConfigurationError(t, client.Proposals.UploadDocument(context.Background(), testUUID, upload))
		})
	}
	assertConfigurationError(t, client.Proposals.UploadDocument(context.Background(), "x", credicarro.ProposalDocumentUpload{}))
	if len(api.recorded()) != 0 {
		t.Fatal("nenhuma requisição deveria ser enviada")
	}
}

func TestProposalsUploadDocumentMapsErrors(t *testing.T) {
	cases := []struct {
		status int
		code   string
	}{{422, "Document.InvalidProposalStage"}, {409, "Idempotency.KeyConflict"}, {502, "Document.UploadFailed"}, {400, "Validation.File.InvalidExtension"}}
	file, _ := credicarro.NewDocumentFileFromBytes([]byte("x"), "a.png")

	for _, c := range cases {
		c := c
		api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
			writeRaw(w, c.status, `{"tipo":"`+c.code+`","titulo":"T"}`)
		})
		client := api.client(withResilience(func(r *credicarro.ResilienceOptions) { r.MaxRetryAttempts = 1 }))
		err := client.Proposals.UploadDocument(context.Background(), testUUID, credicarro.ProposalDocumentUpload{
			Participant: credicarro.ProposalParticipantSeller, DocumentType: credicarro.DocumentTypeRg, File: file,
		})
		var apiErr *credicarro.APIError
		if !asAPIError(err, &apiErr) || apiErr.StatusCode != c.status || apiErr.ErrorCode != c.code {
			t.Errorf("%d %s: veio %T: %v", c.status, c.code, err, err)
		}
	}
}

func buyerWith(step credicarro.ProposalStep, owner credicarro.ActionOwner, docs ...credicarro.ProposalDocumentState) *credicarro.ProposalParticipantState {
	return &credicarro.ProposalParticipantState{
		Document:   "12345678909",
		ActiveStep: &credicarro.ActiveStepState{Step: step, ActionOwner: owner},
		Documents:  docs,
	}
}

func TestPendingActions(t *testing.T) {
	pendingDocs := []credicarro.ProposalDocumentState{
		{Type: credicarro.DocumentTypeProofOfIncome, Status: credicarro.DocumentStatusPending},
		{Type: credicarro.DocumentTypeCnh, Status: credicarro.DocumentStatusValid},
		{Type: credicarro.DocumentTypeProofOfAddress, Status: credicarro.DocumentStatusPending},
		{Type: credicarro.DocumentTypeAtpv, Status: credicarro.DocumentStatusInvalid},
	}

	t.Run("envio de documentos lista só os pendentes", func(t *testing.T) {
		p := &credicarro.ProposalDetails{Buyer: buyerWith(credicarro.ProposalStepDocumentSubmission, credicarro.ActionOwnerIntegrator, pendingDocs...)}
		actions := p.PendingActions()
		expected := []credicarro.PendingAction{{
			Participant: credicarro.ProposalParticipantBuyer, Step: credicarro.ProposalStepDocumentSubmission,
			Kind:          credicarro.PendingActionSendDocuments,
			DocumentTypes: []credicarro.DocumentType{credicarro.DocumentTypeProofOfIncome, credicarro.DocumentTypeProofOfAddress},
		}}
		if !reflect.DeepEqual(actions, expected) {
			t.Fatalf("ações incorretas: %+v", actions)
		}
	})

	t.Run("correção lista só os inválidos", func(t *testing.T) {
		p := &credicarro.ProposalDetails{Buyer: buyerWith(credicarro.ProposalStepDocumentCorrection, credicarro.ActionOwnerIntegrator, pendingDocs...)}
		actions := p.PendingActions()
		if len(actions) != 1 || actions[0].Kind != credicarro.PendingActionCorrectDocuments ||
			!reflect.DeepEqual(actions[0].DocumentTypes, []credicarro.DocumentType{credicarro.DocumentTypeAtpv}) {
			t.Fatalf("ações incorretas: %+v", actions)
		}
	})

	t.Run("dados do veículo e conta de recebimento", func(t *testing.T) {
		p := &credicarro.ProposalDetails{
			Buyer: buyerWith(credicarro.ProposalStepDepositAccount, credicarro.ActionOwnerIntegrator),
			Store: buyerWith(credicarro.ProposalStepVehicleDetails, credicarro.ActionOwnerIntegrator),
		}
		actions := p.PendingActions()
		if len(actions) != 2 ||
			actions[0].Participant != credicarro.ProposalParticipantBuyer || actions[0].Kind != credicarro.PendingActionSendDepositAccount ||
			actions[1].Participant != credicarro.ProposalParticipantStore || actions[1].Kind != credicarro.PendingActionSendVehicleDetails {
			t.Fatalf("ações incorretas: %+v", actions)
		}
		if len(actions[0].DocumentTypes) != 0 {
			t.Fatal("estas ações não carregam tipos de documento")
		}
	})

	t.Run("ignora o que não é do integrador", func(t *testing.T) {
		p := &credicarro.ProposalDetails{
			Buyer:  buyerWith(credicarro.ProposalStepDocumentSubmission, credicarro.ActionOwnerCredicarro, pendingDocs...),
			Seller: buyerWith(credicarro.ProposalStepVehicleDetails, credicarro.ActionOwnerCustomer),
			Store:  buyerWith(credicarro.ProposalStepSignature, credicarro.ActionOwnerIntegrator),
		}
		if actions := p.PendingActions(); len(actions) != 0 {
			t.Fatalf("esperava nenhuma ação, veio %+v", actions)
		}
	})

	t.Run("sem participantes ou receptor nulo", func(t *testing.T) {
		if actions := (&credicarro.ProposalDetails{}).PendingActions(); len(actions) != 0 {
			t.Fatalf("esperava nenhuma ação, veio %+v", actions)
		}
		var nilDetails *credicarro.ProposalDetails
		if actions := nilDetails.PendingActions(); len(actions) != 0 {
			t.Fatalf("esperava nenhuma ação, veio %+v", actions)
		}
	})
}

func TestPendingActionsFromAParsedSnapshot(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { writeRaw(w, http.StatusOK, proposalV2JSON) })
	p, err := client.Proposals.Get(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	actions := p.PendingActions()
	if len(actions) != 2 {
		t.Fatalf("esperava 2 ações, veio %+v", actions)
	}
	correct := actions[0]
	if correct.Participant != credicarro.ProposalParticipantBuyer || correct.Kind != credicarro.PendingActionCorrectDocuments ||
		!reflect.DeepEqual(correct.DocumentTypes, []credicarro.DocumentType{credicarro.DocumentTypeCnh}) {
		t.Fatalf("correção incorreta: %+v", correct)
	}
	send := actions[1]
	if send.Participant != credicarro.ProposalParticipantStore || send.Kind != credicarro.PendingActionSendDocuments ||
		!reflect.DeepEqual(send.DocumentTypes, []credicarro.DocumentType{credicarro.DocumentTypeEcvReport, credicarro.DocumentTypeUnknown}) {
		t.Fatalf("envio incorreto: %+v", send)
	}
}
