{
  description = "flowctl - Control plane for data pipeline orchestration";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        
        # Go version to use
        go = pkgs.go_1_23;
        
        # Build dependencies
        buildInputs = with pkgs; [
          # Go and build tools
          go
          gotools
          go-tools
          gopls
          delve
          
          # Protobuf compiler and plugins
          protobuf
          protoc-gen-go
          protoc-gen-go-grpc
          
          # Development tools
          gnumake
          git
          
          # Optional: for documentation
          mdbook
        ];
        
        # flowctl package
        flowctl = pkgs.buildGoModule rec {
          pname = "flowctl";
          version = "0.1.0";
          
          src = ./.;
          
          vendorHash = null; # Set to null for now, will need to be updated
          
          nativeBuildInputs = [ pkgs.protobuf ];
          
          preBuild = ''
            # Generate protobuf files
            protoc --go_out=. --go_opt=paths=source_relative \
              --go-grpc_out=. --go-grpc_opt=paths=source_relative \
              proto/*.proto
          '';
          
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];
          
          meta = with pkgs.lib; {
            description = "Control plane for data pipeline orchestration";
            homepage = "https://github.com/withObsrvr/flowctl";
            license = licenses.mit;
            maintainers = [ ];
          };
        };
        
        # Development shell script for generating protobuf
        generateScript = pkgs.writeScriptBin "generate-proto" ''
          #!${pkgs.stdenv.shell}
          echo "Generating protobuf files..."
          ${pkgs.protobuf}/bin/protoc \
            --go_out=. --go_opt=paths=source_relative \
            --go-grpc_out=. --go-grpc_opt=paths=source_relative \
            proto/*.proto
          echo "Done!"
        '';
        
        # Development shell script for running flowctl
        runScript = pkgs.writeScriptBin "run-flowctl" ''
          #!${pkgs.stdenv.shell}
          go run ./cmd/flowctl "$@"
        '';
        
      in
      {
        # Package outputs
        packages = {
          default = flowctl;
          flowctl = flowctl;
        };
        
        # App outputs
        apps = {
          default = flake-utils.lib.mkApp {
            drv = flowctl;
          };
          flowctl = flake-utils.lib.mkApp {
            drv = flowctl;
          };
        };
        
        # Development shell
        devShells.default = pkgs.mkShell {
          buildInputs = buildInputs ++ [
            generateScript
            runScript
          ];
          
          shellHook = ''
            echo "flowctl development environment"
            echo "Available commands:"
            echo "  generate-proto - Generate protobuf Go files"
            echo "  run-flowctl    - Run flowctl using 'go run'"
            echo "  go test        - Run tests"
            echo "  go build       - Build flowctl binary"
            echo ""
            echo "Protoc version: $(protoc --version)"
            echo "Go version: $(go version)"
            
            # Set up Go environment
            export GOPATH="$HOME/go"
            export PATH="$GOPATH/bin:$PATH"
            
            # Install Go protobuf plugins if not already installed
            if ! command -v protoc-gen-go &> /dev/null; then
              echo "Installing protoc-gen-go..."
              go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
            fi
            
            if ! command -v protoc-gen-go-grpc &> /dev/null; then
              echo "Installing protoc-gen-go-grpc..."
              go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
            fi
          '';
        };
        
        # Alternative minimal shell just for protobuf generation
        devShells.proto = pkgs.mkShell {
          buildInputs = with pkgs; [
            protobuf
            go
          ];
          
          shellHook = ''
            echo "Minimal shell for protobuf generation"
            echo "Installing Go protobuf plugins..."
            go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
            go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
            echo "Ready to generate protobuf files!"
          '';
        };
      });
}