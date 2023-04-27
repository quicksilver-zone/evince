package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	sdkmath "cosmossdk.io/math"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
)

// get supply
// bondDenom uqck
/* delemap*/

type Supply struct {
	Supply sdktypes.Coins `json:"supply"`
}

type CommunityPool struct {
	Pool sdktypes.DecCoins `json:"pool"`
}

type Account struct {
	Type               string             `json:"@type"`
	BaseVestingAccount BaseVestingAccount `json:"base_vesting_account"`
	StartTime          string             `json:"start_time"`
	VestingPeriods     []VestingPeriods   `json:"vesting_periods"`
}

type BaseVestingAccount struct {
	BaseAccount      BaseAccount    `protobuf:"bytes,1,opt,name=base_account,json=baseAccount,proto3,embedded=base_account" json:"base_account,omitempty"`
	OriginalVesting  sdktypes.Coins `protobuf:"bytes,2,rep,name=original_vesting,json=originalVesting,proto3,castrepeated=github.com/cosmos/cosmos-sdk/types.Coins" json:"original_vesting" yaml:"original_vesting"`
	DelegatedFree    sdktypes.Coins `protobuf:"bytes,3,rep,name=delegated_free,json=delegatedFree,proto3,castrepeated=github.com/cosmos/cosmos-sdk/types.Coins" json:"delegated_free" yaml:"delegated_free"`
	DelegatedVesting sdktypes.Coins `protobuf:"bytes,4,rep,name=delegated_vesting,json=delegatedVesting,proto3,castrepeated=github.com/cosmos/cosmos-sdk/types.Coins" json:"delegated_vesting" yaml:"delegated_vesting"`
	EndTime          string         `protobuf:"varint,5,opt,name=end_time,json=endTime,proto3" json:"end_time,omitempty" yaml:"end_time"`
}

type BaseAccount struct {
	Address       string `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	PubKey        string `protobuf:"bytes,2,opt,name=pub_key,json=pubKey,proto3" json:"public_key,omitempty" yaml:"public_key"`
	AccountNumber string `protobuf:"varint,3,opt,name=account_number,json=accountNumber,proto3" json:"account_number,omitempty" yaml:"account_number"`
	Sequence      string `protobuf:"varint,4,opt,name=sequence,proto3" json:"sequence,omitempty"`
}

type VestingPeriods struct {
	Length string         `json:"length"`
	Amount sdktypes.Coins `json:"amount"`
}

func getVestingAccountLocked(baseurl, address string) (sdkmath.Int, error) {
	url := baseurl + address
	resp, err := http.Get(url)
	if err != nil {
		return sdkmath.Int{}, err
	}

	var result map[string]json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return sdkmath.Int{}, err
	}
	var account Account
	err = json.Unmarshal(result["account"], &account)
	if err != nil {
		return sdkmath.Int{}, err
	}
	if account.Type == "/cosmos.vesting.v1beta1.PeriodicVestingAccount" {
		lockedTokens := account.BaseVestingAccount.OriginalVesting.AmountOf("uqck")
		startTime, err := strconv.ParseInt(account.StartTime, 10, 64)
		if err != nil {
			return sdkmath.Int{}, err
		}
		for _, vestPeriod := range account.VestingPeriods {
			period, err := strconv.ParseInt(vestPeriod.Length, 10, 64)
			if err != nil {
				return sdkmath.Int{}, err
			}
			if (startTime + period) < time.Now().Unix() {
				lockedTokens = lockedTokens.Sub(vestPeriod.Amount.AmountOf("uqck"))
			}
			startTime = startTime + period

		}

		return lockedTokens, nil
	}
	return sdkmath.ZeroInt(), nil
}

func getTotalSupply(url string) (sdkmath.Int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return sdkmath.Int{}, err
	}

	var result json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return sdkmath.Int{}, err
	}

	var supply Supply
	err = json.Unmarshal(result, &supply)
	if err != nil {
		return sdkmath.Int{}, err
	}

	return supply.Supply.AmountOf("uqck"), nil
}

func getCommunityPool(url string) (sdkmath.Int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return sdkmath.Int{}, err
	}

	var result json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return sdkmath.Int{}, err
	}

	var comPool CommunityPool
	err = json.Unmarshal(result, &comPool)
	if err != nil {
		return sdkmath.Int{}, err
	}

	return comPool.Pool.AmountOf("uqck").RoundInt(), nil
}
