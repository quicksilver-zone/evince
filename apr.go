package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
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

func getAPRquery(ctx context.Context, baseurl, chainName string) (ChainAPR, error) {
	url := baseurl + chainName

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return ChainAPR{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return ChainAPR{}, err
	}

	defer resp.Body.Close()

	var result map[string]json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return ChainAPR{}, err
	}

	var chain Chain
	err = json.Unmarshal(result["chain"], &chain)
	if err != nil {
		return ChainAPR{}, err
	}

	if chainName != "quicksilver" {
		feeAdjustedAPR := (chain.Params.EstimatedApr) * (0.965)
		compoundedAPR := math.Pow(1+feeAdjustedAPR/121.66, 121.66) - 1
		return ChainAPR{ChainID: chain.ChainID, APR: compoundedAPR}, nil
	}
	return ChainAPR{ChainID: chain.ChainID, APR: chain.Params.EstimatedApr}, nil
}
