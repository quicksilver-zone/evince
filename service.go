package main

import (
	"time"

	"github.com/dgraph-io/ristretto"
	echov4 "github.com/labstack/echo/v4"
	tmhttp "github.com/tendermint/tendermint/rpc/client/http"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

const LogoStr = `
                               .........                                        
                       ..::-----------------::..                                
                   ..::---------------------------.                             
                ..:---------:::::::::::::::-----=-==:.                          
             ..::-------:::::::::::::::::::::::---====-:                        
           ..::------:::::::::::::::::::::::::::::--=-==-:                      
         ..::-----::::::::::::::::::::::::::::::::::---===-.                    
        ..:------:::::::::::::::::::::::::::::::::::::-====-                    
       .::------:::::::::::::::::::::::::::::::::::::::--===                    
     ..::------:::::::::::::::-----:::::::::::::::::::::----                    
     .::-:----::::::::-=++**###%%%%%%#*=:::::::::::::::::--:                    
    .::-::---::::::=+******#%%%%%%#*+=+##+-::::::::::::::::                     
   ..:--:::::::::=+++****#%@%%%%*:     -%=-=++++++===-::::.                     
   .::--:::::::-=++++**#%@@@@%%#.     .*%....+***++++==:..                      
   .:--:::::::-=+++++*%@@@@@@@%%-..:-+#%%:...-***+++=-:.                        
  ..:--::::::-==++++*%@@@@@@@@%%%%%%%%%%%*:.:+**+=-:..                          
  ..:--:::::-===++++%@@@@@@@@@@%%%%%%%%%%%%#*+-::..                             
  ..:---:::::===+++*@@@@@@@@@@@@%%%%%%%#*+-::...              ....              
   .::--:::::-==+++*@@@@@@@@@@@@@%%%*+=--:...             ..::------.           
   ..:-=-:::::-=++++@@@@@@@@@@@%#*===----.              ..::----=-:-=-          
    .:--=::::::-++++*@@@@@@@@#+====-----:              ..:----=--:::-=-         
    ..:--=::::::-++++%@@@@%*====--------.              .:--=+=+-:::::-=:        
     ..:---::::::-+++*%@%+====---------:             ..:+#%*.:*::+++=-==        
      ..:--=-:::::-=+*++====----------:.           .:=#@@@%%%%#*+*++++==        
       ..:--=-::::::-====-----------::.          ..:+**@@@@%%%%###***==:        
         .::--=--::::-=----------::::.          ..:==+*#@@@@%%%%####+=-         
          ..::--=--::----------:::::.           .:--===+#%@@@%%%%#+==-.         
            ..::---=--------::::::..            .::-======+****+===-:.          
              ...::----------::::..              ..:----======---:..            
                 ...::::::::::..                  ...:::::::::::..              
                     ........                         .........
`

type Service struct {
	Config Config

	*echov4.Echo
	*ristretto.Cache
}

type Config struct {
	RpcEndpoint     string     `yaml:"rpc_endpoint" json:"rpc_endpoint"`
	LcdEndpoint     string     `yaml:"lcd_endpoint" json:"lcd_endpoint"`
	ChainHost       string     `yaml:"chain_rpc_endpoint" json:"chain_rpc_endpoint"`
	Chains          []string   `yaml:"chains" json:"chains"`
	APRURL          string     `yaml:"apr_url" json:"apr_url"`
	APRCacheTime    int        `yaml:"apr_cache_minutes" json:"apr_cache_minutes"`
	SupplyCacheTime int        `yaml:"supply_cache_minutes" json:"supply_cache_minutes"`
	DefiInfo        []DefiInfo `yaml:"defi" json:"defi"`
	DefiApis        DefiApis   `yaml:"defi_apis" json:"defi_apis"`
}

type DefiInfo struct {
	AssetPair string  `yaml:"assetPair" json:"assetPair"`
	Provider  string  `yaml:"provider" json:"provider"`
	Action    string  `yaml:"action" json:"action"`
	APY       float64 `yaml:"apy" json:"apy"`
	TVL       int     `yaml:"tvl" json:"tvl"`
	Link      string  `yaml:"link" json:"link"`
	Id        string  `yaml:"id" json:"id"`
}

type DefiApis struct {
	Ux         string `yaml:"ux" json:"ux"`
	Osmosis    string `yaml:"osmosis" json:"osmosis"`
	OsmosisApy string `yaml:"osmosis_apy" json:"osmosis_apy"`
	Shade      string `yaml:"shade" json:"shade"`
}

func NewCacheService(e *echov4.Echo, cache *ristretto.Cache, cfg Config) *Service {
	return &Service{
		Config: cfg,
		Echo:   e,
		Cache:  cache,
	}
}

func NewRPCClient(addr string, timeout time.Duration) (*tmhttp.HTTP, error) {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return nil, err
	}
	httpClient.Timeout = timeout
	rpcClient, err := tmhttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return nil, err
	}
	return rpcClient, nil
}
