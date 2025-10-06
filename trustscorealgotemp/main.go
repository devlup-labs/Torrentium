package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"time"
)

func main() {

	trackerop := []int{1, 2, 3} //trascker output

	// Read friends.json
	friendsFile, err := os.Open("friends.json")
	if err != nil {
		panic(err)
	}
	defer friendsFile.Close()
	friendsBytes, _ := io.ReadAll(friendsFile)
	var friends []Friend
	if err := json.Unmarshal(friendsBytes, &friends); err != nil {
		panic(err)
	}

	// Read all_peers.json
	peersFile, err := os.Open("all_peers.json")
	if err != nil {
		panic(err)
	}
	defer peersFile.Close()
	peersBytes, _ := io.ReadAll(peersFile)
	var allPeers []struct {
		ID               int     `json:"id"`
		GlobalTrustScore float64 `json:"global_trust_score"`
	}
	if err := json.Unmarshal(peersBytes, &allPeers); err != nil {
		panic(err)
	}

	// Build initialTrustScores for peers in trackerop
	initialTrustScores := make([]float64, 0, len(trackerop))
	for _, peerID := range trackerop {
		// Find global trust score from allPeers
		globalScore := -1.0
		for _, peer := range allPeers {
			if peer.ID == peerID {
				globalScore = peer.GlobalTrustScore
				break
			}
		}
		if globalScore < 0.0 {

			continue
		}
		// Check if peer in friends
		friendScore := -1.0
		for _, f := range friends {
			if f.ID == peerID {
				friendScore = f.TrustScore
				break
			}
		}
		if friendScore >= 0.0 {
			// Peer is in both lists, take average
			initialTrustScores = append(initialTrustScores, (globalScore+friendScore)/2.0)
		} else {
			// Only inallPeers
			initialTrustScores = append(initialTrustScores, globalScore)
		}
	}
	fmt.Println("Initial Trust Scores:", initialTrustScores)
	fmt.Println("PRESENT===================")
	weights := Weights{
		SuccessfulChunks:  0.3,
		UploadSpeed:       0.15,
		SeedLeechRatio:    0.2,
		SuccessRate:       0.15,
		ChunkAvailability: 0.1,
		OnlineTime:        0.1,
	}

	// top 2 peers
	type peerScore struct {
		idx    int
		peerID int
		score  float64
	}
	var scores []peerScore
	for i, peerID := range trackerop {
		scores = append(scores, peerScore{i, peerID, initialTrustScores[i]})
	}

	//bubble sorting
	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}
	if len(scores) > 2 {
		scores = scores[:2]
	}

	for _, s := range scores {
		peerID := s.peerID
		fmt.Printf("\ntop peer: %d\n", peerID)

		//for global score
		// Find the actual global trust score for this peer
		globalScore := -1.0
		for _, peer := range allPeers {
			if peer.ID == peerID {
				globalScore = peer.GlobalTrustScore
				break
			}
		}
		fmt.Println("Enter data for new global trust score of peer ", peerID, " calculation:") //rn input is auto and same for all peers
		newGlobal := CalculateTrustScoreWithInput(globalScore, weights)
		fmt.Printf("New Global Trust Score for Peer %d: %f\n", peerID, newGlobal)

		// Check if peer is a friend and get friend initial trust score
		friendScore := -1.0
		for _, f := range friends {
			if f.ID == peerID {
				friendScore = f.TrustScore
				break
			}
		}
		if friendScore >= 0.0 {
			fmt.Println("Enter data for friend trust score of peer ", peerID, " calculation:")
			newLocal := CalculateTrustScoreWithInput(friendScore, weights)
			fmt.Printf("New Local (Friend) Trust Score for Peer %d: %f\n", peerID, newLocal)
		}
	}
}

// CalculateTrustScoreWithInput prompts for PeerData input and computes the trust score
func CalculateTrustScoreWithInput(currentTrustScore float64, weights Weights) float64 {
	data := PeerData{
		LeechedData:             3.0,
		SeededData:              1.0,
		SinceLastSeen:           time.Date(2025, 8, 20, 0, 0, 0, 0, time.UTC),
		AverageUploadSpeed:      1000,
		AverageOnlineTimePerDay: 10,
		TotalFileChunksUploaded: 250,
		CurrTraSuccChunk:        400,
		CurrTraTotalChunk:       500,
		OriginalChunksShared:    1000,
		NoOfSuccChunksLastK:     27,
		TransSucc:               false,
	}

	return CalculateTrustScore(currentTrustScore, data, weights)
}

// CalculateTrustScore computes the trust score for a peer
func CalculateTrustScore(currentTrustScore float64, data PeerData, weights Weights) float64 {
	leechedBySeeded := data.LeechedData / data.SeededData
	currentDate := time.Now()
	daysSinceLastSeen := int(currentDate.Sub(data.SinceLastSeen).Hours() / 24)
	ratiosucctotal := float64(data.CurrTraSuccChunk) / float64(data.CurrTraTotalChunk)
	decayFactor := -math.Exp(0.1*float64(daysSinceLastSeen)) / 100

	maxPossibleIncrease := 1.0 - currentTrustScore
	alpha := 0.5 * (1 - math.Abs(0.5-currentTrustScore))

	scalingFactor := math.Min(1.0, maxPossibleIncrease/(alpha*2.0))
	if scalingFactor < 0.1 {
		scalingFactor = 0.1
	}

	// Calculate base increase in trust score
	increaseInTrustScore := scalingFactor*(weights.SuccessfulChunks*(math.Log1p(float64(data.NoOfSuccChunksLastK))/math.Log1p(100))+
		weights.UploadSpeed*(math.Log1p(float64(data.AverageUploadSpeed))/math.Log1p(10000))+
		weights.SeedLeechRatio*(1/(1+leechedBySeeded))+
		weights.SuccessRate*ratiosucctotal+
		weights.ChunkAvailability*math.Min(1.0, float64(data.TotalFileChunksUploaded)/float64(data.OriginalChunksShared))+
		weights.OnlineTime*math.Min(1.0, float64(data.AverageOnlineTimePerDay)/24.0)) 

	// Apply penalties for bad behavior
	penalty := 0.0
	//things to penalise for - bad leech seed ratio, low success rate, very low upload speed

	if ratiosucctotal < 0.5 { // Low success rate
		penaltyFactor := math.Max(0.05, currentTrustScore*0.3)
		penalty += penaltyFactor
	}

	if leechedBySeeded > 2.0 {
		penaltyFactor := math.Max(0.05, currentTrustScore*0.25)
		penalty += penaltyFactor
	}

	if float64(data.AverageUploadSpeed) < 100 { // Very low upload speed
		penaltyFactor := math.Max(0.05, currentTrustScore*0.15)
		penalty += penaltyFactor
	}

	if float64(data.NoOfSuccChunksLastK) < 10 { // Very few successful chunks recently
		penaltyFactor := math.Max(0.05, currentTrustScore*0.2) // 5% min, up to 20% of current score
		penalty += penaltyFactor
	}

	// Apply the penalty
	succfactor := 1.0
	if !data.TransSucc {
		succfactor = 0.0
	}
	newTrustScore := currentTrustScore + succfactor*alpha*increaseInTrustScore - penalty
	if newTrustScore > 1.0 {
		newTrustScore = 1.0
	}
	if newTrustScore < 0.0 {
		newTrustScore = 0.0
	}
	return newTrustScore * 100
}
