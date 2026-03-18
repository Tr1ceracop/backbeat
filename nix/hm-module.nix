{ config, lib, pkgs, ... }:

let
  cfg = config.services.backbeat;
in
{
  options.services.backbeat = {
    enable = lib.mkEnableOption "Backbeat active time tracker for Jira Tempo";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.backbeat;
      description = "The backbeat package to use.";
    };

    settings = {
      idleThreshold = lib.mkOption {
        type = lib.types.str;
        default = "5m";
        description = "Duration of inactivity before marking user as idle.";
      };

      pollInterval = lib.mkOption {
        type = lib.types.str;
        default = "30s";
        description = "How often to check for activity.";
      };

      minSession = lib.mkOption {
        type = lib.types.str;
        default = "1m";
        description = "Minimum session duration to keep (shorter sessions are discarded).";
      };
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];

    systemd.user.services.backbeat = {
      Unit = {
        Description = "Backbeat - Active Time Tracker";
        After = [ "graphical-session.target" ];
      };
      Service = {
        Type = "simple";
        ExecStart = "${cfg.package}/bin/backbeat start";
        Restart = "on-failure";
        RestartSec = 5;
      };
      Install = {
        WantedBy = [ "graphical-session.target" ];
      };
    };
  };
}
