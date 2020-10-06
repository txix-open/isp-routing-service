package domain

type ProxyResponse struct {
	requestBody  []byte
	responseBody []byte
	error        error
}

func Create() ProxyResponse {
	return ProxyResponse{requestBody: nil, responseBody: nil, error: nil}
}

func (p ProxyResponse) SetRequestBody(requestBody []byte) ProxyResponse {
	p.requestBody = requestBody
	return p
}

func (p ProxyResponse) SetResponseBody(responseBody []byte) ProxyResponse {
	p.responseBody = responseBody
	return p
}

func (p ProxyResponse) SetError(err error) ProxyResponse {
	p.error = err
	return p
}

func (p ProxyResponse) Get() (requestBody, responseBody []byte, err error) {
	return p.requestBody, p.responseBody, p.error
}
