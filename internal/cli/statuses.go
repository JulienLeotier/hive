package cli

// healthyStatus is the stringified `healthy` health state used across CLI
// flows. Centralised here so the linter doesn't keep flagging the same
// literal duplication across serve/agent/doctor.
const healthyStatus = "healthy"
