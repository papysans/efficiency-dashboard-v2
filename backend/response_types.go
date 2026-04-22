package main

type ErrorResponse struct {
	Error string `json:"error" example:"参数错误"`
}

type StatusResponse struct {
	Status string `json:"status" example:"ok"`
}

type StatusMessageResponse struct {
	Message string `json:"message" example:"操作成功"`
}
