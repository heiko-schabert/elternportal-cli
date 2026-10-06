self:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.elternportal-cli;
in
{
  options.services.elternportal-cli = {
    enable = lib.mkEnableOption "Eltern-Portal MCP server over Streamable HTTP";
    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
    };
    listen = lib.mkOption {
      type = lib.types.str;
      example = "100.64.0.1:8080";
      description = "No auth of its own: bind to a private interface only.";
    };
    environmentFile = lib.mkOption {
      type = lib.types.path;
      description = "ELTERNPORTAL_URL/USER/PASSWORD; kept out of the store.";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.elternportal-cli = {
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      serviceConfig = {
        ExecStart = "${lib.getExe cfg.package} mcp --http ${cfg.listen}";
        EnvironmentFile = cfg.environmentFile;
        DynamicUser = true;
        Restart = "on-failure";
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        NoNewPrivileges = true;
      };
    };
  };
}
