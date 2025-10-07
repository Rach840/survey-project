package httpx

import (
	"encoding/json"
	"net/http"
)

func ReadBody[InitType any](r http.Request) (InitType, error) {
	var body InitType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return body, err
	}
	return body, nil
}
