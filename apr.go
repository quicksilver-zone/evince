package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/dgraph-io/ristretto"
)

type Chain struct {
	ChainID string `json:"chain_id"`
	Params  Params `json:"params"`
}

type Params struct {
	EstimatedApr float64 `json:"estimated_apr"`
}

type APRResponse struct {
	Chains []ChainAPR `json:"chains"`
}

type ChainAPR struct {
	ChainID string  `json:"chain_id"`
	APR     float64 `json:"apr"`
}

func getAPRquery(cache *ristretto.Cache, cfg Config, chainName string) (ChainAPR, error) {
	var apr float64
	var chainID string
	var err error

	client := &http.Client{Timeout: time.Duration(3) * time.Second}

	switch chainName {
	case "sommelier":
		chainID, apr, err = SommelierApr(client, cfg, chainName)
	case "stargaze":
		chainID, apr, err = StargazeApr(client, cfg, chainName)
	default:
		chainID, apr, err = BasicApr(cache, client, cfg, chainName)
	}
	if err != nil {
		return ChainAPR{}, err
	}

	if chainName != "quicksilver" {
		feeadjustedAPR := (apr) * (0.965)
		compoundedAPR := math.Pow(1+feeadjustedAPR/121.66, 121.66) - 1
		return ChainAPR{ChainID: chainID, APR: compoundedAPR}, nil
	}
	return ChainAPR{ChainID: chainID, APR: apr}, nil
}

func BasicApr(cache *ristretto.Cache, client *http.Client, cfg Config, chainName string) (string, float64, error) {
	url := fmt.Sprintf("%s/%s", cfg.APRURL, chainName)
	var result map[string]json.RawMessage

	cachedResult, ok := cache.Get("aprbasic")

	if !ok {
		resp, err := client.Get(url)
		if err != nil {
			return "", 0, err
		}

		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return "", 0, err
		}
		cache.SetWithTTL("aprbasic", result, 1, time.Duration(3)*time.Hour)
	} else {
		result = cachedResult.(map[string]json.RawMessage)
	}
	var chain Chain
	err := json.Unmarshal(result["chain"], &chain)
	if err != nil {
		return "", 0, err
	}

	return chain.ChainID, chain.Params.EstimatedApr, nil
}

func StargazeApr(client *http.Client, cfg Config, chainname string) (string, float64, error) {
	provisionsUrl := "https://stargaze-1.lcd.quicksilver.zone/stargaze/mint/v1beta1/annual_provisions"
	bondedUrl := "https://stargaze-1.lcd.quicksilver.zone/cosmos/staking/v1beta1/pool"

	provisionQuery, err := client.Get(provisionsUrl)
	if err != nil {
		return "", 0, err
	}

	defer provisionQuery.Body.Close()

	var provisionsResult map[string]sdkmath.LegacyDec
	provisionsResponse, err := io.ReadAll(provisionQuery.Body)
	if err != nil {
		fmt.Println("Error:", err)
		return "", 0, err
	}
	err = json.Unmarshal(provisionsResponse, &provisionsResult)
	if err != nil {
		return "", 0, err
	}

	provisions, err := provisionsResult["annual_provisions"].Float64()
	if err != nil {
		return "", 0, err
	}

	bondedQuery, err := client.Get(bondedUrl)
	if err != nil {
		return "", 0, err
	}

	defer bondedQuery.Body.Close()

	var bondedResult map[string]map[string]sdkmath.Int
	bondedResponse, err := io.ReadAll(bondedQuery.Body)
	if err != nil {
		fmt.Println("Error:", err)
		return "", 0, err
	}
	err = json.Unmarshal(bondedResponse, &bondedResult)
	if err != nil {
		return "", 0, err
	}

	bonded := float64(bondedResult["pool"]["bonded_tokens"].Int64())

	return "stargaze-1", provisions / bonded, nil
}

func SommelierApr(client *http.Client, cfg Config, chainname string) (string, float64, error) {
	url := "https://sommelier-3.lcd.quicksilver.zone/sommelier/incentives/v1/apy"
	query, err := client.Get(url)
	if err != nil {
		return "", 0, err
	}
	var result map[string]sdkmath.LegacyDec
	response, err := io.ReadAll(query.Body)
	if err != nil {
		fmt.Println("Error:", err)
		return "", 0, err
	}
	err = json.Unmarshal(response, &result)
	if err != nil {
		return "", 0, err
	}

	apy, err := result["apy"].Float64()
	if err != nil {
		return "", 0, err
	}
	return "sommelier-3", apy, nil
}
