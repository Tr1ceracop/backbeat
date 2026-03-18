{
  description = "Backbeat - Active time tracker for Jira Tempo";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      packages.${system}.default = pkgs.buildGoModule {
        pname = "backbeat";
        version = "0.1.0";
        src = ./.;
        vendorHash = "sha256-HdwDd2N6b7Cg3tlkzUVDYwc3SOhKAG4gWra15pg6EP8=";
      };

      # Overlay so the module can reference pkgs.backbeat
      overlays.default = final: prev: {
        backbeat = self.packages.${final.system}.default;
      };

      # Home Manager module
      homeManagerModules.default = import ./nix/hm-module.nix;

      devShells.${system}.default = pkgs.mkShell {
        name = "backbeat";
        buildInputs = with pkgs; [
          go
          gopls
          golangci-lint
          sqlite
        ];
      };
    };
}
