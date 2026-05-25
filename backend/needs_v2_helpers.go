package main

import "encoding/json"

func jsonUnmarshalQuiet(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
