package main

import "errors"

var (
	ErrRPCClientConnection      = errors.New("unable to connect to RPC client")
	ErrABCIQuery                = errors.New("unable to execute ABCI query")
	ErrUnmarshalResponse        = errors.New("unable to unmarshal ABCI query response")
	ErrMarshalResponse          = errors.New("unable to marshal JSON response")
	ErrReadConfigFile           = errors.New("unable to read config file")
	ErrParseConfigFile          = errors.New("unable to parse config file")
	ErrEchoFatal                = errors.New("shutting down server")
	ErrUnableToGetAPR           = errors.New("unable to get apr response")
	ErrUnableToGetLockedTokens  = errors.New("unable to get locked tokens response")
	ErrUnableToGetTotalSupply   = errors.New("unable to get total supply response")
	ErrUnableToGetCommunityPool = errors.New("unable to get CommunityPool response")
)
