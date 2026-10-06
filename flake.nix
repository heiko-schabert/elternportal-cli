{
  description = "elternportal-cli: Eltern-Portal from the terminal and as MCP server";

  inputs = {
    nixpkgs.url = "nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      ...
    }:
    let
      version = self.shortRev or self.dirtyShortRev or "dev";
      vendorHash = "sha256-XRhuWUmB8/a1Iu/Y+xA7yKob2zXAlIrzfKS1RrLYQhw=";
    in
    {
      nixosModules.default = import ./module.nix self;
    }
    // flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        buildGoModule = pkgs.buildGoModule.override { go = pkgs.go_latest; };

        elternportal-cli = buildGoModule {
          pname = "elternportal-cli";
          inherit version vendorHash;
          src = ./.;
          env.CGO_ENABLED = 0;
          ldflags = [
            "-s"
            "-w"
          ];
          nativeBuildInputs = [ pkgs.makeWrapper ];
          # Lets TestPDFText run instead of skipping.
          nativeCheckInputs = [ pkgs.poppler-utils ];
          # PDF text extraction shells out to pdftotext; ship it with the binary
          # so services work without a system-wide poppler.
          postInstall = ''
            wrapProgram $out/bin/elternportal-cli --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.poppler-utils ]}
          '';
          meta.mainProgram = "elternportal-cli";
        };
      in
      {
        packages = {
          inherit elternportal-cli;
          default = elternportal-cli;
        };

        apps.default = flake-utils.lib.mkApp { drv = elternportal-cli; };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go_latest
            pkgs.gopls
            pkgs.gotools
            pkgs.poppler-utils
          ];
        };

        formatter = pkgs.nixfmt;

        checks = {
          # The package build runs go test.
          build = elternportal-cli;
          gofmt = pkgs.runCommand "gofmt" { nativeBuildInputs = [ pkgs.go_latest ]; } ''
            unformatted=$(gofmt -l ${./.})
            [ -z "$unformatted" ] || { echo "not gofmt'ed: $unformatted"; exit 1; }
            touch $out
          '';
        };
      }
    );
}
