# python.nix — Python runtime and uv package manager
{pkgs, ...}: {
  # Mise-managed python: lets users pick versions per-project via .tool-versions
  # / .python-version and gets a shim at $HOME/.local/share/mise/shims/python.
  # nix-provided python3 (below) still ships as a fallback in the nix profile so
  # build-time scripts have a Python without waiting on mise install at runtime.
  devcell.mise.tools.python = "3.13.2";

  home.packages = with pkgs; [
    python3
    uv
  ];
}
