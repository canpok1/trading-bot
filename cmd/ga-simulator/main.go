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

	"github.com/kelseyhightower/envconfig"
)

const (
	geneSize      = 8
	population    = 40
	maxGeneration = 100
	maxErrorCount = 5
	//maxConvergedCount = 3
	selectionRate = 0.020
	crossoverRate = 0.975
	mutationRate  = 0.005

	randomPopulationCount        = 4
	randomPopulationBornInterval = 10
)

var (
	// [197 440 268 886 994 391 176] => 10.254
	//goodGene = []int{197, 440, 268, 886, 994, 391, 176}
	goodGene = []int{}
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
		FundsRatio:             0.3,
		ShortTermSize:          v[0],
		LongTermSize:           v[0] + v[1],
		LossCutLowerLimitPer:   float64(v[2]) / 1000.0,
		FixProfitUpperLimitPer: 1.0 + float64(v[3])/1000.0,
		BBandsNBDevUp:          float64(v[4]) / 300.0,
		BBandsNBDevDown:        float64(v[5]) / 300.0,
		RsiLower:               float64(v[6]) / 10.0,
		RsiUpper:               float64(v[6])/10.0 + float64(v[7])/10.0,
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
	gi := 1
	convergedCount := 0
	for {
		logger.Info("***** Generation %d *****", gi)

		// 個体群を生成
		if len(individuals) == 0 {
			individuals = makeInitIndividuals(population, goodGene)
			//} else if convergedCount >= maxConvergedCount {
			//	individuals = makeInitIndividuals(population, individuals[0].Gene)
		} else {
			nextIndividual := []*Individual{}

			// 一番成績のいい個体は続投
			nextIndividual = append(nextIndividual, individuals[0])

			// 多様性維持のため定期的にランダムな個体を追加する
			if gi%randomPopulationBornInterval == 0 {
				for n := 0; n < randomPopulationCount; n++ {
					nextIndividual = append(nextIndividual, makeRandomIndividual())
				}
			}

			for {
				nextIndividual = append(nextIndividual, makeNextIndividual(individuals))
				if len(nextIndividual) == population {
					break
				}
			}
			individuals = nextIndividual
		}

		// 個体群の成績を評価
		for i, individual := range individuals {
			logger.Info("running simulation [%d/%d] %s ...", i+1, len(individuals), individual.String())
			errCount := 0
			for {
				p, err := simulation(&logger, &individual.Gene)
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

		bestInd := individuals[0]
		logger.Info("best profit: %.3f", *bestInd.Profit)
		logger.Info("gene: %v", bestInd.Gene)
		logger.Info("params: %#v", *bestInd.Gene.MakeConfig())

		if isConverged(individuals) {
			convergedCount++
		} else {
			convergedCount = 0
		}

		if gi >= maxGeneration {
			logger.Info("***** max generation *****")
			break
		}
		gi++
	}

	logger.Info("***** completed !!! *****")
}

func makeInitIndividuals(size int, goodGene []int) []*Individual {
	individuals := []*Individual{}
	if len(goodGene) == geneSize {
		individuals = append(individuals, &Individual{Profit: nil, Gene: goodGene})
	}

	for {
		individuals = append(individuals, makeRandomIndividual())
		if len(individuals) == size {
			break
		}
	}
	return individuals
}

func makeNextIndividual(current []*Individual) *Individual {
	v := randValue()
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
			newGene = append(newGene, randValue())
		} else {
			newGene = append(newGene, org.Gene[i])
		}
	}
	return &Individual{Gene: newGene}
}

func makeRandomIndividual() *Individual {
	gene := []int{}
	for i := 0; i < geneSize; i++ {
		gene = append(gene, randValue())
	}

	return &Individual{
		Profit: nil,
		Gene:   gene,
	}
}

func simulation(logger domain.Logger, gene *Gene) (float64, error) {
	var conf model.Config
	if err := envconfig.Process("BOT", &conf); err != nil {
		return 0, err
	}

	var sConf model.SimulatorConfig
	if err := envconfig.Process("BOT", &sConf); err != nil {
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

	rdsCli := memory.NewDummyRDS(nil)

	facade := trade.NewFacade(exCli, rdsCli, rdsCli, rdsCli, rdsCli, nil)

	currency := model.CurrencyType(conf.TargetCurrency)
	strategy, err := strategy.NewScalpingStrategy(facade, logger, gene.MakeConfig())
	if err != nil {
		return 0, err
	}

	bot := usecase.NewBot(logger, facade, strategy, &usecase.BotConfig{
		Currency:        currency,
		IntervalSeconds: 0,
	})

	pair := model.CurrencyPair{
		Key:        model.CurrencyType(conf.TargetCurrency),
		Settlement: model.JPY,
	}
	fetcher := usecase.NewFetcher(exCli, pair, rdsCli)

	simulator := usecase.Simulator{
		Bot:          bot,
		Fetcher:      fetcher,
		TradeRepo:    rdsCli,
		ExchangeMock: exCli,
		Logger:       logger,
	}

	return simulator.Run(context.Background())
}

func isConverged(individuals []*Individual) bool {
	isSame := true
	for i := range individuals {
		if i == 0 {
			continue
		}
		if *individuals[i].Profit != *individuals[i-1].Profit {
			isSame = false
		}
	}

	return isSame
}

func randValue() int {
	return rand.Intn(1000)
}
