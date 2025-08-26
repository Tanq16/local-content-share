{
  description = "Share file/text in your local network";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs =
    {
      self,
      nixpkgs,
      ...
    }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f system);
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          local-content-share = pkgs.buildGoModule {
            pname = "local-content-share";
            version = ""; # nix needs version to be set, though setting it to the right one would mean it needs to be updated at each release
            src = ./.;
            vendorHash = null;

            meta = {
              mainProgram = "local-content-share";
              description = "Self-hosted app for storing/sharing text/files in your local network with no setup on client devices";
              homepage = "https://github.com/Tanq16/local-content-share";
            };
          };
          default = self.packages.${system}.local-content-share;
        }
      );

      nixosModules.local-content-share =
        {
          pkgs,
          lib,
          config,
          ...
        }:
        let
          cfg = config.services.local-content-share;
        in
        {
          options.services.local-content-share = {
            enable = lib.mkEnableOption "Local-Content-Share";

            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.local-content-share;
            };

            port = lib.mkOption {
              type = lib.types.port;
              default = 8080;
              description = "Port on which the service will be available";
            };

            openFirewall = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Open chosen port";
            };
          };

          config = lib.mkIf cfg.enable {
            systemd.services.local-content-share = {
              description = "Local-Content-Share";
              after = [ "network.target" ];
              wantedBy = [ "multi-user.target" ];

              serviceConfig = {
                Type = "simple";
                DynamicUser = true;
                StateDirectory = "local-content-share";
                WorkingDirectory = "/var/lib/local-content-share";
                ExecStart = "${lib.getExe' cfg.package "local-content-share"} -listen=:${toString cfg.port}";
                Restart = "on-failure";
              };
            };

            networking.firewall = lib.mkIf cfg.openFirewall {
              allowedTCPPorts = [ cfg.port ];
            };
          };
        };

      nixosModules.default = self.nixosModules.local-content-share;
    };
}
