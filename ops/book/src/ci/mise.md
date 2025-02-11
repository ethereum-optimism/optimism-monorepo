# Mise

Our build environment is managed through a tool called [mise][mise]. Mise provides a convenient way to install and
manage all necessary dependencies for building and testing the packages in this repository. It helps guarantee that
every job in the pipeline runs using the same set tools, which leads to more deterministic builds and fewer
environment-related issues.

Mise defines a list of required software (like Go, Foundry, etc.) in a configuration file called mise.toml. The
CI pipeline then runs mise at the beginning of every job in order to ensure that dependencies are installed at their
proper versions. Therefore, **all build-time dependencies must be defined in mise.toml.** For this reason, we
recommend developers use mise for local development as well.

[mise]: https://mise.jdx.dev/

## Adding New Mise Dependencies

For the most part, adding a new Mise dependency is as straightforward as adding a new package to `mise.toml`.
However, some packages require some additional configuration.

### Aliased Packages

GitHub packages that expose multiple executables or where the executable name is different from the package name
will require an alias to be defined in `mise.toml`. To do this:

1. Add an alias named after the package to the `[alias]` stanza.
2. Configure the alias to point to the package repository and specify the executable at the end 
   as `[exe=<your-executable>]`, e.g.`ubi:goreleaser/goreleaser-pro[exe=goreleaser]`.
3. Add your package and its version to the list of tools in the `[tools]` stanza.