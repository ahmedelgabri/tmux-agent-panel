{
  description = "tmux-agent-panel (tap) - agent-aware tmux pane picker";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  outputs = inputs:
    inputs.flake-parts.lib.mkFlake {inherit inputs;} {
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];

      imports = [
        inputs.treefmt-nix.flakeModule
      ];

      perSystem = {
        pkgs,
        config,
        self',
        lib,
        ...
      }: {
        packages = {
          default = self'.packages.tap;
          tap = pkgs.buildGoModule {
            pname = "tap";
            version = "0.1.10";

            src = lib.cleanSource ./.;

            vendorHash = "sha256-dYdfxde+eiebCKMe6XwULLgd20jBOKMEHrIM+qK48dM=";

            ldflags = [
              "-s"
              "-w"
              "-X github.com/ahmedelgabri/tmux-agent-panel/internal/cmd.Version=${self'.packages.tap.version}"
            ];

            subPackages = ["cmd/tap"];

            meta = {
              mainProgram = "tap";
              homepage = "https://github.com/ahmedelgabri/tmux-agent-panel";
              description = "Agent-aware tmux pane picker with embedded fzf";
              license = lib.licenses.mit;
              platforms = lib.platforms.unix;
            };
          };
        };

        treefmt = {
          projectRootFile = "flake.nix";

          programs = {
            gofumpt.enable = true;
            prettier = {
              enable = true;
              includes = [
                "*.md"
                "*.yml"
                "*.yaml"
                "*.json"
              ];
            };
            alejandra.enable = true;
          };
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            git
            nixd
            bats
            tmux
            jq
            go
            gopls
            gofumpt
            go-tools # staticcheck, etc...
            govulncheck
            gotools # goimports
            just
          ];

          inputsFrom = [config.treefmt.build.devShell];
        };
      };
    };
}
