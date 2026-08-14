package generated

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 -generate types,strict-server,std-http-server,spec -package generated -o api.gen.go ../../../api/openapi.yaml
//go:generate go run ./internal/postprocess api.gen.go
//go:generate sh -c "cd ../../../ && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate -f db/sqlc.yaml"
