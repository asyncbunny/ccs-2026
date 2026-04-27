# Anon

This repository contains the source code for an anonymized blockchain
implementation submitted for double-blind academic review.

## Build and install

To build and install, you need to have Go 1.23 available. Follow the
instructions on the [Golang page](https://go.dev/doc/install) to do that.

To build the binary:

```console
make build
```

The binary will then be available at `./build/anond`.

To install the binary to system directories:

```console
make install
```

## Documentation

Technical documents are under [docs](./docs). Each module under `x/`
also contains a document about its design and implementation.
