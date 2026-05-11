package version

// Version is overridden at build time via -ldflags. "dev" is the fallback for
// go test, go run, and any build that omits ldflags.
var Version = "dev"
