package response

type ErrorResponse struct {
	StatusCode int      `json:"statusCode"`
	Error      string   `json:"error"`
	Code       string   `json:"code,omitempty"`
	Message    []string `json:"messages"`
}

const (
	ErrorBadRequest          = "bad-request"
	ErrorUnauthorized        = "unauthorized"
	ErrorForbidden           = "forbidden"
	ErrorNotFound            = "not-found"
	ErrorNotAcceptable       = "not-acceptable"
	ErrorConflict            = "conflict"
	ErrorRequestTimeout      = "request-timeout"
	ErrorGone                = "gone"
	ErrorPayloadTooLarge     = "payload-too-large"
	ErrorUnsupportedMedia    = "unsupported-media-type"
	ErrorUnprocessableEntity = "unprocessable-entity"
	ErrorTooManyRequests     = "too-many-requests"
	ErrorInternalServer      = "internal-server-error"
	ErrorServiceUnavailable  = "service-unavailable"
)
