package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
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

type SOMMAPYResponse struct {
	APY string `json:"apy"`
}

type ChainAPR struct {
	ChainID string  `json:"chain_id"`
	APR     float64 `json:"apr"`
}

func getAPRquery(baseurl string, chainname string) (ChainAPR, error) {
	url := baseurl + chainname
	if chainname == "sommelier" {
		url = "https://lcd.sommelier-3.quicksilver.zone/sommelier/incentives/v1/apy"
	}

	resp, err := http.Get(url)
	if err != nil {
		return ChainAPR{}, err
	}

	defer resp.Body.Close()

	var result map[string]json.RawMessage
	var apr float64
	var chain Chain

	if chainname == "sommelier" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error:", err)
			return ChainAPR{}, err
		}
		var aprResp SOMMAPYResponse
		err = json.Unmarshal(body, &aprResp)
		if err != nil {
			return ChainAPR{}, err
		}
		chain.ChainID = "sommelier-3"
		apr, err = strconv.ParseFloat(aprResp.APY, 64)
		if err != nil {
			fmt.Println("Error:", err)
			return ChainAPR{}, err
		}
	} else {
		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return ChainAPR{}, err
		}
		err = json.Unmarshal(result["chain"], &chain)
		if err != nil {
			return ChainAPR{}, err
		}
		apr = chain.Params.EstimatedApr
	}

	if chainname != "quicksilver" {
		feeadjustedAPR := (apr) * (0.965)
		compoundedAPR := math.Pow(1+feeadjustedAPR/121.66, 121.66) - 1
		return ChainAPR{ChainID: chain.ChainID, APR: compoundedAPR}, nil
	}
	return ChainAPR{ChainID: chain.ChainID, APR: apr}, nil

}
