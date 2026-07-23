{
  description = "Pottery Shop development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [ "aarch64-darwin" "x86_64-darwin" "x86_64-linux" "aarch64-linux" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Map Nix system to the OS/ARCH strings used by Replicated and
        # Troubleshoot release tarballs.
        platform = {
          aarch64-darwin = "darwin_all";
          x86_64-darwin = "darwin_all";
          x86_64-linux = "linux_amd64";
          aarch64-linux = "linux_arm64";
        }.${system};

        replicated = pkgs.stdenv.mkDerivation rec {
          pname = "replicated";
          version = "0.129.11";
          src = pkgs.fetchurl {
            url = "https://github.com/replicatedhq/replicated/releases/download/v${version}/replicated_${version}_${platform}.tar.gz";
            sha256 = {
              aarch64-darwin = "05xq6r325w1s7s6fib4af27mdfvhk53h5w7ynwh0lba721sda72a";
              x86_64-darwin = "05xq6r325w1s7s6fib4af27mdfvhk53h5w7ynwh0lba721sda72a";
              x86_64-linux = "0fwcljv028p349b5zz5zdk2s8p8ixxm3z2g9a0zjv39wjmah1jnp";
              aarch64-linux = "1cvhyvcjxmrmbg003jsavxksm4xkjmjr1avydnzwbj6gg6ic9sjv";
            }.${system};
          };
          sourceRoot = ".";
          phases = [ "unpackPhase" "installPhase" ];
          installPhase = ''
            install -D -m 755 replicated $out/bin/replicated
          '';
        };

        preflight = pkgs.stdenv.mkDerivation rec {
          pname = "preflight";
          version = "0.130.1";
          src = pkgs.fetchurl {
            url = "https://github.com/replicatedhq/troubleshoot/releases/download/v${version}/preflight_${platform}.tar.gz";
            sha256 = {
              aarch64-darwin = "1qhaar2j6f104mxyrbk45dvybzz68s29nqg0v68plvh22j13qa72";
              x86_64-darwin = "1qhaar2j6f104mxyrbk45dvybzz68s29nqg0v68plvh22j13qa72";
              x86_64-linux = "06nmml0lm84jh4ki3zqd9yzq46ahx1ic757i6f5njqcys0cm607m";
              aarch64-linux = "1awbwrd0wjq6v1g4jw6lil21i161s88hy637q9wll6ypwaczw12q";
            }.${system};
          };
          sourceRoot = ".";
          phases = [ "unpackPhase" "installPhase" ];
          installPhase = ''
            install -D -m 755 preflight $out/bin/preflight
          '';
        };

        support-bundle = pkgs.stdenv.mkDerivation rec {
          pname = "support-bundle";
          version = "0.130.1";
          src = pkgs.fetchurl {
            url = "https://github.com/replicatedhq/troubleshoot/releases/download/v${version}/support-bundle_${platform}.tar.gz";
            sha256 = {
              aarch64-darwin = "1qqwzaywya3wkag7zbqa85pwzvp90vbmcdrslhqcgxmrkvi4gmld";
              x86_64-darwin = "1qqwzaywya3wkag7zbqa85pwzvp90vbmcdrslhqcgxmrkvi4gmld";
              x86_64-linux = "0ch37wxajnaqhrihbdxzqbyzmfnmjblr20npb228gxwcy0wsm64i";
              aarch64-linux = "0l9587wqzs7wzp2gkpps8zjahk76vy0g44d09fagh805wrrpw3mr";
            }.${system};
          };
          sourceRoot = ".";
          phases = [ "unpackPhase" "installPhase" ];
          installPhase = ''
            install -D -m 755 support-bundle $out/bin/support-bundle
          '';
        };
      in {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_26
            git
            gnumake
            python3
            docker
            kubernetes-helm
            kubectl
            k3d
            jq
            replicated
            preflight
            support-bundle
          ];

          shellHook = ''
            echo "Pottery Shop dev shell — $(go version)"
            echo "Tools available: make, docker, helm, kubectl, k3d, jq, replicated, preflight, support-bundle"
          '';
        };
      });
}
