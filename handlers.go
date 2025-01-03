package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"image/png"
	"io"
	"log"
	"math"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/dustin/go-humanize"
	icstypes "github.com/ingenuity-build/quicksilver/x/interchainstaking/types"
	echov4 "github.com/labstack/echo/v4"
	rpcclient "github.com/tendermint/tendermint/rpc/client"

	"github.com/disintegration/imaging"
)

func (s *Service) ConfigureRoutes() {
	s.Echo.GET("/", func(ctx echov4.Context) error {
		output := fmt.Sprintf("Quicksilver (evince): %v\n%v", GitCommit, LogoStr)
		return ctx.String(http.StatusOK, output)
	})

	s.Echo.GET("/validatorList/:chainId", func(ctx echov4.Context) error {
		chainId := ctx.Param("chainId")

		key := fmt.Sprintf("validatorList.%s", chainId)

		data, found := s.Cache.Get(key)
		if !found {
			return s.getValidatorList(ctx, key, chainId)
		}

		return ctx.JSONBlob(http.StatusOK, data.([]byte))
	})

	s.Echo.GET("/existingDelegations/:chainId/:address", func(c echov4.Context) error {
		chainId := c.Param("chainId")
		address := c.Param("address")

		key := fmt.Sprintf("existingDelegations.%s.%s", chainId, address)

		data, found := s.Cache.Get(key)
		if !found {
			return s.getExistingDelegations(c, key, chainId, address)
		}

		return c.JSONBlob(http.StatusOK, data.([]byte))
	})

	s.Echo.GET("/zones", func(ctx echov4.Context) error {
		key := "zones"

		data, found := s.Cache.Get(key)
		if !found {
			return s.getZones(ctx, key)
		}

		return ctx.JSONBlob(http.StatusOK, data.([]byte))
	})

	s.Echo.GET("/apr", func(ctx echov4.Context) error {
		key := "apr"

		data, found := s.Cache.Get(key)
		if !found {
			return s.getAPR(ctx, key)
		}
		return ctx.JSONBlob(http.StatusOK, data.([]byte))
	})

	s.Echo.GET("/total_supply", func(ctx echov4.Context) error {
		key := "total_supply"

		data, found := s.Cache.Get(key)
		if !found {
			return s.getSupply(ctx, key)
		}
		return ctx.JSONBlob(http.StatusOK, data.([]byte))
	})

	s.Echo.GET("/circulating_supply", func(ctx echov4.Context) error {
		key := "circulating_supply"

		data, found := s.Cache.Get(key)
		if !found {
			return s.getCirculatingSupply(ctx, key)
		}
		return ctx.JSONBlob(http.StatusOK, data.([]byte))
	})

	s.Echo.GET("/top100/json", func(ctx echov4.Context) error {
		return s.getTopAccounts(ctx, false)
	})

	s.Echo.GET("/top100", func(ctx echov4.Context) error {
		return s.getTopAccounts(ctx, true)
	})

	s.Echo.GET("/prices", func(ctx echov4.Context) error {
		return s.getPrices(ctx)
	})

	s.Echo.GET("/defi", func(ctx echov4.Context) error {

		defi, err := s.doDefi(ctx)
		if err != nil {
			return err
		}
		data, err := json.Marshal(defi)
		if err != nil {
			return err
		}
		return ctx.JSONBlob(http.StatusOK, data)
	})

	s.Echo.GET("/valoper/:chainId/:address/:h/:w", func(ctx echov4.Context) error {
		address := ctx.Param("address")
		chainId := ctx.Param("chainId")
		height, err := strconv.Atoi(ctx.Param("h"))
		if err != nil {
			return echov4.ErrBadRequest
		}
		width, err := strconv.Atoi(ctx.Param("w"))
		if err != nil {
			return echov4.ErrBadRequest
		}

		key := fmt.Sprintf("logo.%s.%s.%d.%d", chainId, address, height, width)
		data, found := s.Cache.Get(key)
		if !found {
			data, err = s.getLogo(ctx, key, chainId, address, height, width)
			if err != nil {
				return echov4.ErrServiceUnavailable
			}
		}

		ctx.Response().Header().Add("Cache-Control", "public, max-age=86400, ") // expiry 24h
		return ctx.Blob(http.StatusOK, "image/png", data.([]byte))
	})

	s.Echo.GET("/valoper/:chainId/:address", func(ctx echov4.Context) error {
		address := ctx.Param("address")
		chainId := ctx.Param("chainId")

		key := fmt.Sprintf("logo.%s.%s.%d.%d", chainId, address, 200, 200)
		var err error
		data, found := s.Cache.Get(key)
		if !found {
			data, err = s.getLogo(ctx, key, chainId, address, 200, 200)
			if err != nil {
				return echov4.ErrServiceUnavailable
			}
		}

		ctx.Response().Header().Add("Cache-Control", "public, max-age=86400, ") // expiry 24h
		return ctx.Blob(http.StatusOK, "image/png", data.([]byte))
	})
}

func (s *Service) getValidatorList(ctx echov4.Context, key string, chainId string) error {
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

func (s *Service) getExistingDelegations(ctx echov4.Context, key string, chainId string, address string) error {
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

func (s *Service) getZones(ctx echov4.Context, key string) error {
	s.Echo.Logger.Infof("getZones")

	// establish client connection
	client, err := NewRPCClient(s.Config.RpcEndpoint, 30*time.Second)
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

func (s *Service) getAPR(ctx echov4.Context, key string) error {
	s.Echo.Logger.Infof("getAPR")

	chains := s.Config.Chains
	aprResp := APRResponse{}

	// WaitGroup to synchronize goroutines
	var wg sync.WaitGroup
	wg.Add(len(s.Config.Chains))

	for _, chain := range chains {
		go func(chainname string) {
			defer wg.Done()
			chainAPR, err := getAPRquery(s.Cache, s.Config, chainname)
			if err != nil {
				s.Echo.Logger.Errorf("unable to retrieve apy for %s: %s", chainname, err.Error())
			}
			aprResp.Chains = append(aprResp.Chains, chainAPR)
		}(chain)
	}
	// Wait for goroutines to complete
	wg.Wait()

	respdata, err := json.Marshal(aprResp)
	if err != nil {
		s.Echo.Logger.Errorf("getAPR: %v - %v", ErrMarshalResponse, err)
		return ErrMarshalResponse
	}

	s.Cache.SetWithTTL(key, respdata, 1, time.Duration(s.Config.APRCacheTime)*time.Minute)

	return ctx.JSONBlob(http.StatusOK, respdata)
}

func (s *Service) getSupply(ctx echov4.Context, key string) error {
	s.Echo.Logger.Infof("getSupply")

	supply, _, err := getSupply(s.Config.SupplyLcdEndpoint + "/quicksilver/supply/v1/supply")
	if err != nil {
		s.Echo.Logger.Errorf("getCirculatingSupply: %v - %v", ErrUnableToGetTotalSupply, err)
		return ErrUnableToGetTotalSupply
	}

	respData, err := json.Marshal(supply.Quo(sdkmath.NewInt(1_000_000)).Int64())
	if err != nil {
		s.Echo.Logger.Errorf("getCirculatingSupply: %v - %v", ErrMarshalResponse, err)
		return ErrMarshalResponse
	}
	s.Cache.SetWithTTL(key, respData, 1, time.Duration(s.Config.SupplyCacheTime)*time.Minute)

	return ctx.JSONBlob(http.StatusOK, respData)
}
func (s *Service) getCirculatingSupply(ctx echov4.Context, key string) error {
	s.Echo.Logger.Infof("getCirculatingSupply")

	_, circulatingSupply, err := getSupply(s.Config.SupplyLcdEndpoint + "/quicksilver/supply/v1/supply")
	if err != nil {
		s.Echo.Logger.Errorf("getCirculatingSupply: %v - %v", ErrUnableToGetTotalSupply, err)
		return ErrUnableToGetTotalSupply
	}

	respData, err := json.Marshal(circulatingSupply.Quo(sdkmath.NewInt(1_000_000)).Int64())
	if err != nil {
		s.Echo.Logger.Errorf("getCirculatingSupply: %v - %v", ErrMarshalResponse, err)
		return ErrMarshalResponse
	}
	s.Cache.SetWithTTL(key, respData, 1, time.Duration(s.Config.SupplyCacheTime)*time.Minute)

	return ctx.JSONBlob(http.StatusOK, respData)
}

func (s *Service) getPrices(ctx echov4.Context) error {
	s.Echo.Logger.Infof("getPrices")
	key := "prices"
	data, found := s.Cache.Get(key)
	if found {
		return ctx.JSONBlob(http.StatusOK, data.([]byte))
	} else {
		// Create a new GET request
		req, err := http.NewRequest("GET", "https://pro-api.coinmarketcap.com/v1/cryptocurrency/quotes/latest?slug="+strings.Join(s.Config.CMCSlugs, ","), nil)
		if err != nil {
			log.Fatalf("Failed to create request: %v", err)
		}

		// Set headers
		req.Header.Add("X-CMC_PRO_API_KEY", os.Getenv("CMC_KEY"))

		// Use a client to send the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// Read all the response body
		result, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("failed to read response body: %v", err)
		}

		var cmcResponse CMCResponse
		err = json.Unmarshal(result, &cmcResponse)
		if err != nil {
			log.Fatalf("failed to unmarshal response: %v", err)
		}

		priceOutput := PriceOutput{}
		for _, data := range cmcResponse.Data {
			priceOutput[data.Symbol] = data.Quote["USD"].Price
		}

		respData, err := json.Marshal(priceOutput)
		if err != nil {
			log.Fatalf("failed to marshal response: %v", err)
		}

		s.Cache.SetWithTTL(key, respData, 1, 5*time.Minute)

		return ctx.JSONBlob(http.StatusOK, respData)
	}

}

type PriceOutput map[string]float64

type CMCStatus struct {
	ErrorCode    int    `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Elapsed      int    `json:"elapsed"`
	CreditCount  int    `json:"credit_count"`
}

type CMCResponse struct {
	Status CMCStatus          `json:"status"`
	Data   map[string]CMCData `json:"data"`
}

type CMCData struct {
	Symbol string              `json:"symbol"`
	Quote  map[string]CMCQuote `json:"quote"`
}

type CMCQuote struct {
	Price float64 `json:"price"`
}

func (s *Service) getTopAccounts(ctx echov4.Context, pretty bool) error {
	key := "top100"
	var result []byte
	data, found := s.Cache.Get(key)
	if found {
		result = data.([]byte)
	} else {
		resp, err := http.Get(s.Config.SupplyLcdEndpoint + "/quicksilver/supply/v1/topn/100")
		if err != nil {
			s.Logger.Error("unable to get top accounts", err)
			return err
		}

		defer resp.Body.Close()

		// Read all the response body
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("failed to read response body: %v", err)
		}

		s.Logger.Info("set cache for top accounts")
		s.Cache.SetWithTTL(key, result, 1, 1*time.Hour)
	}

	if !pretty {
		return ctx.Blob(http.StatusOK, "application/json", result)
	}

	var accountResponse AccountResponse
	err := json.Unmarshal(result, &accountResponse)
	if err != nil {
		return err
	}

	htmlContent, err := generateHTML(accountResponse.Accounts)
	if err != nil {
		return err
	}
	return ctx.HTML(http.StatusOK, htmlContent)
}

type AccountResponse struct {
	Accounts []TopAccount `json:"accounts"`
}

type TopAccount struct {
	Address string      `json:"address"`
	Balance sdkmath.Int `json:"balance"`
}

type Supply struct {
	Supply            sdkmath.Int `json:"supply"`
	CirculatingSupply sdkmath.Int `json:"circulating_supply"`
}

func getSupply(url string) (sdkmath.Int, sdkmath.Int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return sdkmath.Int{}, sdkmath.Int{}, err
	}

	var result json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return sdkmath.Int{}, sdkmath.Int{}, err
	}

	var supply Supply
	err = json.Unmarshal(result, &supply)
	if err != nil {
		return sdkmath.Int{}, sdkmath.Int{}, err
	}

	return supply.Supply, supply.CirculatingSupply, nil

}

func (s *Service) getLogo(ctx echov4.Context, key string, chain string, address string, height int, width int) ([]byte, error) {
	resp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/cosmostation/chainlist/main/chain/%s/moniker/%s.png", chain, address))
	if err != nil {
		// fetch from keybase
		return s.placeHolder(key, height, width), nil
	}

	defer resp.Body.Close()
	img, err := imaging.Decode(resp.Body)

	if err != nil {
		return s.placeHolder(key, height, width), nil
	}

	img = imaging.Resize(img, height, width, imaging.Lanczos)

	out := bytes.NewBuffer([]byte{})
	imaging.Encode(out, img, imaging.PNG, imaging.PNGCompressionLevel(png.BestCompression))

	ctx.Logger().Error(fmt.Sprintf("read %d bytes", len(out.Bytes())))

	s.Cache.SetWithTTL(key, out.Bytes(), 1, 12*time.Hour)

	return out.Bytes(), nil
}

func (s *Service) placeHolder(key string, height int, width int) []byte {
	img, _ := imaging.Open("placeholder.png")
	img = imaging.Resize(img, height, width, imaging.Lanczos)
	result := bytes.NewBuffer([]byte{})
	imaging.Encode(result, img, imaging.PNG, imaging.PNGCompressionLevel(png.BestCompression))
	s.Cache.SetWithTTL(key, result.Bytes(), 1, 1*time.Hour)
	return result.Bytes()
}

func (s *Service) doDefi(ctx echov4.Context) ([]DefiInfo, error) {
	out := []DefiInfo{}
	for _, d := range s.Config.DefiInfo {
		switch d.Provider {
		case "ux":
			r, err := s.doDefiUx(d)
			if err != nil {
				ctx.Logger().Error("unable to fetch ux defi", err)
				out = append(out, d)
			}
			out = append(out, r)
		case "osmosis":
			r, err := s.doDefiOsmosis(d)
			if err != nil {
				ctx.Logger().Error("unable to fetch osmosis defi", err)
				out = append(out, d)
			}
			out = append(out, r)
		case "shade":
			r, err := s.doDefiShade(d)
			if err != nil {
				ctx.Logger().Error("unable to fetch shade defi", err)
				out = append(out, d)
			}
			out = append(out, r)
		default:
			// if config doesn't exist, just use the static content
			out = append(out, d)
		}
	}
	return out, nil
}

type UxResult struct {
	Asset string  `json:"asset"`
	Tvl   float64 `json:"collateral_usd"`
	Apy   float64 `json:"supply_apy"`
}

func (s *Service) doDefiUx(d DefiInfo) (DefiInfo, error) {
	key := fmt.Sprintf("defi.%s.%s", d.Provider, d.Id)
	cached, found := s.Cache.Get(key)
	if found {
		s.Logger.Info(fmt.Sprintf("hit cache for ux pool %s", d.Id))
		return cached.(DefiInfo), nil
	}
	var result []UxResult
	var err error
	cachedResult, found := s.Cache.Get("defi.raw.ux")
	if !found {
		result, err = s.queryUx()
		if err != nil {
			return d, err
		}
	} else {
		result = cachedResult.([]UxResult)
	}

	for _, r := range result {
		if r.Asset == d.Id {
			d.APY = r.Apy
			d.TVL = int(r.Tvl)
			break
		}
	}
	s.Cache.SetWithTTL(key, d, 1, 3*time.Hour)

	return d, nil
}

type OsmosisPoolCacheResult struct {
	Pools    []OsmosisPoolResult    `json:"pools"`
	PoolAprs []OsmosisPoolAprResult `json:"-"`
}
type OsmosisPoolResult struct {
	Id  string  `json:"id"`
	Tvl float64 `json:"liquidityUsd"`
}

type OsmosisPoolAprResult struct {
	Id  string  `json:"pool_id"`
	Apr float64 `json:"total_apr"`
}

func (s *Service) doDefiOsmosis(d DefiInfo) (DefiInfo, error) {
	key := fmt.Sprintf("defi.%s.%s", d.Provider, d.Id)
	cached, found := s.Cache.Get(key)
	if found {
		s.Logger.Info(fmt.Sprintf("hit cache for osmosis pool %s", d.Id))
		return cached.(DefiInfo), nil
	}

	var result OsmosisPoolCacheResult
	var err error
	cachedResult, found := s.Cache.Get("defi.raw.osmosis")
	if !found {
		result, err = s.queryOsmo()
		if err != nil {
			return d, err
		}
	} else {
		result = cachedResult.(OsmosisPoolCacheResult)
	}

	for _, pool := range result.Pools {
		if pool.Id == d.Id {
			d.TVL = int(pool.Tvl)
			break
		}
	}

	for _, pool := range result.PoolAprs {
		if pool.Id == d.Id {
			d.APY = pool.Apr / 100
			break
		}
	}

	s.Cache.SetWithTTL(key, d, 1, 3*time.Hour)

	return d, nil
}

type ShadeApr struct {
	Total float64 `json:"total"`
}
type ShadeResult struct {
	Id  string   `json:"id"`
	Apy ShadeApr `json:"apy"`
	Tvl string   `json:"liquidity_usd"`
}

func (s *Service) doDefiShade(d DefiInfo) (DefiInfo, error) {
	key := fmt.Sprintf("defi.%s.%s", d.Provider, d.Id)
	cached, found := s.Cache.Get(key)
	if found {
		s.Logger.Info(fmt.Sprintf("hit cache for shade pool %s", d.Id))
		return cached.(DefiInfo), nil
	}

	var result []ShadeResult
	var err error
	cachedResult, found := s.Cache.Get("defi.raw.shade")
	if !found {
		result, err = s.queryShade()
		if err != nil {
			return d, err
		}
	} else {
		result = cachedResult.([]ShadeResult)
	}

	for _, r := range result {
		if r.Id == d.Id {
			d.APY = r.Apy.Total / 100
			fl, _ := strconv.ParseFloat(r.Tvl, 64)
			d.TVL = int(math.Floor(fl))
			break
		}
	}
	s.Cache.SetWithTTL(key, d, 1, 3*time.Hour)
	return d, nil
}

func (s *Service) queryShade() ([]ShadeResult, error) {

	s.Logger.Info("querying shade api")
	resp, err := http.Get(s.Config.DefiApis.Shade)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []ShadeResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	s.Cache.SetWithTTL("defi.raw.shade", result, 1, 3*time.Hour)
	time.Sleep(time.Millisecond * 200)

	return result, nil
}

func (s *Service) queryUx() ([]UxResult, error) {

	s.Logger.Info("querying ux api")
	resp, err := http.Get(s.Config.DefiApis.Ux)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []UxResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	s.Cache.SetWithTTL("defi.raw.ux", result, 1, 3*time.Hour)
	time.Sleep(time.Millisecond * 200)

	return result, nil
}

func (s *Service) queryOsmo() (OsmosisPoolCacheResult, error) {

	s.Logger.Info("querying osmosis api")
	resp, err := http.Get(s.Config.DefiApis.Osmosis)
	if err != nil {
		return OsmosisPoolCacheResult{}, err
	}
	defer resp.Body.Close()

	var poolResult OsmosisPoolCacheResult
	err = json.NewDecoder(resp.Body).Decode(&poolResult)
	if err != nil {
		return OsmosisPoolCacheResult{}, err
	}
	fmt.Printf("poolResult: %v\n", poolResult)

	s.Logger.Info("querying osmosis apr api")

	resp, err = http.Get(s.Config.DefiApis.OsmosisApy)
	if err != nil {
		return OsmosisPoolCacheResult{}, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&poolResult.PoolAprs)
	if err != nil {
		return OsmosisPoolCacheResult{}, err
	}
	fmt.Printf("poolResult.PoolAprs: %v\n", poolResult.PoolAprs)

	s.Cache.SetWithTTL("defi.raw.osmosis", poolResult, 1, 3*time.Hour)
	time.Sleep(time.Millisecond * 200)

	return poolResult, nil
}

// templateHTML is a simple HTML template with inline styling
// to display the TopAccount data in a table.
const templateHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>Top 100 Accounts</title>
    <link rel="preconnect" href="https://fonts.gstatic.com" />
    <link href="https://fonts.googleapis.com/css?family=Lato:400,700&display=swap" rel="stylesheet">
    <style>
        body {
            background-color: #000;
            color: #fff;
            font-family: 'Lato', sans-serif;
            margin: 0;
            padding: 0;
        }
        header {
            text-align: center;
            margin: 1rem;
        }
        .logo {
            display: block;
            margin: 0 auto 1rem;
            width: 200px; /* Adjust as needed */
        }
        h1 {
            text-align: center;
            margin-bottom: 0.5rem;
            font-family: 'Lato', sans-serif;
        }
        p {
            text-align: center;
            max-width: 600px;
            margin: 0 auto 2rem;
            font-family: 'Lato', sans-serif;
        }
        table {
            width: 70%;
            border-collapse: collapse;
            margin: 2rem auto;
            background-color: #222;
        }
        th, td {
            padding: 12px;
            border: 1px solid #666;
            text-align: left;
            font-family: 'Lato', sans-serif;
        }
        th {
            background-color: #333;
        }
		.balance-col {
            /* Monospaced font & right alignment for numeric values */
            text-align: right;
            font-family: 'Courier New', Courier, monospace;
        }
        tr:nth-child(even) {
            background-color: #2a2a2a;
        }
        tr:nth-child(odd) {
            background-color: #222;
        }
        tr:hover {
            background-color: #444;
        }
    </style>
</head>
<body>
    <header>
        <img 
            src="https://app.quicksilver.zone/img/quicksilverWord.png" 
            alt="Quicksilver Logo" 
            class="logo" 
        />
        <h1>Top 100 Accounts</h1>
        <p>This table shows the top 100 accounts (including locked and delegated tokens) on the Quicksilver network, excluding module accounts.</p>
    </header>
    <table>
        <thead>
            <tr>
                <th>Address</th>
                <th>Balance</th>
            </tr>
        </thead>
        <tbody>
        {{- range . }}
            <tr>
                <td>{{ .Address }}</td>
                <td class="balance-col">{{ .Balance | FormatAmount }}</td>
            </tr>
        {{- end }}
        </tbody>
    </table>
</body>
</html>
`

// generateHTML takes a slice of TopAccount and generates HTML that displays a table.
func generateHTML(topAccounts []TopAccount) (string, error) {
	t, err := template.New("topaccounts").Funcs(templateFuncs).Parse(templateHTML)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, topAccounts); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

var templateFuncs = template.FuncMap{
	"FormatAmount": func(balance sdkmath.Int) string {
		// Convert to big.Rat so we can do decimal division by 1e6
		rat := new(big.Rat).SetInt(balance.BigInt())

		// Divide by 1e6
		divisor := big.NewRat(1_000_000, 1)
		rat = rat.Quo(rat, divisor)

		// Convert to float64
		floatVal, _ := rat.Float64()

		// Insert commas, show 2 decimal places (adjust as needed)
		formatted := humanize.CommafWithDigits(floatVal, 6)

		// Append "QCK"
		return fmt.Sprintf("%s QCK", formatted)
	},
}
