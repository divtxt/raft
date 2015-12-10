package raft

// Validate time settings.
// tickerDurtaion must be greater than zero.
// electionTimeout must be greater than tickerDuration.
// These checks are basic sanity checks and currently don't include the
// softer usefulness checks recommended by the raft protocol.
func ValidateTimeSettings(timeSettings TimeSettings) string {
	if timeSettings.TickerDuration.Nanoseconds() <= 0 {
		return "TickerDuration must be greater than zero"
	}
	if timeSettings.ElectionTimeoutLow.Nanoseconds() <= timeSettings.TickerDuration.Nanoseconds() {
		return "ElectionTimeoutLow must be greater than TickerDuration"
	}

	return ""
}
