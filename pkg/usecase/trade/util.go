package trade

func (f *Facade) MaxRate(rates []float64) (float64, int) {
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

func (f *Facade) MinRate(rates []float64) (float64, int) {
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
