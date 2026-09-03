// Package featuregate owns stable identifiers for opt-in product surfaces.
// Keeping these identifiers independent of config and transports prevents the
// CLI, operation registry, MCP server, and monitors from naming the same
// rolling feature differently.
package featuregate

const PaperTrading = "paper-trading"
