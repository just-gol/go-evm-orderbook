package bootstrap

import (
	"context"
	"go-evm-orderbook/logger"
	"go-evm-orderbook/service"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gopkg.in/ini.v1"
)

func NewApp() (*gin.Engine, error) {
	logger.Init()
	config, err := ini.Load("./config/orderbook.ini")
	if err != nil {
		logger.WithModule("bootstrap").WithError(err).Error("load config failed")
		return nil, err
	}
	wsClient, err := ethclient.Dial(config.Section("url").Key("ws_url").String())
	if err != nil {
		logger.WithModule("bootstrap").WithError(err).Error("dial ws failed")
		return nil, err
	}
	listenerService := service.NewListenerService(wsClient)
	contractAddress := common.HexToAddress(config.Section("eth").Key("contract_address").String())
	//rpcClient, err := ethclient.Dial(config.Section("url").Key("rpc_url").String())
	//if err != nil {
	//	logger.WithModule("bootstrap").WithError(err).Error("dial rpc failed")
	//	return nil, err
	//}

	go func() {
		// 调用区块链回放
		if err := listenerService.ReplayFromLast(
			context.Background(),
			contractAddress,
			config.Section("eth").Key("start_block").MustUint64(0),
			config.Section("eth").Key("confirmations").MustUint64(1),
		); err != nil {
			logger.WithModule("listener").WithError(err).Error("replay from last failed")
			return
		}
		listenerService.StartReplayLoop(
			context.Background(),
			contractAddress,
			config.Section("eth").Key("start_block").MustUint64(0),
			config.Section("eth").Key("confirmations").MustUint64(1),
			time.Duration(config.Section("eth").Key("interval").MustUint64(1))*time.Second,
		)
	}()
	r := gin.Default()
	r.Use(cors.Default())
	return r, nil
}
