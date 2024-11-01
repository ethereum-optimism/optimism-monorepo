# Use the old Apple linker to workaround broken xcode - https://github.com/golang/go/issues/65169
ifeq ($(shell uname),Darwin)
	FUZZLDFLAGS := -ldflags=-extldflags=-Wl,-ld_classic
endif

test:
	go test -v ./...

generate-mocks:
	go generate ./...

fuzz:
	printf "%s\n" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzExecutionPayloadUnmarshal ./eth" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzExecutionPayloadMarshalUnmarshalV1 ./eth" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzExecutionPayloadMarshalUnmarshalV2 ./eth" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzExecutionPayloadMarshalUnmarshalV3 ./eth" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzOBP01 ./eth" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzEncodeDecodeBlob ./eth" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzDetectNonBijectivity ./eth" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzEncodeScalar ./eth" \
	| parallel -j 8 {}

.PHONY: \
	test \
	generate-mocks \
	fuzz
