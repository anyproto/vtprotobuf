module github.com/planetscale/vtprotobuf

go 1.22

toolchain go1.24.0

require (
	github.com/stretchr/testify v1.8.4
	google.golang.org/grpc v1.64.1
	google.golang.org/protobuf v1.36.5
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace google.golang.org/protobuf => github.com/anyproto/protobuf-go v0.0.0-20250314161123-d58efe595bdd
