package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"time"
	"trading-bot/pkg/domain"
	"trading-bot/pkg/domain/model"
	"trading-bot/pkg/infrastructure/memory"
	"trading-bot/pkg/usecase"
	"trading-bot/pkg/usecase/strategy"
	"trading-bot/pkg/usecase/trade"

	"github.com/BurntSushi/toml"
	"github.com/kelseyhightower/envconfig"
)

const (
	currencyType  = model.MONA
	population    = 20
	maxGeneration = 100
	maxErrorCount = 5
	selectionRate = 0.050
	crossoverRate = 0.940
	mutationRate  = 0.010
)

var (
	goodGene = []int{360, 864, 572, 990, 888, 220}
)

type Individual struct {
	Profit *float64
	Gene   Gene
}

// Gene 遺伝子
// 値は0～999
type Gene []int

func (g *Gene) MakeConfig() *strategy.ScalpingConfig {
	v := []int(*g)
	return &strategy.ScalpingConfig{
		TargetCurrency:         string(currencyType),
		IntervalSeconds:        0,
		PositionCountMax:       1,
		FundsRatio:             0.3,
		ShortTermSize:          v[0],
		LongTermSize:           v[0] + v[1],
		LossCutLowerLimitPer:   float64(v[2]) / 1000.0,
		FixProfitUpperLimitPer: float64(v[3]) / 1000.0,
		BBandsNBDevUp:          float64(v[4]) / 1000.0,
		BBandsNBDevDown:        float64(v[5]) / 1000.0,
	}
}

func (i *Individual) String() string {
	return fmt.Sprintf("%v", i.Gene)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	logger := memory.Logger{Level: memory.Info}

	logger.Info("===== START GA SIMULATION ====================")
	defer logger.Info("===== END GA SIMULATION ======================")

	var individuals []*Individual
	for gi := 1; gi <= maxGeneration; gi++ {
		logger.Info("***** Generation %d *****", gi)

		// 個体群を生成
		if len(individuals) == 0 {
			individuals = makeInitIndividuals(population)
		} else {
			nextIndividual := []*Individual{}
			for {
				nextIndividual = append(nextIndividual, makeNextIndividual(individuals))
				if len(nextIndividual) == population {
					break
				}
			}
			individuals = nextIndividual
		}

		// 個体群の成績を評価
		for _, individual := range individuals {
			logger.Info("running simulation for %s ...", individual.String())
			errCount := 0
			for {
				p, err := simulation(&logger, "./configs/simulator.toml", &individual.Gene)
				if err != nil {
					logger.Error("error occured; %v", err)
					errCount++
					if errCount >= maxErrorCount {
						logger.Error("terminate simulation [%d/%d]", errCount, maxErrorCount)
						return
					}
				}
				individual.Profit = &p
				break
			}
		}

		// 利益順にソート
		sort.SliceStable(individuals, func(i, j int) bool {
			return *individuals[i].Profit > *individuals[j].Profit
		})

		for _, individual := range individuals {
			logger.Info("result %s => %.3f", individual.String(), *individual.Profit)
		}
	}

	logger.Info("***** completed !!! *****")
	bestInd := individuals[0]
	logger.Info("best profit: %.3f", *bestInd.Profit)
	logger.Info("params: %#v", *bestInd.Gene.MakeConfig())
}

func makeInitIndividuals(size int) []*Individual {
	individuals := []*Individual{{Profit: nil, Gene: goodGene}}

	for {
		individuals = append(individuals, makeRandomIndividual())
		if len(individuals) == size {
			break
		}
	}
	return individuals
}

func makeNextIndividual(current []*Individual) *Individual {
	v := rand.Intn(1000)
	if v <= int(selectionRate*1000) {
		// 再生
		return choose(current)
	}
	if v <= int((selectionRate+crossoverRate)*1000) {
		// 交叉
		var ind1, ind2 *Individual
		for {
			ind1 = choose(current)
			ind2 = choose(current)
			if !reflect.DeepEqual(ind1, ind2) {
				break
			}
		}
		return crossover(ind1, ind2)
	}

	// 突然変異
	return mutate(choose(current))
}

// choose 選択
func choose(current []*Individual) *Individual {
	// ランキング上位の個体ほど選択されやすいようにする
	size := len(current)
	list := []*Individual{}
	for i, ind := range current {
		count := size - i
		for j := 0; j < count; j++ {
			list = append(list, ind)
		}
	}

	n := rand.Intn(len(list))
	return list[n]
}

func crossover(ind1 *Individual, ind2 *Individual) *Individual {
	size := len(ind1.Gene)
	var idx1, idx2 int
	for {
		idx1 = rand.Intn(size)
		idx2 = rand.Intn(size)
		if idx1 != idx2 {
			break
		}
	}

	var begin, end int
	if idx1 < idx2 {
		begin = idx1
		end = idx2
	} else {
		begin = idx2
		end = idx1
	}

	newGene := []int{}
	for i := 0; i < size; i++ {
		if i < begin || i > end {
			newGene = append(newGene, ind1.Gene[i])
		} else {
			newGene = append(newGene, ind2.Gene[i])
		}
	}

	return &Individual{Gene: newGene}
}

func mutate(org *Individual) *Individual {
	size := len(org.Gene)
	n := rand.Intn(size)

	newGene := []int{}
	for i := 0; i < size; i++ {
		if i == n {
			newGene = append(newGene, rand.Intn(1000))
		} else {
			newGene = append(newGene, org.Gene[i])
		}
	}
	return &Individual{Gene: newGene}
}

func makeRandomIndividual() *Individual {
	gene := []int{}
	for i := 0; i < 6; i++ {
		gene = append(gene, rand.Intn(1000))
	}

	return &Individual{
		Profit: nil,
		Gene:   gene,
	}
}

func simulation(logger domain.Logger, configPath string, gene *Gene) (float64, error) {
	var conf model.Config
	if err := envconfig.Process("BOT", &conf); err != nil {
		return 0, err
	}

	var sConf model.SimulatorConfig
	if _, err := toml.DecodeFile(configPath, &sConf); err != nil {
		return 0, err
	}

	historical, err := os.Open(sConf.RateHistoryFile)
	if err != nil {
		return 0, err
	}
	exCli, err := memory.NewExchangeMock(historical, sConf.Slippage)
	if err != nil {
		return 0, err
	}

	rateRepo := memory.NewRateRepository(sConf.RateHistorySize)
	rdsCli := memory.NewDummyRDS()

	facade := trade.NewFacade(exCli, rateRepo, rdsCli, rdsCli, rdsCli)

	strategy, err := strategy.NewScalpingStrategy(facade, logger, gene.MakeConfig())
	if err != nil {
		return 0, err
	}

	simulator := usecase.Simulator{
		Strategy:     strategy,
		TradeRepo:    rdsCli,
		ExchangeMock: exCli,
		Logger:       logger,
	}

	return simulator.Run(context.Background())
}
