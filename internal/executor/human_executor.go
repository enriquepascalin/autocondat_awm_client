package executor

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	awmv1 "github.com/enriquepascalin/awm-orchestrator/internal/proto/awm/v1"
)

type HumanExecutor struct{}

func NewHumanExecutor() *HumanExecutor {
	return &HumanExecutor{}
}

func (h *HumanExecutor) Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error) {
	fmt.Printf("\n=== New Task: %s ===\n", task.ActivityName)
	fmt.Printf("Input: %v\n", task.Input.AsMap())
	fmt.Print("Enter result (key=value pairs, comma separated): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	result := make(map[string]interface{})
	for _, pair := range strings.Split(input, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result, nil
}
