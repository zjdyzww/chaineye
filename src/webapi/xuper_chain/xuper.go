package xuper_chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"gitee.com/chunanyong/zorm"
	"github.com/didi/nightingale/v5/src/models"
	"github.com/didi/nightingale/v5/src/webapi/config"
	"github.com/toolkits/pkg/logger"
	"github.com/xuperchain/xuper-sdk-go/v2/xuper"
)

func SyncXuperBlockTimer() {
	duration := time.Duration(3) * time.Second
	for {
		time.Sleep(duration)
		SyncXuperBlock()
	}
}

// 三秒同步一次 最新区块，以及交易数量
func SyncXuperBlock() {
	ctx := context.TODO()
	// 查询最新区块高度
	xuperClient, err := xuper.New(config.C.XuperChain.XuperChainNode, xuper.WithConfigFile(config.C.XuperChain.XuperSdkYmlPath))
	if err != nil {
		logger.Error("sync xuper chain block failed", err.Error())
		return
	}
	defer xuperClient.Close()
	b, err := xuperClient.QuerySystemStatus()
	if err != nil {
		logger.Error("get chain system status failed", err.Error())
		return
	}
	var newHeight = b.SystemsStatus.BcsStatus[0].Block.Height
	heightInDB, err := GetMaxHeightInDB(ctx)
	if err != nil {
		logger.Error("get height max in db failed", err.Error())
		return
	}
	txCounts := heightInDB.TotalTxCount
	for i := heightInDB.BlockHeight + 1; i <= newHeight; i++ {
		// 解析区块
		b2, err := xuperClient.QueryBlockByHeight(i)
		if err != nil {
			logger.Error("get height by height failed", err.Error())
			continue
		}
		var dataToDB models.XuperStruct
		dataToDB.BlockHeight = i
		dataToDB.BlockHash = hex.EncodeToString(b2.Blockid)
		// 截取十位时间戳  到秒级
		s := strconv.FormatInt(b2.Block.Timestamp, 10)[0:10]
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			logger.Error("timestamp string to int64 failed", err.Error())
			continue
		}
		dataToDB.Timestamp = i
		dataToDB.BlockTxCount = int64(b2.Block.TxCount)
		txCounts = txCounts + int64(b2.Block.TxCount)
		dataToDB.TotalTxCount = txCounts

		_, err = zorm.Transaction(ctx, func(ctx context.Context) (interface{}, error) {
			_, err := zorm.Insert(ctx, &dataToDB)
			return nil, err
		})
		if err != nil {
			logger.Error("insert into db XuperStruct failed", err.Error())
			continue
		}
	}
}

// 定期清除区块数据，交易数据，每天执行一次。删除30天之前的数据
func DeleteXuperDataTimer() {
	duration := time.Duration(24) * time.Hour
	for {
		time.Sleep(duration)
		DeleteXuperData()
	}
}

func DeleteXuperData() {
	// 获取数据库最新一条区块高度
	ctx := context.TODO()
	heightInDB, err := GetMaxHeightInDB(ctx)
	if err != nil {
		logger.Error("get height max in db failed", err.Error())
		return
	}
	lastTime := heightInDB.Timestamp
	oldTime := lastTime - 3600*24*10
	// 删除 oldTime之前的数据
	_, err = zorm.Transaction(ctx, func(ctx context.Context) (interface{}, error) {
		finder := zorm.NewDeleteFinder(models.XuperStructTableName)
		finder.Append("where timestamp <= ?", oldTime)
		_, err := zorm.UpdateFinder(ctx, finder)
		return nil, err
	})
	if err != nil {
		logger.Error("delete old block info in db XuperStruct failed", err.Error())
		return
	}
}

func GetMaxHeightInDB(ctx context.Context) (models.XuperStruct, error) {
	// 查询数据库最新区块高度
	f := zorm.NewSelectFinder(models.XuperStructTableName)
	f.Append("ORDER BY block_height DESC")
	p := zorm.NewPage()
	p.PageSize = 1
	p.PageNo = 1
	var heightInDB []models.XuperStruct
	err := zorm.Query(ctx, f, &heightInDB, p)
	if err != nil {
		logger.Error("get height in db  failed", err.Error())
		return heightInDB[0], err
	}
	if len(heightInDB) <= 0 {
		logger.Error("heightInDB <= 0")
		return heightInDB[0], fmt.Errorf("heightInDB <= 0")
	}
	return heightInDB[0], nil
}

func GetTxCountsSection(ctx context.Context, timestamp int64) (int64, error) {
	var countInSection int64
	// 查询数据库最新区块高度
	f := zorm.NewSelectFinder(models.XuperStructTableName)
	f.Append("WHERE timestamp <= ? ORDER BY block_height DESC", timestamp)
	p := zorm.NewPage()
	p.PageSize = 1
	p.PageNo = 1
	var heightInDB []models.XuperStruct
	err := zorm.Query(ctx, f, &heightInDB, p)
	if err != nil {
		logger.Error("get height in db  failed", err.Error())
		return countInSection, err
	}
	if len(heightInDB) <= 0 {
		logger.Error("heightInDB <= 0")
		return 0, nil
	}
	return heightInDB[0].TotalTxCount, nil
}
