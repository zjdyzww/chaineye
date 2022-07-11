package router

import (
	"encoding/hex"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/didi/nightingale/v5/src/webapi/config"
	"github.com/didi/nightingale/v5/src/webapi/xuper_chain"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"

	"github.com/xuperchain/xuper-sdk-go/v2/xuper"
	"github.com/xuperchain/xuperchain/service/pb"
)

// query contract count
func getXuperChainContractCount(c *gin.Context) {
	chainName := ginx.QueryStr(c, "chain_name", "")
	if chainName == "" {
		chainName = "xuper"
	}
	client, err := xuper.New(config.C.XuperChain.XuperChainNode, xuper.WithConfigFile(config.C.XuperChain.XuperSdkYmlPath))
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "new xuperchain sdk client failed")
	}
	defer client.Close()
	// query contract counts
	csdr, err := client.QueryContractCount(xuper.WithQueryBcname(chainName))
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "query contract count failed")
	}
	// query block height
	b, err := client.QuerySystemStatus(xuper.WithQueryBcname(chainName))
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "query block height failed")
	}
	// query mint present address
	status, err := client.QueryBlockChainStatus(xuper.WithQueryBcname(chainName))
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "query mint present address failed")
	}
	// query peer counts
	var peerCounts int

	// query tx counts
	xs, err := xuper_chain.GetMaxHeightInDB(c)
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "query tx counts failed")
	}

	if len(b.SystemsStatus.PeerUrls) == 0 {
		peerCounts = 1
	} else {
		peerCounts = len(b.SystemsStatus.PeerUrls)
	}
	ginx.NewRender(c).Data(
		gin.H{
			"count":     csdr.Data.ContractCount,                   // 返回合约数量
			"height":    b.SystemsStatus.BcsStatus[0].Block.Height, // 返回当前区块高度
			"proposer":  status.Block.Proposer,                     // 返回当前打包区块的矿工地址
			"node_sum":  peerCounts,                                // 返回当前链的节点数量 如果是
			"tx_counts": xs.TotalTxCount,                           // 返回当前链的交易总数
		}, nil)
}

// query block or tx
func getXuperChainTx(c *gin.Context) {
	chainName := ginx.QueryStr(c, "chain_name", "")
	input := ginx.QueryStr(c, "input", "")
	if chainName == "" {
		chainName = "xuper"
	}
	client, err := xuper.New(config.C.XuperChain.XuperChainNode, xuper.WithConfigFile(config.C.XuperChain.XuperSdkYmlPath))
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "new xuperchain sdk client failed")
	}
	defer client.Close()
	// 正则判断查询条件 全数字按照区块高度查询 转换异常 按照交易hash 或者 区块hash查询
	i, err := strconv.ParseUint(input, 10, 64)
	// err is nil 证明传入的是数字，按照区块高度查询
	if err == nil {
		b, err := client.QueryBlockByHeight(int64(i), xuper.WithQueryBcname(chainName))
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "未查询到相关结果")
		}
		br, err := blockToBlockRsp(b)
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "未查询到相关结果")
		}
		ginx.NewRender(c).Data(gin.H{
			"block": br,
		}, nil)
	} else {
		// err is not nil。 先按照交易hash 查询
		tx, err := client.QueryTxByID(input, xuper.WithQueryBcname(chainName))
		if err == nil {
			// query Block
			block, err := client.QueryBlockByID(hex.EncodeToString(tx.Blockid), xuper.WithQueryBcname(chainName))
			if err != nil {
				ginx.Bomb(http.StatusInternalServerError, "未查询到相关结果")
			}
			tdr, err := TxToTxRsp(tx, block)
			if err != nil {
				ginx.Bomb(http.StatusInternalServerError, "未查询到相关结果")
			}
			ginx.NewRender(c).Data(gin.H{
				"transaction": tdr,
			}, nil)
		} else {
			// 按照区块 hash 查询
			b, err := client.QueryBlockByID(input, xuper.WithQueryBcname(chainName))
			if err != nil {
				ginx.Bomb(http.StatusInternalServerError, "未查询到相关结果")
			}
			br, err := blockToBlockRsp(b)
			if err != nil {
				ginx.Bomb(http.StatusInternalServerError, "未查询到相关结果")
			}
			ginx.NewRender(c).Data(gin.H{
				"block": br,
			}, nil)
		}
	}
}

// 查询十天 每天的交易总数 返回前端生成折线图
func getXuperChainTxLineChart(c *gin.Context) {
	var dataRsp []string
	var txCountsRsp []int64

	// 查询时间戳 并转成日期 取当前日期后一天
	s := time.Now().Local().Add(time.Hour * 24).Format("2006-01-02")
	ts, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		ginx.Bomb(http.StatusInternalServerError, "time parse in location failed")
	}
	newTime := ts.Unix()
	for i := 0; i < 10; i++ {
		// 计算最新的高度和前一天的最新高度。
		t := time.Unix(int64(newTime)-1, 0)
		s2 := t.Local().Format("2006-01-02")
		presentTxs, err := xuper_chain.GetTxCountsSection(c, newTime)
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "get tx countsSection failed")
		}
		newTime = newTime - 24*3600
		yestodayTxs, err := xuper_chain.GetTxCountsSection(c, newTime)
		if err != nil {
			ginx.Bomb(http.StatusInternalServerError, "get tx countsSection failed")
		}
		dataRsp = append(dataRsp, s2)
		txCountsRsp = append(txCountsRsp, presentTxs-yestodayTxs)
	}
	reverse(dataRsp)
	reverseInt(txCountsRsp)
	ginx.NewRender(c).Data(gin.H{
		"data":   dataRsp,
		"counts": txCountsRsp,
	}, nil)
}

func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
func reverseInt(s []int64) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func TxToTxRsp(tx *pb.Transaction, block *pb.Block) (TxDetailRsp, error) {
	var rsp TxDetailRsp
	rsp.TxId = hex.EncodeToString(tx.Txid)
	rsp.BlockId = hex.EncodeToString(tx.Blockid)
	rsp.BlockHeight = block.Block.Height
	rsp.Coinbase = tx.Coinbase
	rsp.Miner = string(block.Block.Proposer)
	timestamp := block.Block.Timestamp / 1e9
	tm1 := time.Unix(timestamp, 0)
	rsp.BlockTimestamp = tm1.Local().Format("2006-01-02 15:04:05")
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
	tm := time.Unix(tx.Timestamp/1e9, 0)
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
	timestamp := block.Block.Timestamp / 1e9
	tm := time.Unix(timestamp, 0)
	rsp.Timestamp = tm.Local().Format("2006-01-02 15:04:05")
	rsp.TxNumber = block.Block.TxCount
	for _, v := range block.Block.Transactions {
		rsp.Txs = append(rsp.Txs, hex.EncodeToString(v.Txid))
	}
	return rsp, nil
}

// type ContractCountReq struct {
// 	ChainName string `json:"chain_name"`
// }

// type TxQueryReq struct {
// 	ChainName string `json:"chain_name"`
// 	Input     string `json:"input"`
// }

type BlockRsp struct {
	BlockHeight int64    `json:"block_height"`
	BlockId     string   `json:"block_id"`
	Miner       string   `json:"miner"`
	NextHash    string   `json:"next_hash"`
	PreHash     string   `json:"pre_hash"`
	Timestamp   string   `json:"timestamp"`
	TxNumber    int32    `json:"tx_number"`
	Txs         []string `json:"txs"`
}

type TxDetailRsp struct {
	TxId           string   `json:"tx_id"`
	BlockId        string   `json:"block_id"`
	BlockHeight    int64    `json:"block_height"`
	Coinbase       bool     `json:"coinbase"`
	Miner          string   `json:"miner"`
	BlockTimestamp string   `json:"block_timestamp"`
	Initiator      string   `json:"initiator"`
	FromAddress    []string `json:"from_address"`
	ToAddresses    []string `json:"to_addresses"`
	FromTotal      *big.Int `json:"from_total"`
	ToTotal        *big.Int `json:"to_total"`
	Fee            int64    `json:"fee"`
	Date           string   `json:"date"`
	Contracts      []string `json:"contracts"`
}

const (
	ByTxId = iota + 1
	ByBlockHeight
	ByBlockId
)
