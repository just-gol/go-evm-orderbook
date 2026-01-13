package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	orderbook "go-evm-orderbook/gen"
	"go-evm-orderbook/logger"
	"go-evm-orderbook/models"
	"strconv"
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

var contractName = "seaport"

type listenerService struct {
	client *ethclient.Client
}

func NewListenerService(client *ethclient.Client) ListenerService {
	return &listenerService{client: client}
}

func (l *listenerService) ReplayFromLast(ctx context.Context, contractAddress common.Address, starkBlock uint64, confirmations uint64) error {
	key := syncKey(contractName, contractAddress)
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
	if err = l.consumerCounterIncremented(ob, ctx, start, &endCopy); err != nil {
		return err
	}
	if err = l.consumerOrderCancelled(ob, ctx, start, &endCopy); err != nil {
		return err
	}
	if err = l.consumerOrderFulfilled(ob, ctx, start, &endCopy); err != nil {
		return err
	}
	if err = l.consumerOrderValidated(ob, ctx, start, &endCopy); err != nil {
		return err
	}
	if err = l.consumerOrdersMatched(ob, ctx, start, &endCopy); err != nil {
		return err
	}

	return l.setSyncBlock(syncKey(contractName, contractAddress), endCopy)
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
func (l *listenerService) consumerOrderCancelled(ob *orderbook.Orderbook, ctx context.Context, start uint64, end *uint64) error {
	iter, err := ob.FilterOrderCancelled(&bind.FilterOpts{Start: start, End: end, Context: ctx}, nil, nil)
	if err != nil {
		return err
	}
	for iter.Next() {
		event := iter.Event
		l.handleOrderCancelled(event)
	}
	return nil
}

func (l *listenerService) consumerOrderFulfilled(ob *orderbook.Orderbook, ctx context.Context, start uint64, end *uint64) error {
	iter, err := ob.FilterOrderFulfilled(&bind.FilterOpts{Start: start, End: end, Context: ctx}, nil, nil)
	if err != nil {
		return err
	}
	defer iter.Close()
	for iter.Next() {
		event := iter.Event
		l.handleOrderFulfilled(event)
	}
	return nil
}

func (l *listenerService) consumerOrderValidated(ob *orderbook.Orderbook, ctx context.Context, start uint64, end *uint64) error {
	iter, err := ob.FilterOrderValidated(&bind.FilterOpts{Start: start, End: end, Context: ctx})
	if err != nil {
		return err
	}
	defer iter.Close()
	for iter.Next() {
		event := iter.Event
		l.handleOrderValidated(event)
	}
	return nil
}

func (l *listenerService) consumerOrdersMatched(ob *orderbook.Orderbook, ctx context.Context, start uint64, end *uint64) error {
	iter, err := ob.FilterOrdersMatched(&bind.FilterOpts{Start: start, End: end, Context: ctx})
	if err != nil {
		return err
	}
	defer iter.Close()
	for iter.Next() {
		event := iter.Event
		l.handleOrdersMatched(event)
	}
	return nil
}

func (l *listenerService) handleOrderCancelled(event *orderbook.OrderbookOrderCancelled) {
	if event == nil {
		return
	}
	signature := ""
	if len(event.Raw.Topics) > 0 {
		signature = event.Raw.Topics[0].Hex()
	}
	/**
	OrderHash [32]byte
	Offerer   common.Address
	Zone      common.Address
	*/
	var indexedMap = map[string]string{
		"signature": signature,
		"orderHash": "0x" + hex.EncodeToString(event.OrderHash[:]),
		"offerer":   event.Offerer.Hex(),
		"zone":      event.Zone.String(),
	}
	ok, err := l.recordEventMap(event.Raw, "OrderCancelled", indexedMap)
	if err != nil || !ok {
		return
	}
	_ = l.setSyncBlock(syncKey(contractName, event.Raw.Address), event.Raw.BlockNumber)
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

func (l *listenerService) handleOrderFulfilled(event *orderbook.OrderbookOrderFulfilled) {
	if event == nil {
		return
	}
	indexedMap := map[string]string{
		"orderHash":          "0x" + hex.EncodeToString(event.OrderHash[:]),
		"offerer":            event.Offerer.Hex(),
		"zone":               event.Zone.Hex(),
		"recipient":          event.Recipient.Hex(),
		"offerItemCount":     intToString(len(event.Offer)),
		"considerationCount": intToString(len(event.Consideration)),
	}
	ok, err := l.recordEventMap(event.Raw, "OrderFulfilled", indexedMap)
	if err != nil || !ok {
		return
	}
	_ = l.setSyncBlock(syncKey(contractName, event.Raw.Address), event.Raw.BlockNumber)
}

func (l *listenerService) handleOrderValidated(event *orderbook.OrderbookOrderValidated) {
	if event == nil {
		return
	}
	params := event.OrderParameters
	indexedMap := map[string]string{
		"orderHash":     "0x" + hex.EncodeToString(event.OrderHash[:]),
		"offerer":       params.Offerer.Hex(),
		"zone":          params.Zone.Hex(),
		"orderType":     intToString(int(params.OrderType)),
		"startTime":     params.StartTime.String(),
		"endTime":       params.EndTime.String(),
		"salt":          params.Salt.String(),
		"conduitKey":    "0x" + hex.EncodeToString(params.ConduitKey[:]),
		"offerCount":    intToString(len(params.Offer)),
		"considerCount": intToString(len(params.Consideration)),
	}
	ok, err := l.recordEventMap(event.Raw, "OrderValidated", indexedMap)
	if err != nil || !ok {
		return
	}
	_ = l.setSyncBlock(syncKey(contractName, event.Raw.Address), event.Raw.BlockNumber)
}

func (l *listenerService) handleOrdersMatched(event *orderbook.OrderbookOrdersMatched) {
	if event == nil {
		return
	}
	hashes := make([]string, 0, len(event.OrderHashes))
	for _, h := range event.OrderHashes {
		hashes = append(hashes, "0x"+hex.EncodeToString(h[:]))
	}
	indexedMap := map[string]string{
		"orderHashes": strings.Join(hashes, ","),
	}
	ok, err := l.recordEventMap(event.Raw, "OrdersMatched", indexedMap)
	if err != nil || !ok {
		return
	}
	_ = l.setSyncBlock(syncKey(contractName, event.Raw.Address), event.Raw.BlockNumber)
}

func intToString(value int) string {
	return strconv.Itoa(value)
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
