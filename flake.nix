{
  description = "Share file/text in your local network";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let

      version = "31";

      pkgs = import nixpkgs { system = "x86_64-linux"; };

    in
    {
      packages.x86_64-linux.local-content-share = pkgs.buildGoModule {
        pname = "local-content-share";
        inherit version;
        src = ./.;

        # no dependencies, hash must be null
        vendorHash = null;
      };

      meta = {
        mainProgram = "local-content-share";
        description = "Self-hosted app for storing/sharing text/files in your local network with no setup on client devices";
        homepage = "https://github.com/Tanq16/local-content-share";
      };

      packages.x86_64-linux.default = self.packages.x86_64-linux.local-content-share;
    };
}
