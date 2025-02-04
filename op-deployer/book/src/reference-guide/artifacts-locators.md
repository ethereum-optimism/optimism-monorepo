# Artifacts Locators

OP Deployer calls into precompiled contract artifacts. To make this work, OP Deployer uses artifacts locators to
point to the location of contract artifacts. While locators are nothing more than URLs, they do encode some
additional behaviors which are described here.

## Locator Types

Locators can be one of three types:

- `tag://` locators, which point to a versioned contracts release. These resolve to a known URL. Artifacts
  downloaded using a tagged locator are validated against a hardcoded checksum in the OP Deployer implementation.
  This prevents tampering with the contract artifacts once they have been tagged. Additionally, tagged locators are
  cached on disk to avoid repeated downloads.
- `https://` locators, which point to a tarball of contract artifacts somewhere on the web. HTTP locators are cached
  just like tagged locators are, but they are not validated against a checksum.
- `file://` locators, which point to a directory on disk containing the artifacts.

## Version Hints

OP Deployer supports multiple different contract versions at the same time. Sometimes, contracts at version X are
backwards-incompatible with version Y. OP Deployer will support both versions at the same time when this happens.
However, OP Deployer needs to know which versioning behavior to use with each locator. For `tag` locators this is
easy since the behavior is encoded in the tag itself. However, it's more complicated for `https` and `file` locators.

To support multiple versions of each contract, OP Deployer supports specifying _version hints_ in the locator. These 
hints are URL fragments (e.g., the part of the URL that comes after the `#` symbol) denoting how OP Deployer should 
treat the artifacts at that URL. For example, the URL `https://example.com/artifacts.tar.gz#v1` would treat the 
artifacts at the URL with the versioning behavior of version `v1`.

This only applies to `https` and `file` locators. `tag` locators are versioned by the tag itself, and any hints will 
be ignored.