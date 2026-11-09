package docs

import _ "embed"

// OpenAPI is the OpenAPI specification served by the API.
//
//go:embed openapi.yaml
var OpenAPI string
