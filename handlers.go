package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	icstypes "github.com/ingenuity-build/quicksilver/x/interchainstaking/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

func (s *Service) ConfigureRoutes() {
	s.Echo.GET("/", func(ctx echo.Context) error {
		output := fmt.Sprintf("Quicksilver (evince): %v\n%v", GitCommit, LogoStr)
		return ctx.String(http.StatusOK, output)
	})

	s.Echo.GET("/validatorList/:chainId", func(ctx echo.Context) error {
		chainId := ctx.Param("chainId")

		key := fmt.Sprintf("validatorList.%s", chainId)

		data, found := s.Cache.Get(key)
		if !found {
			return s.getValidatorList(ctx, key, chainId)
		}

		return ctx.JSONBlob(http.StatusOK, data.([]byte))
	})

	s.Echo.GET("/existingDelegations/:chainId/:address", func(c echo.Context) error {
		chainId := c.Param("chainId")
		address := c.Param("address")

		key := fmt.Sprintf("existingDelegations.%s.%s", chainId, address)

		data, found := s.Cache.Get(key)
		if !found {
			return s.getExistingDelegations(c, key, chainId, address)
		}

		return c.JSONBlob(http.StatusOK, data.([]byte))
	})

	s.Echo.GET("/zones", func(ctx echo.Context) error {
		key := "zones"

		data, found := s.Cache.Get(key)
		if !found {
			return s.getZones(ctx, key)
		}

		return ctx.JSONBlob(http.StatusOK, data.([]byte))
	})
}

func (s *Service) getValidatorList(ctx echo.Context, key string, chainId string) error {
	s.Echo.Logger.Infof("getValidatorList")

	host := fmt.Sprintf(s.Config.ChainHost, chainId)

	// establish client connection
	client, err := NewRPCClient(host, 30*time.Second)
	if err != nil {
		s.Echo.Logger.Errorf("getValidatorList: %v - %v", ErrRPCClientConnection, err)
		return ErrRPCClientConnection
	}

	// prepare codecs
	interfaceRegistry := cdctypes.NewInterfaceRegistry()
	stakingtypes.RegisterInterfaces(interfaceRegistry)
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	queryResponse := stakingtypes.QueryValidatorsResponse{}
	for i := 0; queryResponse.Pagination == nil || len(queryResponse.Validators) < int(queryResponse.Pagination.Total); i++ {
		// prepare query
		vQuery := stakingtypes.QueryValidatorsRequest{
			Status: "",
		}
		if queryResponse.Pagination != nil && len(queryResponse.Pagination.NextKey) > 0 {
			vQuery.Pagination = &query.PageRequest{
				Key: queryResponse.Pagination.NextKey,
			}
		}
		qBytes := marshaler.MustMarshal(&vQuery)

		// execute query
		abciquery, err := client.ABCIQueryWithOptions(
			context.Background(),
			"/cosmos.staking.v1beta1.Query/Validators",
			qBytes,
			rpcclient.ABCIQueryOptions{Height: 0},
		)
		if err != nil {
			s.Echo.Logger.Errorf("getValidatorList: %v - %v", ErrABCIQuery, err)
			return ErrABCIQuery
		}

		// decode query response
		if err := marshaler.Unmarshal(abciquery.Response.Value, &queryResponse); err != nil {
			s.Echo.Logger.Errorf("getValidatorList: %v - %v", ErrUnmarshalResponse, err)
			return ErrUnmarshalResponse
		}
	}

	// encode response & cache
	respdata, err := codec.ProtoMarshalJSON(&queryResponse, nil)
	if err != nil {
		s.Echo.Logger.Errorf("getValidatorList: %v - %v", ErrMarshalResponse, err)
		return ErrMarshalResponse
	}

	s.Cache.SetWithTTL(key, respdata, 1, 1*time.Hour)

	return ctx.JSONBlob(http.StatusOK, respdata)
}

func (s *Service) getExistingDelegations(ctx echo.Context, key string, chainId string, address string) error {
	s.Echo.Logger.Infof("getExistingDelegations")

	host := fmt.Sprintf(s.Config.ChainHost, chainId)

	// establish client connection
	client, err := NewRPCClient(host, 30*time.Second)
	if err != nil {
		s.Echo.Logger.Errorf("getExistingDelegations: %v - %v", ErrRPCClientConnection, err)
		return ErrRPCClientConnection
	}

	// prepare codecs
	interfaceRegistry := cdctypes.NewInterfaceRegistry()
	stakingtypes.RegisterInterfaces(interfaceRegistry)
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	// prepare query
	query := stakingtypes.QueryDelegatorDelegationsRequest{
		DelegatorAddr: address,
	}
	qBytes := marshaler.MustMarshal(&query)

	// execute query
	abciquery, err := client.ABCIQueryWithOptions(
		context.Background(),
		"/cosmos.staking.v1beta1.Query/DelegatorDelegations",
		qBytes,
		rpcclient.ABCIQueryOptions{Height: 0},
	)
	if err != nil {
		s.Echo.Logger.Errorf("getExistingDelegations: %v - %v", ErrABCIQuery, err)
		return ErrABCIQuery
	}

	// decode query response
	queryResponse := stakingtypes.QueryDelegatorDelegationsResponse{}
	if err := marshaler.Unmarshal(abciquery.Response.Value, &queryResponse); err != nil {
		s.Echo.Logger.Errorf("getExistingDelegations: %v - %v", ErrUnmarshalResponse, err)
		return ErrUnmarshalResponse
	}

	// encode response & cache
	respdata, err := marshaler.MarshalJSON(&queryResponse)
	if err != nil {
		s.Echo.Logger.Errorf("getExistingDelegations: %v - %v", ErrMarshalResponse, err)
		return ErrMarshalResponse
	}

	s.Cache.SetWithTTL(key, respdata, 1, 2*time.Minute)

	return ctx.JSONBlob(http.StatusOK, respdata)
}

func (s *Service) getZones(ctx echo.Context, key string) error {
	s.Echo.Logger.Infof("getZones")

	// establish client connection
	client, err := NewRPCClient(s.Config.QuickHost, 30*time.Second)
	if err != nil {
		s.Echo.Logger.Errorf("getZones: %v - %v", ErrRPCClientConnection, err)
		return ErrRPCClientConnection
	}

	// prepare codecs
	interfaceRegistry := cdctypes.NewInterfaceRegistry()
	icstypes.RegisterInterfaces(interfaceRegistry)
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	// prepare query
	query := icstypes.QueryZonesInfoRequest{}
	qBytes := marshaler.MustMarshal(&query)

	// execute query
	abciquery, err := client.ABCIQueryWithOptions(
		context.Background(),
		"/quicksilver.interchainstaking.v1.Query/ZoneInfos",
		qBytes,
		rpcclient.ABCIQueryOptions{Height: 0},
	)
	if err != nil {
		s.Echo.Logger.Errorf("getZones: %v - %v", ErrABCIQuery, err)
		return ErrABCIQuery
	}

	// decode query response
	queryResponse := icstypes.QueryZonesInfoResponse{}
	if err := marshaler.Unmarshal(abciquery.Response.Value, &queryResponse); err != nil {
		s.Echo.Logger.Errorf("getZones: %v - %v", ErrUnmarshalResponse, err)
		return ErrUnmarshalResponse
	}

	// encode response & cache
	respdata, err := marshaler.MarshalJSON(&queryResponse)
	if err != nil {
		s.Echo.Logger.Errorf("getZones: %v - %v", ErrMarshalResponse, err)
		return ErrMarshalResponse
	}

	s.Cache.SetWithTTL(key, respdata, 1, 1*time.Minute)

	return ctx.JSONBlob(http.StatusOK, respdata)
}
