# nixhome/packages/cell.nix — devcell CLI (Go binary)
#
# Built from the repo root via `buildGoModule`. The source is passed in
# as `src` — the flake resolves it from `self + "/.."`  for local builds
# or from a GitHub fetch for remote builds.
{
  pkgs,
  lib,
  src,
  version ? "0.0.0",
}:
pkgs.buildGoModule {
  pname = "cell";
  inherit version src;

  vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";

  subPackages = ["cmd"];

  # Swagger docs are generated at build time by the Dockerfile path;
  # the nix derivation doesn't need them (cell serve imports the docs
  # package, but `go build` with the generate step skipped still compiles
  # because docs/ is gitignored and the import is guarded).
  preBuild = ''
    mkdir -p docs
    cat > docs/docs.go << 'EOF'
    package docs
    EOF
  '';

  # Rename the binary from "cmd" to "cell"
  postInstall = ''
    mv $out/bin/cmd $out/bin/cell
  '';

  meta = with lib; {
    description = "devcell CLI — container-native dev environments";
    homepage = "https://github.com/DimmKirr/devcell";
    license = licenses.mit;
    mainProgram = "cell";
  };
}
