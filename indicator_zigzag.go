package gotrader

type ZigZagStrategy struct {
}

var (

	// zigZagInstances keeps track of instnce of the indicator for each symbol.
	// The ZigZag indicator is a patter-n-follow, and relies on historical data
	zigZagInstances = map[Symbol]ZigZagStrategy{}
)

func ZigZag(candles []Candle) []Candle {
	changePerc := 0.2
	var zigzag []Candle
	var zigzagVal []float64

	// Start with the first candle
	zigzag = append(zigzag, candles[0])
	zigzagVal = append(zigzagVal, candles[0].Low)

	currentTrend := 1 // 1 for rising, -1 for falling
	prevHigh := candles[0].High
	prevLow := candles[0].Low

	for i := 1; i < len(candles); i++ {
		currentCandle := candles[i]

		// Calculate the percentage change from the previous high and low
		percentChangeHigh := (currentCandle.High - prevHigh) / prevHigh * 100
		percentChangeLow := (currentCandle.Low - prevLow) / prevLow * 100

		// If the current candle reverses the trend, add it to the zigzag list
		if (currentTrend == 1 && percentChangeLow <= -changePerc) || (currentTrend == -1 && percentChangeHigh >= changePerc) {
			zigzag = append(zigzag, currentCandle)
			currentTrend *= -1 // Reverse the trend direction
			prevHigh = currentCandle.High
			prevLow = currentCandle.Low
		} else {
			// Update the previous high and low based on the current candle
			if currentCandle.High > prevHigh {
				prevHigh = currentCandle.High
			}
			if currentCandle.Low < prevLow {
				prevLow = currentCandle.Low
				if currentCandle.High > prevHigh {
					prevHigh = currentCandle.High
				}
			}
		}
	}

	return zigzag
}
