{ pkgs ? import <nixpkgs> {} }: with pkgs;
mkShell {
  buildInputs = [ go_1_22 jetbrains.goland ];
}
