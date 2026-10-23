package domain

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"testing"
)

var summaryMutex sync.Mutex
var passedTests int
var skippedTests int
var failedTests []string

type testResult struct {
	test *testing.T
}

// registerTestSummary agenda o registro do resultado ao terminar o teste.
// Chame esta função no início de cada teste que deve entrar no resumo.
func registerTestSummary(test *testing.T) {
	result := testResult{test: test}
	test.Cleanup(result.record)
}

// record também é executado quando o teste termina com t.Fatal ou t.Fatalf.
func (result testResult) record() {
	summaryMutex.Lock()
	defer summaryMutex.Unlock()

	if result.test.Failed() {
		failedTests = append(failedTests, result.test.Name())
		return
	}

	if result.test.Skipped() {
		skippedTests++
		return
	}

	passedTests++
}

func printTestSummary() {
	sort.Strings(failedTests)

	fmt.Println()
	fmt.Println("Resumo dos testes registrados:")
	fmt.Printf("Aprovados: %d\n", passedTests)
	fmt.Printf("Falhas: %d\n", len(failedTests))
	fmt.Printf("Ignorados: %d\n", skippedTests)

	for _, name := range failedTests {
		fmt.Printf("- FALHOU: %s\n", name)
	}
}

// TestMain executa os testes do pacote e mostra o resumo ao final.
func TestMain(tests *testing.M) {
	exitCode := tests.Run()
	printTestSummary()
	os.Exit(exitCode)
}
