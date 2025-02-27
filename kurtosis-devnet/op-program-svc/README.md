# op-program-svc

This small service is a temporary measure until we come up with a better way
of generating/serving prestate files based on chain information.

# API

The API is intentionally extremely simple:
- `POST /`: generate new prestates from provided inputs
- `GET /HASH.(bin.gz|json)`: get prestate data
- `GET /info.json`: get prestates mapping

The idea is for this service to be basically a function
(chains_specs, deptsets) -> prestates.

In the future, we definitely want to replace the implementation of that
function (see implementation notes below)

## Trigger new build:

Example using curl

```
$ curl -X POST -H "Content-Type: multipart/form-data" \
    -F "files[]=@rollup-2151908.json" \
    -F "files[]=@rollup-2151909.json" \
    -F "files[]=@genesis-2151908.json" \
    -F "files[]=@genesis-2151909.json" \
    -F "files[]=@depsets.json" \
    http://localhost:8080
```

## Retrieve prestates mapping

```
$ curl -q http://localhost:8080/info.json
{
  "prestate": "0x03f4b7435fec731578c72635d8e8180f7b48703073d038fc7f8c494eeed1ce19",
  "prestate_interop": "0x034731331d519c93fc0562643e0728c43f8e45a0af1160ad4c57c4e5141d2bbb",
  "prestate_mt64": "0x0325bb0ca8521b468bb8234d8ba54b1b74db60e2b5bc75d0077a0fe2098b6b45"
}
```

## Implementation notes

Unfortunately, op-program-client relies on embedded (using `//go:embed`)
configuration files to store unannounced chain configs.

This means that in the context of devnets, we need to store the configs
(which are available only mid-deployment) into the **source tree** and
trigger a late build step.

So effectively, we need to package the relevant part of the sources into
a container, deploy that one alongside the devnet, and run that build step
on demand.

This is ugly, unsafe, easy to run a DOS against,... we need to do better.
But for now this is what we have.