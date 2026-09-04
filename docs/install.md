# Install

## From source

```sh
git clone https://github.com/Esturban/bfr.git
cd bfr
go build -o bfr .
```

## Via go install

If the module is already available to your Go toolchain:

```sh
go install github.com/Esturban/bfr@latest
```

Requires Go 1.21+.

## From a release

Each tagged release publishes prebuilt binaries for darwin (amd64, arm64)
and linux (amd64, arm64) via [goreleaser](https://goreleaser.com), attached
to the [GitHub release](https://github.com/Esturban/bfr/releases) for that
tag, alongside a `checksums.txt`.

```sh
curl -LO https://github.com/Esturban/bfr/releases/latest/download/bfr_darwin_arm64.tar.gz
tar -xzf bfr_darwin_arm64.tar.gz
./bfr version
```
