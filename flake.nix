{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    foundry.url = "github:shazow/foundry.nix/monthly";
  };


  outputs = inputs:
    inputs.flake-utils.lib.eachDefaultSystem (system:
      let
      overlays = [
        inputs.foundry.overlay
      ];
      pkgs = import inputs.nixpkgs { inherit overlays system;};
      in
      {
        devShell = pkgs.mkShell {
          packages = [
            pkgs.jq
            pkgs.yq-go
            pkgs.uv
            pkgs.shellcheck
            pkgs.python311
            pkgs.foundry-bin
            pkgs.just
            pkgs.go
            pkgs.gotools
          ];
        };
      }
    );
}
