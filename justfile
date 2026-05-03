default:
    @just --list

build:
    ./scripts/build.sh

install:
    ./scripts/install.sh

test:
    cd src && go test ./...
