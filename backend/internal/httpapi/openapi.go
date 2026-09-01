package httpapi

import (
	"net/http"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/magedmg/RSP-website/backend/internal/generated"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

const maxRequestBodyBytes int64 = 1 << 20

// validateOpenAPI checks the HTTP request against api/openapi.yaml before the
// route handler runs. Authentication is intentionally left to protected, which
// validates the bearer token and loads the application's actor.
func (a *API) validateOpenAPI(next http.Handler) http.Handler {
	spec, err := generated.GetSwagger()
	if err != nil {
		panic(err)
	}

	validator := nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		Options:               openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		SilenceServersWarning: true,
		ErrorHandler: func(w http.ResponseWriter, message string, status int) {
			writeErrorResponse(w, status, "openapi_validation_failed", message)
		},
	})(next)

	return validator
}

// limitRequestBody prevents the OpenAPI validator from reading an unbounded
// request body. When Content-Length is known, it can also return the API's
// specific oversized-body error before validation starts.
func (a *API) limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		if r.ContentLength > maxRequestBodyBytes {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_request_body", "The request body is too large or unreadable.")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}
