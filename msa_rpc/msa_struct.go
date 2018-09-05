package msa_rpc

type MsError struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

type EmptyParam struct {
}
