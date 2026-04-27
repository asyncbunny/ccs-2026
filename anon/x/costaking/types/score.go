package types

import (
	sdkmath "cosmossdk.io/math"
)

// UpdateScore updates the TotalScore property of the costaker rewards tracker based on the current
// values of ActiveSatoshis, ActiveNtk and the given scoreRatioBtcByNtk by parameter.
// The formula for the total score is defined as Min(total active btc staked, total active ntk staked / ratio)
// It also returns the delta value difference from the (current total score - previous score).
// The returned value can be negative, meaning that the current total score is lower than the previous score.
func (c *CostakerRewardsTracker) UpdateScore(scoreRatioBtcByNtk sdkmath.Int) (deltaPreviousScore sdkmath.Int) {
	previousTotalScore := c.TotalScore

	c.TotalScore = CalculateScore(scoreRatioBtcByNtk, c.ActiveNtk, c.ActiveSatoshis)
	return c.TotalScore.Sub(previousTotalScore)
}

// CalculateScore only calculates the total score based on the func min(ActiveSats, ActiveNtk/ScoreRatioBtcByNtk)
func CalculateScore(scoreRatioBtcByNtk, activeNtk, activeSats sdkmath.Int) (totalScore sdkmath.Int) {
	activeNtkByRatio := activeNtk.Quo(scoreRatioBtcByNtk)
	return sdkmath.MinInt(activeSats, activeNtkByRatio)
}
