{ pkgs ? import <nixpkgs> { } }:
let
  unstable = import <unstable> { };
in
pkgs.mkShell {
  buildInputs = [
    unstable.go_1_22
    unstable.jetbrains.goland
    pkgs.atlas
  ];
  hardeningDisable = [ "fortify" ];
}
