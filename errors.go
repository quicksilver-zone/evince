package main

import "errors"

var (
	ErrRPCClientConnection = errors.New("unable to connect to RPC client")
	ErrABCIQuery           = errors.New("unable to execute ABCI query")
	ErrUnmarshalResponse   = errors.New("unable to unmarshal ABCI query response")
	ErrMarshalResponse     = errors.New("unable to marshal JSON response")
	ErrReadConfigFile      = errors.New("unable to read config file")
	ErrParseConfigFile     = errors.New("unable to parse config file")
	ErrEchoFatal           = errors.New("shutting down server")
)
