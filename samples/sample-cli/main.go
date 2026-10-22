// sample-cli consulta a faixa de entrada do financiamento de uma placa, e lista as ações
// pendentes de uma proposta, usando o ambiente de staging.
//
// Uso:
//
//	export CREDICARRO_CLIENT_ID=...
//	export CREDICARRO_CLIENT_SECRET=...
//	go run . faixa ABC1D23 11222333000181
//	go run . pendencias bea8d060-f93d-48eb-b6c4-45f64483cdd9
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

func main() {
	clientID := os.Getenv("CREDICARRO_CLIENT_ID")
	clientSecret := os.Getenv("CREDICARRO_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		fmt.Fprintln(os.Stderr, "Defina CREDICARRO_CLIENT_ID e CREDICARRO_CLIENT_SECRET no ambiente.")
		os.Exit(1)
	}
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Uso: sample-cli faixa <placa> <cnpjLoja> | sample-cli pendencias <idProposta>")
		os.Exit(2)
	}

	client, err := credicarro.NewClient(credicarro.ClientOptions{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Environment:  credicarro.EnvironmentStaging,
	})
	if err != nil {
		fail(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	switch os.Args[1] {
	case "faixa":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Uso: sample-cli faixa <placa> <cnpjLoja>")
			os.Exit(2)
		}
		options, err := client.Simulations.GetFinancingRange(ctx, os.Args[2], os.Args[3])
		if err != nil {
			fail(err)
		}
		for _, option := range options {
			fmt.Printf("%d meses: entrada mínima de R$ %.2f\n", option.Term, option.MinimumDownPayment)
		}

	case "pendencias":
		proposal, err := client.Proposals.Get(ctx, os.Args[2])
		if err != nil {
			fail(err)
		}
		fmt.Printf("Proposta %d: %s\n", proposal.Number, proposal.StatusName)
		for _, action := range proposal.PendingActions() {
			fmt.Printf("- %s (etapa %d): %s %v\n", action.Participant, action.Step, action.Kind, action.DocumentTypes)
		}

	default:
		fmt.Fprintf(os.Stderr, "Comando desconhecido: %s\n", os.Args[1])
		os.Exit(2)
	}
}

func fail(err error) {
	var rule *credicarro.BusinessRuleError
	if errors.As(err, &rule) {
		fmt.Fprintf(os.Stderr, "Regra de negócio não atendida (%s).\n", rule.ErrorCode)
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}
