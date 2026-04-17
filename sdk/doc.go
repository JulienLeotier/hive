// Package sdk is the official Go client for the Hive API.
//
// It wraps `/api/v1/*` endpoints in typed methods so callers can manage
// agents, fire workflows, retry tasks, and query events without hand-rolling
// HTTP plumbing. The SDK keeps zero imports from internal/* — its types live
// here so the import graph stays clean and the package can be vendored.
//
// Basic usage:
//
//	c := sdk.NewClient("https://hive.example.com", "hive_YOUR_API_KEY")
//	agents, err := c.Agents().List(ctx)
//	if err != nil { ... }
//	for _, a := range agents {
//	    fmt.Println(a.Name, a.HealthStatus)
//	}
//
// All methods take a context for cancellation. Non-2xx responses surface as
// *sdk.APIError so callers can switch on Code ("NOT_FOUND", "BAD_REQUEST",
// etc.) without parsing error messages.
package sdk
