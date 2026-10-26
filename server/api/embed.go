// Package api exposes the guide's OpenAPI description of the posts API as
// a build-time asset: openapi.yaml rides the binary via go:embed, so the
// spec-serving route, the docs-link demo's facts, and the drift tests all
// read the SAME bytes from the SAME file — the document cannot drift from
// the copy any consumer renders.
package api

import _ "embed"

// OpenAPIYAML is the verbatim openapi.yaml beside this file, embedded at
// build time. It is the single in-binary copy of the contract.
//
//go:embed openapi.yaml
var OpenAPIYAML []byte

// SpecPath is the spec's repo-root-relative location — what teaching
// prose and error messages cite.
const SpecPath = "server/api/openapi.yaml"

// SpecURL is the route that serves the spec verbatim (wired in
// cmd/guide/main.go with the presentation handler).
const SpecURL = "/api/openapi.yaml"
