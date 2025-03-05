package infra

import (
	"errors"
	"strings"
)

type Scope struct {
	Environment string
	Role        string
	Metadata    string
}

// ParseScope parses the scope to get the execution context (environment)
// Scope format must be: {environment}-{app role}[-{metadata}]
// The only requirement value is environment, the others are optionals.
// For example: test, test-search, develop-indexer, production-indexer-feature-new-context, default-pusher.
func ParseScope(scope string) (Scope, error) {
	// If we receive an empty scope, then we lack information for bootstrapping the server.
	if scope == "" {
		return Scope{}, errors.New("SCOPE is empty")
	}

	parts := strings.SplitN(strings.ToLower(scope), "-", 3)

	var env, role, metadata string
	switch len(parts) {
	case 1:
		// If scope has only 1 part, then we use it to load the environment for the current scope.
		env = parts[0]
	case 2:
		// If scope has 2 parts, then we use the first value load the environment and
		// the second to load the role value for the current scope.
		env, role = parts[0], parts[1]
	default:
		// If scope has 3 parts, then we use the first value load the environment,
		// the second to load the role value and the third value as metadata for the current scope.
		env, role, metadata = parts[0], parts[1], parts[2]
	}

	return Scope{
		Environment: env,
		Role:        role,
		Metadata:    metadata,
	}, nil
}
