import 'git.just'

_EXTRALDFLAGS := if os() == "macos" { "-ldflags=-extldflags=-Wl,-ld_classic" } else { "" }

# We use both GOOS/GOARCH and TARGETOS/TARGETARCH to set the build targets.
# From the usage patterns, it looks like TARGETOS/TARGETARCH should take
# precedence if set, and default to GOOS/GOARCH if not set.
# TODO: should we just remove TARGETOS/TARGETARCH altogether eventually?
GOOS := env('GOOS', `go env GOOS`)
GOARCH := env('GOARCH', `go env GOARCH`)
TARGETOS := env('TARGETOS', GOOS)
TARGETARCH := env('TARGETARCH', GOARCH)

GORACE := "0"

_GORACE_FLAG := if GORACE == "1" { "-race " } else { "" }

[private]
go_build BIN PKG *FLAGS:
    env GO111MODULE=on GOOS={{TARGETOS}} GOARCH={{TARGETARCH}} CGO_ENABLED=0 go build -v {{_GORACE_FLAG}} {{FLAGS}} -o {{BIN}} {{PKG}}

[private]
go_test SELECTOR *FLAGS:
    go test -v {{_GORACE_FLAG}} {{FLAGS}} {{SELECTOR}}

[private]
go_fuzz FUZZ TIME='10s' PKG='': (go_test PKG _EXTRALDFLAGS "-fuzztime" TIME "-fuzz" FUZZ "-run" "NOTAREALTEST")

[private]
go_generate SELECTOR *FLAGS:
    go generate -v {{FLAGS}} {{SELECTOR}}