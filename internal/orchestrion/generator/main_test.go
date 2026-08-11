package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestAllModuleDoesNotDependOnOrchestrion(t *testing.T) {
	const orchestrion = "github.com/GuanceCloud/orchestrion"

	var toolFile bytes.Buffer
	if err := fileTemplate.Execute(&toolFile, []string{"example.com/integration"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(toolFile.String(), orchestrion) {
		t.Fatalf("orchestrion tool template must not import %q", orchestrion)
	}

	var goMod bytes.Buffer
	err := goModTemplate.Execute(&goMod, map[string]any{
		"GoVersion":  "1.25.0",
		"Modules":    [][2]string{{"example.com/integration", "../../integration"}},
		"VersionTag": "v2.10.3-ext",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(goMod.String(), orchestrion) {
		t.Fatalf("go.mod template must not require %q", orchestrion)
	}
}
