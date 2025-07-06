{
  description = "Share file/text in your local network";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs = { self, nixpkgs }: 
    let
      version = "31";

      pkgsFor = system: import nixpkgs { inherit system; };

      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];

      forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: f (pkgsFor system));
    in
    {
      packages = forAllSystems (pkgs: rec {
        local-content-share = pkgs.buildGoModule {
          pname = "local-content-share";
          inherit version;
          src = ./.;
          vendorHash = null;
        };

        # set default to our package for convenience
        default = local-content-share;
      });

      meta = {
        mainProgram = "local-content-share";
        description = "Self-hosted app for storing/sharing text/files in your local network with no setup on client devices";
        homepage = "https://github.com/Tanq16/local-content-share";
      };
    };
}
