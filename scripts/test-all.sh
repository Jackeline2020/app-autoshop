#!/bin/sh
# Roda os testes unitários e de integração com cobertura, como no pipeline.
# A cobertura dos repositórios vem dos testes de integração (-coverpkg).
# Precisa das variáveis DB_* apontando para um banco só de testes.
set -e
PKGS=$(go list ./... | grep -v -E '/(cmd|mocks|docs)' | paste -sd, -)
go test ./... -coverpkg="$PKGS" -coverprofile=coverage.out -covermode=atomic
