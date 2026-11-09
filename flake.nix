{
  description = "google-domain-detector — which domains run Google Workspace or Google Cloud, from public DNS alone";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

    go-nix-helpers = {
      url = "github:LarsArtmann/go-nix-helpers";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs@{
      flake-parts,
      ...
    }:
    let
      lib = inputs.nixpkgs.lib;
      packageSrc = lib.fileset.toSource {
        root = ./.;
        fileset = lib.fileset.gitTracked ./.;
      };
    in
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ inputs.go-nix-helpers.flakeModules.go-standard ];

      go-standard = {
        pname = "google-domain-detector";
        description = "Detect Google Workspace and Google Cloud usage for a list of domains, from public DNS alone";
        goPkgAttr = "go_1_27";
        systems = [
          "x86_64-linux"
          "aarch64-linux"
        ];
        src = packageSrc;
        subPackages = [ "cmd/google-domain-detector" ];
        vendorHash = "sha256-qsUM2pa4W87ypF8/8eJrb72iffzU6bfUpYygkL2puIw=";
      };
    };
}
