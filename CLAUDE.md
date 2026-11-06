Go-trader is a framework for backtesting and trading
A `Datafeed` datafeed.go is an interface to conenct the trading strateiges with data provider, like Alpaca or InteractiveBrokers
Their implementmentaiton are in the `interactivebrokers` and `alpaca` packages.
When the user asks what we were working on, read alligator/history.txt: one line per session, "<session id> | <summary>", newest first.
