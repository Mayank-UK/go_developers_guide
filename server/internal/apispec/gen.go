// Package apispec is the spec-first half of the api-documentation topic:
// the OpenAPI description at server/api/openapi.yaml is the contract's
// source of truth, and this package carries what code generation makes of
// it. types.gen.go below is GENERATED — oapi-codegen (pinned v2.8.1 as a
// go.mod tool) renders the spec's components/schemas as plain Go structs
// with stdlib-only types, and the output is committed so the repository
// diff shows exactly what a spec change costs in generated code.
//
// The discipline this package teaches: edit the YAML first, regenerate,
// and let the compiler walk the changes into every caller — the same
// contract-first loop protobuf and GraphQL codegen give their worlds,
// available to plain REST through OpenAPI.
package apispec

//go:generate go tool oapi-codegen -config cfg.yaml ../../api/openapi.yaml
