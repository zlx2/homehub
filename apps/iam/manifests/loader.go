package manifests

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"gitee.com/zlx23/homehub/apps/iam/internal/domain"
)

//go:embed iam.json
var iamJSON []byte

//go:embed control.json
var controlJSON []byte

//go:embed drop.json
var dropJSON []byte

//go:embed ai-gateway.json
var aiGatewayJSON []byte

func Builtin() ([]domain.ServiceManifest, error) {
	inputs := [][]byte{iamJSON, controlJSON, dropJSON, aiGatewayJSON}
	result := make([]domain.ServiceManifest, 0, len(inputs))
	for _, input := range inputs {
		var manifest domain.ServiceManifest
		if err := json.Unmarshal(input, &manifest); err != nil {
			return nil, fmt.Errorf("decode built-in service manifest: %w", err)
		}
		if err := manifest.Validate(); err != nil {
			return nil, fmt.Errorf("validate built-in service manifest %q: %w", manifest.ServiceID, err)
		}
		result = append(result, manifest)
	}
	return result, nil
}
