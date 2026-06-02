# codex.nix — Codex CLI MCP server staging and entrypoint merge logic.
# Extracted from managed-mcp.nix.
{
  pkgs,
  pkgsEdge,
  lib,
  config,
  ...
}: let
  mcpCfg = config.devcell.managedMcp;

  toml = pkgs.formats.toml {};

  # Only stdio servers — Codex doesn't support HTTP transport.
  # Also skip servers explicitly disabled (enabled = false). Default: enabled.
  stdioServers = lib.filterAttrs (
    _: s: (s.type or "stdio") == "stdio" && (s.enabled or true)
  ) mcpCfg.servers;

  toCodexServer = _: s:
    {
      command = s.command;
      args = s.args or [];
    }
    // lib.optionalAttrs ((s.env or {}) != {}) {env = s.env;};

  codexConfig = toml.generate "codex-nix-mcp-servers.toml" {
    backupBeforeMerge = mcpCfg.backupBeforeMerge;
    mcp_servers = lib.mapAttrs toCodexServer stdioServers;
  };

  hasServers = mcpCfg.servers != {};
in {
  options.devcell.managedCodex = {
    # Read-only — exposes the generated config derivation so the pure
    # (nix2container) image builder can stage it directly to /etc/codex/ at
    # image-build time. Activation-script-based staging (line ~49 below)
    # doesn't run on pure images because home-manager activation is skipped.
    nixMcpConfigFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = if hasServers then codexConfig else null;
      internal = true;
      readOnly = true;
      description = "Nix-store path of the generated Codex MCP servers TOML (null when no servers declared).";
    };
  };

  config = {
    home.packages = [ pkgsEdge.codex ];

    # Always generate the Codex merge fragment (self-guards at runtime)
    home.file.".config/devcell/entrypoint.d/30-codex.sh" = {
      executable = true;
      source = ../fragments/30-codex.sh;
    };

    # Stage Codex MCP config when servers are defined
    home.activation.setupManagedCodex = lib.mkIf hasServers (
      lib.hm.dag.entryAfter ["writeBoundary"] ''
        export PATH="/usr/bin:/bin:$PATH"
        $DRY_RUN_CMD sudo mkdir -p /etc/codex
        $DRY_RUN_CMD sudo rm -f /etc/codex/managed_config.toml
        $DRY_RUN_CMD sudo cp ${codexConfig} /etc/codex/nix-mcp-servers.toml
      ''
    );
  };
}
