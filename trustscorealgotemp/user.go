package main

import "time"

type Peer struct {
	globaldata GlobalDataStruct
	frienddata FriendDataStruct
}

type GlobalDataStruct struct {
	LeechedData             float64
	SeededData              float64
	SinceLastSeen           time.Time
	AverageUploadSpeed      int
	AverageOnlineTimePerDay int
	TotalFileChunksUploaded int
	CurrTraSuccChunk        int
	CurrTraTotalChunk       int
	OriginalChunksShared    int
	NoOfSuccChunksLastK     int
}



type FriendDataStruct struct {
	Friends []Friend
}
