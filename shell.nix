{ pkgs ? import <nixpkgs> { } }:
let
  unstable = import <unstable> { };
in
pkgs.mkShell {
  buildInputs = [
    unstable.go_1_27
    unstable.jetbrains.goland
  ];
  hardeningDisable = [ "fortify" ];
}
