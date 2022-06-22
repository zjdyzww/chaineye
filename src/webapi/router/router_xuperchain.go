package router

import (
	"encoding/hex"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/didi/nightingale/v5/src/webapi/config"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"

	"github.com/xuperchain/xuper-sdk-go/v2/xuper"
	"github.com/xuperchain/xuperchain/service/pb"
)

// query contract count
func getXuperChainContractCount(c *gin.Context) {
	var f ContractCountReq
	ginx.BindJSON(c, &f)
	if f.ChainName == "" {
		f.ChainName = "xuper"
	}
	client, err := xuper.New(config.C.XuperChain.XuperChainNode, xuper.WithConfigFile(config.C.XuperChain.XuperSdkYmlPath))
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "new xuperchain sdk client failed")
	}
	csdr, err := client.QueryContractCount(xuper.WithQueryBcname(f.ChainName))
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "query contract count failed")
	}
	ginx.NewRender(c).Data(gin.H{
		"counte": csdr.Data.ContractCount,
	}, nil)
}

// query block or tx
func getXuperChainTx(c *gin.Context) {
	var f TxQueryReq
	ginx.BindJSON(c, &f)
	if f.ChainName == "" {
		f.ChainName = "xuper"
	}
	client, err := xuper.New(config.C.XuperChain.XuperChainNode, xuper.WithConfigFile(config.C.XuperChain.XuperSdkYmlPath))
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "new xuperchain sdk client failed")
	}
	switch f.Type {
	case ByTxId: // query by tx id
		tx, err := client.QueryTxByID(f.Input, xuper.WithQueryBcname(f.ChainName))
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "query Tx failed")
		}
		// query Block
		block, err := client.QueryBlockByID(hex.EncodeToString(tx.Blockid), xuper.WithQueryBcname(f.ChainName))
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "query Tx failed")
		}
		tdr, err := TxToTxRsp(tx, block)
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "query Tx failed")
		}
		ginx.NewRender(c).Data(gin.H{
			"transaction": tdr,
		}, nil)
	case ByBlockHeight: // query by block height
		i, err := strconv.ParseInt(f.Input, 36, 10)
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "query Tx failed")
		}
		b, err := client.QueryBlockByHeight(int64(i), xuper.WithQueryBcname(f.ChainName))
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "query Tx failed")
		}
		br, err := blockToBlockRsp(b)
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "query Tx failed")
		}
		ginx.NewRender(c).Data(gin.H{
			"transaction": br,
		}, nil)
	case ByBlockId: // query by block id
		b, err := client.QueryBlockByID(f.Input, xuper.WithQueryBcname(f.ChainName))
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "query Tx failed")
		}
		br, err := blockToBlockRsp(b)
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "query Tx failed")
		}
		ginx.NewRender(c).Data(gin.H{
			"transaction": br,
		}, nil)
	}

}

func TxToTxRsp(tx *pb.Transaction, block *pb.Block) (TxDetailRsp, error) {
	var rsp TxDetailRsp
	rsp.TxId = hex.EncodeToString(tx.Txid)
	rsp.BlockId = hex.EncodeToString(tx.Blockid)
	rsp.BlockHeight = block.Block.Height
	rsp.Coinbase = tx.Coinbase
	rsp.Miner = string(block.Block.Proposer)
	rsp.BlockTimestamp = block.Block.Timestamp / 1e9
	rsp.Initiator = tx.Initiator
	for _, v := range tx.TxInputs {
		rsp.FromAddress = append(rsp.FromAddress, string(v.FromAddr))
	}
	for _, v := range tx.TxOutputs {
		rsp.ToAddresses = append(rsp.ToAddresses, string(v.ToAddr))
	}
	var amountInput = &big.Int{}
	var amountOuput = &big.Int{}
	for _, v := range tx.TxInputs {
		amountInput = amountInput.Add(FromAmountBytes(v.Amount), amountInput)
	}
	for _, v := range tx.TxOutputs {
		amountOuput = amountOuput.Add(FromAmountBytes(v.Amount), amountOuput)
	}
	rsp.FromTotal = amountInput
	rsp.ToTotal = amountOuput
	// 计算fee
	var gasUsed int64
	for _, v := range tx.ContractRequests {
		for _, v1 := range v.ResourceLimits {
			gasUsed += v1.Limit
		}
	}
	rsp.Fee = gasUsed
	rsp.Timestamp = tx.Timestamp / 1e9
	tm := time.Unix(rsp.Timestamp, 0)
	rsp.Date = tm.Local().Format("2006-01-02 15:04:05")
	for _, v := range tx.ContractRequests {
		rsp.Contracts = append(rsp.Contracts, v.ContractName)
	}
	return rsp, nil
}

func FromAmountBytes(buf []byte) *big.Int {
	n := big.Int{}
	n.SetBytes(buf)
	return &n
}

func blockToBlockRsp(block *pb.Block) (BlockRsp, error) {
	var rsp BlockRsp
	rsp.BlockHeight = block.Block.Height
	rsp.BlockId = hex.EncodeToString(block.Block.Blockid)
	rsp.Miner = string(block.Block.Proposer)
	rsp.NextHash = hex.EncodeToString(block.Block.NextHash)
	rsp.PreHash = hex.EncodeToString(block.Block.PreHash)
	rsp.Timestamp = block.Block.Timestamp / 1e9
	rsp.TxNumber = block.Block.TxCount
	for _, v := range block.Block.Transactions {
		rsp.Txs = append(rsp.Txs, hex.EncodeToString(v.Txid))
	}
	return rsp, nil
}

type ContractCountReq struct {
	ChainName string `json:"chain_name"`
}

type TxQueryReq struct {
	ChainName string `json:"chain_name"`
	Input     string `json:"input"`
	Type      int    `json:"type"`
}

type BlockRsp struct {
	BlockHeight int64    `json:"block_height"`
	BlockId     string   `json:"block_id"`
	Miner       string   `json:"miner"`
	NextHash    string   `json:"next_hash"`
	PreHash     string   `json:"pre_hash"`
	Timestamp   int64    `json:"timestamp"`
	TxNumber    int32    `json:"tx_number"`
	Txs         []string `json:"txs"`
}

type TxDetailRsp struct {
	TxId           string   `json:"tx_id"`
	BlockId        string   `json:"block_id"`
	BlockHeight    int64    `json:"block_height"`
	Coinbase       bool     `json:"coinbase"`
	Miner          string   `json:"miner"`
	BlockTimestamp int64    `json:"block_timestamp"`
	Initiator      string   `json:"initiator"`
	FromAddress    []string `json:"from_address"`
	ToAddresses    []string `json:"to_addresses"`
	FromTotal      *big.Int `json:"from_total"`
	ToTotal        *big.Int `json:"to_total"`
	Fee            int64    `json:"fee"`
	Timestamp      int64    `json:"timestamp"`
	Date           string   `json:"date"`
	Contracts      []string `json:"contracts"`
}

const (
	ByTxId = iota + 1
	ByBlockHeight
	ByBlockId
)
