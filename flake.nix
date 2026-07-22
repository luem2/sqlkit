{
  description = "SQL workflow CLI";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs =
    { nixpkgs, ... }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      packages.${system}.default = pkgs.buildGoModule {
        pname = "sqlkit";
        version = "0.1.0";
        src = ./.;
        vendorHash = "sha256-UZPg6kDxstMeSRvHDS76VtxtJw6LoIsZiyQi/hKdK1w=";
        subPackages = [ "cmd/sqlkit" ];
      };

      devShells.${system}.default = pkgs.mkShell {
        packages = with pkgs; [
          go
          gopls
          gotools
        ];
      };
    };
}
