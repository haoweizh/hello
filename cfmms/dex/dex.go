package dex

type Dex interface {
	PoolCreatedEventSignature() []byte
	NewPoolFromEvent(log any)
	NewEmptyPoolFromEvent(log any)
	GetAllPools()
	GetAllPoolsData()

	GetPolWithBestLiquidity()
	GetAllPoolsForPair()
	GetAllPoolsFromLogs()
	GetAllPoolsFromLogsWithinRange()
}
