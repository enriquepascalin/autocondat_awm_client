module github.com/enriquepascalin/awm-cli

go 1.22

require (
	github.com/enriquepascalin/awm-orchestrator v0.0.0
	google.golang.org/grpc v1.80.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/enriquepascalin/awm-orchestrator => ../awm-orchestratorgit