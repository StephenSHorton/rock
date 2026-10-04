package version

// Version is injected on a tagged release with
//
//	-ldflags "-X github.com/StephenSHorton/rock/internal/version.Version=1.0.1"
//
// Local and `go install` builds keep the dev fallback.
var Version = "dev"
