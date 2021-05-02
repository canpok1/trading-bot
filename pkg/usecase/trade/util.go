package trade

import (
	"trading-bot/pkg/domain/model"
)

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

// func SupportLine(rates []float64, term1, term2 int) ([]float64, float64) {
// 	term1End := len(rates) - 1
// 	term1Begin := term1End - (term1 - 1)
// 	term1Min, term1MinIdx := MinRate(rates[term1Begin : term1End-1])
// 	term1MinIdx = term1MinIdx + term1Begin
//
// 	term2End := term1Begin - 1
// 	term2Begin := term2End - (term2 - 1)
// 	term2Min, term2MinIdx := MinRate(rates[term2Begin : term2End-1])
// 	term2MinIdx = term2MinIdx + term2Begin
//
// 	a := (term1Min - term2Min) / float64(term1MinIdx-term2MinIdx)
// 	b := term1Min - float64(term1MinIdx)*a
//
// 	supportLine := []float64{}
// 	for x := range rates {
// 		supportLine = append(supportLine, float64(x)*a+b)
// 	}
//
// 	return supportLine, a
// }

func SupportLine(rates []float64, period, periodInterval int) ([]float64, float64) {
	term1End := len(rates) - 1
	term1Begin := term1End - (period - 1)
	term1Min, term1MinIdx := MinRate(rates[term1Begin:term1End])
	term1MinIdx = term1MinIdx + term1Begin

	term2End := term1MinIdx - 1 - periodInterval
	term2Begin := term2End - (period - 1)
	term2Min, term2MinIdx := MinRate(rates[term2Begin : term2End-1])
	term2MinIdx = term2MinIdx + term2Begin

	a := (term1Min - term2Min) / float64(term1MinIdx-term2MinIdx)
	b := term1Min - float64(term1MinIdx)*a

	supportLine := []float64{}
	for x := range rates {
		supportLine = append(supportLine, float64(x)*a+b)
	}

	return supportLine, a
}

func ResistanceLine(rates []float64, period, periodInterval int) ([]float64, float64) {
	term1End := len(rates) - 1
	term1Begin := term1End - (period - 1)
	term1Max, term1MaxIdx := MaxRate(rates[term1Begin:term1End])
	term1MaxIdx = term1MaxIdx + term1Begin

	term2End := term1MaxIdx - 1 - periodInterval
	term2Begin := term2End - (period - 1)
	term2Max, term2MaxIdx := MaxRate(rates[term2Begin : term2End-1])
	term2MaxIdx = term2MaxIdx + term2Begin

	a := (term1Max - term2Max) / float64(term1MaxIdx-term2MaxIdx)
	b := term1Max - float64(term1MaxIdx)*a

	supportLine := []float64{}
	for x := range rates {
		supportLine = append(supportLine, float64(x)*a+b)
	}

	return supportLine, a
}

func SupportLine2(rates []float64, beginIdx, endIdx int) (a, b float64) {
	begin := true
	for {
		x := []float64{}
		y := []float64{}
		for i, rate := range rates {
			if i < beginIdx || i > endIdx {
				continue
			}
			if begin || rate <= a*float64(i)+b {
				x = append(x, float64(i))
				y = append(y, rate)
			}
		}

		if len(x) <= 3 {
			return
		}

		a, b = LinFit(x, y)
		begin = false
	}
}

func ResistanceLine2(rates []float64, beginIdx, endIdx int) (a, b float64) {
	begin := true
	for {
		x := []float64{}
		y := []float64{}
		for i, rate := range rates {
			if i < beginIdx || i > endIdx {
				continue
			}
			if begin || rate >= a*float64(i)+b {
				x = append(x, float64(i))
				y = append(y, rate)
			}
		}

		if len(x) <= 3 {
			return
		}

		a, b = LinFit(x, y)
		begin = false
	}
}

func MakeLine(a, b float64, size int) []float64 {
	line := []float64{}
	for i := 0; i < size; i++ {
		line = append(line, a*float64(i)+b)
	}
	return line
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

// CalcAmount 未決済分の購入金額を算出
func CalcAmount(pair *model.CurrencyPair, cc []model.Contract, keyAmount, fraction float64) (usedJPY float64, obtainedCurrency float64) {
	tmp := keyAmount
	for _, c := range cc {
		if tmp < fraction {
			break
		}
		if c.DecreaseCurrency == pair.Settlement && c.IncreaseCurrency == pair.Key {
			// 買い注文
			usedJPY -= c.DecreaseAmount
			obtainedCurrency += c.IncreaseAmount
			tmp -= c.IncreaseAmount
			continue
		}
		if c.DecreaseCurrency == pair.Settlement && c.IncreaseCurrency == pair.Key {
			// 売り注文
			usedJPY -= c.IncreaseAmount
			obtainedCurrency += c.DecreaseAmount
			tmp -= c.DecreaseAmount
			continue
		}
	}

	return
}

func LinFit(x, y []float64) (a, b float64) {
	var sx, sy, t, st2 float64
	ndata := len(x)
	if ndata < 2 {
		return
	}

	for i := 0; i < ndata; i++ {
		sx += x[i]
		sy += y[i]
	}

	ss := float64(ndata)
	sxoss := sx / ss
	for i := 0; i < ndata; i++ {
		t = x[i] - sxoss
		st2 += t * t
		a += t * y[i]
	}
	a /= st2

	b = (sy - sx*a) / ss
	return
}
