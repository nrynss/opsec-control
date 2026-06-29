package llm

import (
	"context"
	"testing"

	"github.com/nrynss/opsec-control/internal/contracts"
)

func TestPerceptionMock(t *testing.T) {
	t.Setenv("LLM_MOCK", "true")
	client := NewClient(Config{
		APIKey:     "", // Trigger mock mode
		BaseURL:    "",
		Model:      "",
		MaxRetries: 1,
	})

	tests := []struct {
		name       string
		input      contracts.ImageInput
		wantType   contracts.EventType
		wantSource string
	}{
		{
			name: "Vora bridge collapsed",
			input: contracts.ImageInput{
				Source: "drone",
				Data:   []byte("drone_vora_bridge_collapsed.png"),
			},
			wantType:   contracts.EventBridgeCollapsed,
			wantSource: "Cerebras-Perception-drone",
		},
		{
			name: "Highgate masonry collapse",
			input: contracts.ImageInput{
				Source: "satellite",
				Data:   []byte("satellite_highgate_masonry_collapse.png"),
			},
			wantType:   contracts.EventBuildingCollapsed,
			wantSource: "Cerebras-Perception-satellite",
		},
		{
			name: "Southport levee breach",
			input: contracts.ImageInput{
				Source: "drone",
				Data:   []byte("drone_southport_levee_breach.png"),
			},
			wantType:   contracts.EventLeveeBreached,
			wantSource: "Cerebras-Perception-drone",
		},
		{
			name: "Default drone generic",
			input: contracts.ImageInput{
				Source: "drone",
				Data:   []byte("unknown_image_bytes"),
			},
			wantType:   contracts.EventRoadBlocked,
			wantSource: "Cerebras-Perception-drone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := client.Interpret(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("Interpret failed: %v", err)
			}
			if len(events) == 0 {
				t.Fatal("expected at least one event, got 0")
			}
			ev := events[0]
			if ev.Type != tt.wantType {
				t.Errorf("expected event type %q, got %q", tt.wantType, ev.Type)
			}
			if ev.Source != tt.wantSource {
				t.Errorf("expected event source %q, got %q", tt.wantSource, ev.Source)
			}
		})
	}
}
