# ultimate.nix — every capability module enabled.
# Personal-credential MCPs (Plex/TripIt/Hubstaff/Inoreader) ride along inert
# without their env vars — they exist in claude.json but fail when invoked.
# CELL-63 (Modules 2.0): ultimate = literally everything in the catalog.
{
  imports = [
    ./fullstack.nix
    ../modules/android.nix
    ../modules/desktop
    ../modules/electronics.nix
    ../modules/financial.nix
    ../modules/graphics.nix
    ../modules/llm
    ../modules/media
    ../modules/mise.nix
    ../modules/news.nix
    ../modules/nixos.nix
    ../modules/publishing.nix
    ../modules/security.nix
    ../modules/shell.nix
    ../modules/travel.nix
  ];

  # Enable every opt-in module imported above (or transitively via fullstack).
  # fullstack already enables: build, go, apple, infra, node, project-management,
  # python, qa-tools, scraping.
  devcell.modules.android.enable = true;
  devcell.modules.desktop.enable = true;
  devcell.modules.electronics.enable = true;
  devcell.modules.financial.enable = true;
  devcell.modules.graphics.enable = true;
  devcell.modules.news.enable = true;
  devcell.modules.nixos.enable = true;
  devcell.modules.publishing.enable = true;
  devcell.modules.security.enable = true;
  devcell.modules.travel.enable = true;
  devcell.modules.plex.enable = true;  # from ../modules/media
}
