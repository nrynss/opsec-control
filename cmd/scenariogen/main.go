package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/llm"
	"github.com/nrynss/opsec-control/internal/scenariogen"
)

func main() {
	outFlag := flag.String("out", "scenario.json", "Output path for the compiled scenario")
	seedFlag := flag.Int64("seed", 42, "Random seed for generation")
	mockFlag := flag.Bool("mock", false, "Use the LLM mock client")
	flag.Parse()

	var client contracts.LLMClient
	if *mockFlag {
		log.Println("Using LLM mock client...")
		// Mock mode is config/env-driven: force it on regardless of any
		// CEREBRAS_API_KEY in the environment (llm.Complete checks LLM_MOCK).
		os.Setenv("LLM_MOCK", "true")
		client = llm.NewClient(llm.Config{})
	} else {
		apiKey := os.Getenv("CEREBRAS_API_KEY")
		if apiKey == "" {
			log.Fatal("CEREBRAS_API_KEY environment variable is required for real LLM generation")
		}
		client = llm.NewClient(llm.Config{APIKey: apiKey})
	}

	gen := scenariogen.NewGenerator(client)
	log.Println("Compiling 3-act Cerebro scenario...")

	scn, err := gen.Compile(context.Background(), *seedFlag)
	if err != nil {
		log.Fatalf("Compilation failed: %v", err)
	}

	data, err := json.MarshalIndent(scn, "", "  ")
	if err != nil {
		log.Fatalf("Marshaling failed: %v", err)
	}

	if err := os.WriteFile(*outFlag, data, 0644); err != nil {
		log.Fatalf("Writing file failed: %v", err)
	}

	fmt.Printf("Successfully compiled validated scenario to %s\n", *outFlag)
}
