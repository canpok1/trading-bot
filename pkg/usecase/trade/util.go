package trade

import "trading-bot/pkg/domain/model"

func MaxRate(rates []float64) (float64, int) {
	max := rates[0]
	maxIndex := 0
	for i := 0; i < len(rates); i++ {
		rate := rates[i]
		if rate > max {
			max = rate
			maxIndex = i
		}
	}
	return max, maxIndex
}

func MinRate(rates []float64) (float64, int) {
	min := rates[0]
	minIndex := 0
	for i := 0; i < len(rates); i++ {
		rate := rates[i]
		if rate < min {
			min = rate
			minIndex = i
		}
	}
	return min, minIndex
}

func SupportLine(rates []float64, term1, term2 int) []float64 {
	term1End := len(rates) - 1
	term1Begin := term1End - (term1 - 1)
	term1Min, term1MinIdx := MinRate(rates[term1Begin : term1End-1])
	term1MinIdx = term1MinIdx + term1Begin

	term2End := term1Begin - 1
	term2Begin := term2End - (term2 - 1)
	term2Min, term2MinIdx := MinRate(rates[term2Begin : term2End-1])
	term2MinIdx = term2MinIdx + term2Begin

	a := (term1Min - term2Min) / float64(term1MinIdx-term2MinIdx)
	b := term1Min - float64(term1MinIdx)*a

	supportLine := []float64{}
	for x := range rates {
		supportLine = append(supportLine, float64(x)*a+b)
	}

	return supportLine
}

func GetLastBuyContracts(pair *model.CurrencyPair, cc []model.Contract) []model.Contract {
	buyContracts := []model.Contract{}
	for i := 0; i < len(cc); i++ {
		c := cc[i]
		if c.Side != model.BuySide {
			break
		}
		if c.IncreaseCurrency == pair.Key && c.DecreaseCurrency == pair.Settlement {
			buyContracts = append(buyContracts, c)
		}
	}
	return buyContracts
}
