package server

import (
	"SOMAS2023/internal/common/objects"
	"SOMAS2023/internal/common/utils"
	"SOMAS2023/internal/common/voting"
	"fmt"

	"github.com/google/uuid"
)

func (s *Server) RunRulerAction(bike objects.IMegaBike, governance utils.Governance) uuid.UUID {
	// vote for dictator
	agents := bike.GetAgents()
	ruler := s.RulerElection(agents, governance)
	// communicate dictator
	bike.SetRuler(ruler)
	// get dictators direction choice
	rulerAgent := s.GetAgentMap()[ruler]
	var direction uuid.UUID
	switch governance {
	case utils.Dictatorship:
		direction = rulerAgent.DictateDirection()
	case utils.Leadership:
		direction = rulerAgent.LeadDirection()
	}
	return direction
}

func (s *Server) RulerElection(agents []objects.IBaseBiker, governance utils.Governance) uuid.UUID {
	// TODO: need extra input "voteWeight". For now, we just initialise a unit weight for each agent
	votes := make(map[uuid.UUID]voting.IdVoteMap, len(agents))
	voteWeight := make(map[uuid.UUID]float64)
	for _, agent := range agents {
		voteWeight[agent.GetID()] = 1
		switch governance {
		case utils.Dictatorship:
			votes[agent.GetID()] = agent.VoteDictator()
		case utils.Leadership:
			votes[agent.GetID()] = agent.VoteLeader()
		}
	}

	IVotes := make(map[uuid.UUID]voting.IVoter, len(votes))
	for i, vote := range votes {
		IVotes[i] = vote
	}

	ruler := voting.WinnerFromDist(IVotes, voteWeight)
	return ruler
}

func (s *Server) RunDemocraticAction(bike objects.IMegaBike) uuid.UUID {
	fmt.Printf("In Run Decmocratic Action \n")
	// map of the proposed lootboxes by bike (for each bike a list of lootbox proposals is made, with one lootbox proposed by each agent on the bike)
	agents := bike.GetAgents()
	proposedDirections := make(map[uuid.UUID]uuid.UUID)
	for _, agent := range agents {
		// agents that have decided to stay on the bike (and that haven't been kicked off it)
		// will participate in the voting for the directions
		// ---------------------------VOTING ROUTINE - STEP 1 ---------------------
		if agent.GetBikeStatus() {
			proposedDirections[agent.GetID()] = agent.ProposeDirection()
		}
	}
	fmt.Printf("proposedDirections %v", proposedDirections)
	// pass the pitched directions of a bike to all agents on that bike and get their final vote
	finalVotes := make(map[uuid.UUID]voting.LootboxVoteMap, len(agents))
	for _, agent := range agents {
		// ---------------------------VOTING ROUTINE - STEP 2 ---------------------
		finalVotes[agent.GetID()] = agent.FinalDirectionVote(proposedDirections)
	}
	fmt.Printf("final votes %v", finalVotes)

	// ---------------------------VOTING ROUTINE - STEP 3 --------------
	direction := s.GetWinningDirection(finalVotes)
	fmt.Printf("winning direction %v", direction)
	return direction
}
