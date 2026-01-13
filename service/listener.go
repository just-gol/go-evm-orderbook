package service

import (
	"context"
	"encoding/json"
	"errors"
	orderbook "go-evm-orderbook/gen"
	"go-evm-orderbook/logger"
	"go-evm-orderbook/models"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type ListenerService interface {
	ReplayFromLast(ctx context.Context, contractAddress common.Address, starkBlock uint64, confirmations uint64) error
	StartReplayLoop(ctx context.Context, contractAddress common.Address, starkBlock uint64, confirmations uint64, interval time.Duration)
}

type listenerService struct {
	client *ethclient.Client
}

func NewListenerService(client *ethclient.Client) ListenerService {
	return &listenerService{client: client}
}

func (l *listenerService) ReplayFromLast(ctx context.Context, contractAddress common.Address, starkBlock uint64, confirmations uint64) error {
	key := syncKey("seaport", contractAddress)
	// 获取已经同步的区块高度
	last, err := l.getSyncBlock(key)
	if err != nil {
		return err
	}
	if last == 0 && starkBlock > 0 {
		last = starkBlock - 1
	}
	lasterHeader, err := l.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	laster := lasterHeader.Number.Uint64()
	if confirmations > 1 && laster >= confirmations-1 {
		laster = laster - (confirmations - 1)
	}
	if last > laster {
		return nil
	}
	return nil
}

func (l *listenerService) StartReplayLoop(ctx context.Context, contractAddress common.Address, starkBlock uint64, confirmations uint64, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	select {
	case <-ticker.C:
		err := l.ReplayFromLast(ctx, contractAddress, starkBlock, confirmations)
		if err != nil {
			logger.WithModule("listener").WithError(err).Error("start replay loop failed")
		}
	case <-ctx.Done():
		return
	}
}

/*
*
CounterIncremented
OrderCancelled
OrderFulfilled
OrderValidated
OrdersMatched
*/
func (l *listenerService) replayRange(ctx context.Context, contractAddress common.Address, start uint64, end uint64) error {
	client := l.client
	ob, err := orderbook.NewOrderbook(contractAddress, client)
	if err != nil {
		return err
	}
	endCopy := end
	err = l.consumerCounterIncremented(ob, ctx, start, &endCopy)
	if err != nil {
		return err
	}
	return l.setSyncBlock(syncKey("seaport", contractAddress), endCopy)
}

func (l *listenerService) consumerCounterIncremented(ob *orderbook.Orderbook, ctx context.Context, start uint64, end *uint64) error {
	iter, err := ob.FilterCounterIncremented(&bind.FilterOpts{Start: start, End: end, Context: ctx}, nil)
	if err != nil {
		return err
	}
	defer iter.Close()
	for iter.Next() {
		event := iter.Event
		l.handleCounterIncremented(event)
	}
	return nil
}

func (l *listenerService) handleCounterIncremented(event *orderbook.OrderbookCounterIncremented) {
	if event == nil {
		return
	}
	signature := ""
	if len(event.Raw.Topics) > 0 {
		signature = event.Raw.Topics[0].Hex()
	}
	indexedMap := map[string]string{
		"signature":  signature,
		"newCounter": event.NewCounter.String(),
		"offerer":    event.Offerer.String(),
	}
	ok, err := l.recordEventMap(event.Raw, "CounterIncremented", indexedMap)
	if err != nil || !ok {
		return
	}
	_ = l.setSyncBlock(syncKey("staking", event.Raw.Address), event.Raw.BlockNumber)
}

func (l *listenerService) recordEventMap(logEntry types.Log, eventName string, indexedMap map[string]string) (bool, error) {
	signature := ""
	if len(logEntry.Topics) > 0 {
		signature = logEntry.Topics[0].Hex()
	}
	indexedMap["signature"] = signature
	marshal, err := json.Marshal(indexedMap)
	if err != nil {
		return false, err
	}
	entry := models.EventLog{
		TxHash:      logEntry.TxHash.Hex(),
		LogIndex:    logEntry.Index,
		BlockNumber: logEntry.BlockNumber,
		Event:       eventName,
		EventArgs:   string(marshal),
		Contract:    logEntry.Address.Hex(),
	}
	result := models.DB.Where("tx_hash=? and log_index=?", entry.TxHash, entry.LogIndex).FirstOrCreate(&entry)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (l *listenerService) getSyncBlock(key string) (uint64, error) {
	var state models.SyncState
	err := models.DB.Where("name=?", key).First(&state).Error
	if err == nil {
		return state.BlockNumber, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return 0, err
}
func (l *listenerService) setSyncBlock(key string, block uint64) error {
	state := models.SyncState{Name: key, BlockNumber: block}
	return models.DB.Where("name=?", key).Assign(models.SyncState{BlockNumber: block}).FirstOrCreate(&state).Error
}
func syncKey(prefix string, contractAddress common.Address) string {
	return prefix + strings.ToLower(contractAddress.Hex())
}
