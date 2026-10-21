// Mescla múltiplas SARIF runs em uma só antes do upload.
//
// O GitHub Code Scanning recusa um results.sarif com mais de uma run sob a mesma
// categoria (desde 2025-07-21). O Codacy CLI às vezes gera mais de uma run no mesmo
// arquivo — este script combina rules e results de todas as runs numa única, sem
// depender de adivinhar por que o Codacy as separa (mesma causa raiz e correção já
// confirmadas nos SDKs Node, Python e PHP).
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Uso: mergesarif <arquivo.sarif>")
		os.Exit(1)
	}
	path := os.Args[1]

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var sarif map[string]interface{}
	if err := json.Unmarshal(data, &sarif); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	runs, _ := sarif["runs"].([]interface{})
	if len(runs) <= 1 {
		fmt.Printf("[mergesarif] %s já tem %d run(s), nada a fazer.\n", path, len(runs))
		return
	}

	first, _ := runs[0].(map[string]interface{})
	rest := runs[1:]

	tool, _ := first["tool"].(map[string]interface{})
	driver, _ := tool["driver"].(map[string]interface{})
	rules, _ := driver["rules"].([]interface{})

	rulesByID := map[string]interface{}{}
	ruleOrder := []string{}
	for _, r := range rules {
		rule, _ := r.(map[string]interface{})
		id, _ := rule["id"].(string)
		if _, exists := rulesByID[id]; !exists {
			ruleOrder = append(ruleOrder, id)
		}
		rulesByID[id] = rule
	}

	results, _ := first["results"].([]interface{})

	for _, r := range rest {
		run, _ := r.(map[string]interface{})
		runTool, _ := run["tool"].(map[string]interface{})
		runDriver, _ := runTool["driver"].(map[string]interface{})
		runRules, _ := runDriver["rules"].([]interface{})

		for _, rr := range runRules {
			rule, _ := rr.(map[string]interface{})
			id, _ := rule["id"].(string)
			if _, exists := rulesByID[id]; !exists {
				rulesByID[id] = rule
				ruleOrder = append(ruleOrder, id)
			}
		}

		runResults, _ := run["results"].([]interface{})
		results = append(results, runResults...)
	}

	mergedRules := make([]interface{}, 0, len(ruleOrder))
	for _, id := range ruleOrder {
		mergedRules = append(mergedRules, rulesByID[id])
	}

	driver["rules"] = mergedRules
	first["results"] = results
	sarif["runs"] = []interface{}{first}

	out, err := json.Marshal(sarif)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.WriteFile(path, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("[mergesarif] Mescladas %d runs em 1 (%d resultados) em %s.\n", 1+len(rest), len(results), path)
}
