package team1

import (
	obj "SOMAS2023/internal/common/objects"
	"SOMAS2023/internal/common/physics"
	utils "SOMAS2023/internal/common/utils"
	voting "SOMAS2023/internal/common/voting"
	"fmt"
	"math"
	"sort"

	"github.com/MattSScott/basePlatformSOMAS/messaging"
	"github.com/google/uuid"
)

// agent specific parameters
const deviateNegative = -0.2     // trust loss on deviation
const deviatePositive = 0.1      // trust gain on non deviation
const effortScaling = 0.1        // scaling factor for effort, highr it is the more effort chages each round
const fairnessScaling = 0.1      // scaling factor for fairness, higher it is the more fairness changes each round
const leaveThreshold = 0.2       // threshold for leaving
const kickThreshhold = 0.4       // threshold for kicking
const fairnessConstant = 1       // weight of fairness in opinion
const trustconstant = 1          // weight of trust in opinion
const effortConstant = 1         // weight of effort in opinion
const fairnessDifference = 0.5   // modifies how much fairness increases of decreases, higher is more increase, 0.5 is fair
const lowEnergyLevel = 0.3       // energy level below which the agent will try to get a lootbox of the desired colour
const leavingThreshold = 0.3     // how low the agent's vote must be to leave bike
const colorOpinionConstant = 0.2 // how much any agent likes any other of the same colour in the objective function

// Governance decision constants
const democracyOpinonThreshold = 0.5
const democracyReputationThreshold = 0.3
const leadershipOpinionThreshold = 0.7
const leadershipReputationThreshold = 0.5
const dictatorshipOpinionThreshold = 0.9
const dictatorshipReputationThreshold = 0.7

type Opinion struct {
	effort   float64
	trust    float64
	fairness float64
	opinion  float64
}

type Biker1 struct {
	*obj.BaseBiker                       // BaseBiker inherits functions from BaseAgent such as GetID(), GetAllMessages() and UpdateAgentInternalState()
	recentVote     voting.LootboxVoteMap // the agent's most recent vote
	recentDecided  uuid.UUID             // the most recent decision
	dislikeVote    bool                  // whether the agent disliked the most recent vote
	opinions       map[uuid.UUID]Opinion
}

// -------------------SETTERS AND GETTERS-----------------------------
// Returns a list of bikers on the same bike as the agent
func (bb *Biker1) GetFellowBikers() []obj.IBaseBiker {
	gs := bb.GetGameState()
	bikeId := bb.GetBike()
	return gs.GetMegaBikes()[bikeId].GetAgents()
}

func (bb *Biker1) GetBikeInstance() obj.IMegaBike {
	gs := bb.GetGameState()
	bikeId := bb.GetBike()
	return gs.GetMegaBikes()[bikeId]
}

func (bb *Biker1) GetLootLocation(id uuid.UUID) utils.Coordinates {
	gs := bb.GetGameState()
	lootboxes := gs.GetLootBoxes()
	lootbox := lootboxes[id]
	return lootbox.GetPosition()
}

//-------------------END OF SETTERS AND GETTERS----------------------

// part 1:
// the biker itself doesn't technically have a location (as it's on the map only when it's on a bike)
// in fact this function is only called when the biker needs to make a decision about the pedaling forces
func (bb *Biker1) GetLocation() utils.Coordinates {
	gs := bb.GetGameState()
	bikeId := bb.GetMegaBikeId()
	megaBikes := gs.GetMegaBikes()
	return megaBikes[bikeId].GetPosition()
}

// Success-Relationship algo for calculating selfishness score
func calculateSelfishnessScore(success float64, relationship float64) float64 {
	difference := math.Abs(success - relationship)
	var overallScore float64
	if success >= relationship {
		overallScore = 0.5 + ((difference) / 2)
	} else if relationship > success {
		overallScore = 0.5 - ((difference) / 2)
	}
	return overallScore
}

// ---------------LOOT ALLOCATION FUNCTIONS------------------
// TODO FIX THIS
// through this function the agent submits their desired allocation of resources
// in the MVP each agent returns 1 whcih will cause the distribution to be equal across all of them
func (bb *Biker1) DecideAllocation() voting.IdVoteMap {
	fellowBikers := bb.GetFellowBikers()

	sumEnergyNeeds := 0.0
	helpfulAllocation := make(map[uuid.UUID]float64)
	selfishAllocation := make(map[uuid.UUID]float64)

	for _, agent := range fellowBikers {
		energyNeed := 1.0 - agent.GetEnergyLevel()
		helpfulAllocation[agent.GetID()] = energyNeed
		selfishAllocation[agent.GetID()] = energyNeed
		sumEnergyNeeds = sumEnergyNeeds + energyNeed
	}

	for agentId, _ := range helpfulAllocation {
		helpfulAllocation[agentId] /= sumEnergyNeeds
	}

	sumEnergyNeeds -= (1.0 - bb.GetEnergyLevel()) // remove our energy need from the sum

	for agentId, _ := range selfishAllocation {
		if agentId != bb.GetID() {
			selfishAllocation[agentId] = (selfishAllocation[agentId] / sumEnergyNeeds) * bb.GetEnergyLevel() //NB assuming energy is 0-1
		}
	}

	//3/4) Look in success vector to see relative success of each agent and calculate selfishness score using suc-rel chart (0-1)
	//TI - Around line 350, we have Soma`s pseudocode on agent opinion held in bb.Opinion.opinion, lets assume its normalized between 0-1
	selfishnessScore := make(map[uuid.UUID]float64)
	runningScore := 0.0

	for _, agent := range fellowBikers {
		if agent.GetID() != bb.GetID() {
			relativeSuccess := float64((agent.GetPoints() - bb.GetPoints()) / (bb.GetPoints() + agent.GetPoints())) //-1 to 1
			relativeSuccess = (relativeSuccess + 1.0) / 2.0                                                         //shift to 0-1
			id := agent.GetID()
			ourRelationship := bb.opinions[id].opinion
			selfishnessScore[id] = calculateSelfishnessScore(relativeSuccess, ourRelationship)
			runningScore = runningScore + selfishnessScore[id]
		}
	}

	selfishnessScore[bb.GetID()] = runningScore / float64((len(fellowBikers) - 1))

	//5) Linearly interpolate between selfish and helpful allocations based on selfishness score
	distribution := make(map[uuid.UUID]float64)
	runningDistribution := 0.0
	for _, agent := range fellowBikers {
		id := agent.GetID()
		Adistribution := (selfishnessScore[id] * selfishAllocation[id]) + ((1.0 - selfishnessScore[id]) * helpfulAllocation[id])
		distribution[id] = Adistribution
		runningDistribution = runningDistribution + Adistribution
	}
	for agentId, _ := range distribution {
		distribution[agentId] = distribution[agentId] / runningDistribution // Normalise!
	}
	return distribution
}

// ---------------END OF LOOT ALLOCATION FUNCTIONS------------------

// ---------------DIRECTION DECISION FUNCTIONS------------------

// Simulates a step of the game, assuming all bikers pedal with the same force as us.
// Returns the distance travelled and the remaining energy
func (bb *Biker1) simulateGameStep(energy float64, velocity float64, force float64) (float64, float64) {
	bikerNum := len(bb.GetFellowBikers())
	totalBikerForce := force * float64(len(bb.GetFellowBikers()))
	totalMass := utils.MassBike + float64(bikerNum)*utils.MassBiker
	acceleration := physics.CalcAcceleration(totalBikerForce, totalMass, velocity)
	distance := velocity + 0.5*acceleration
	energy = energy - force*utils.MovingDepletion
	return distance, energy
}

// Calculates the approximate distance that can be travelled with the given energy
func (bb *Biker1) energyToReachableDistance(energy float64) float64 {
	distance := 0.0
	totalDistance := 0.0
	remainingEnergy := energy
	for remainingEnergy > 0 {
		distance, remainingEnergy = bb.simulateGameStep(remainingEnergy, bb.GetBikeInstance().GetVelocity(), bb.getPedalForce())
		totalDistance = totalDistance + distance
	}
	return totalDistance
}

// Calculates the energy remaining after travelling the given distance
func (bb *Biker1) distanceToEnergy(distance float64, initialEnergy float64) float64 {
	totalDistance := 0.0
	remainingEnergy := initialEnergy
	for totalDistance < distance {
		distance, remainingEnergy = bb.simulateGameStep(remainingEnergy, bb.GetBikeInstance().GetPhysicalState().Mass, utils.BikerMaxForce*remainingEnergy)
	}
	// remainingEnergy := 1.0 //TODO: this is just for bug fix, REMOVE THIS LINE IN REAL AGENT

	return remainingEnergy
}

// Finds all boxes within our reachable distance
func (bb *Biker1) getAllReachableBoxes() []uuid.UUID {
	currLocation := bb.GetLocation()
	ourEnergy := bb.GetEnergyLevel()
	lootBoxes := bb.GetGameState().GetLootBoxes()
	reachableBoxes := make([]uuid.UUID, 0)
	var currDist float64
	for _, loot := range lootBoxes {
		lootPos := loot.GetPosition()
		currDist = physics.ComputeDistance(currLocation, lootPos)
		if currDist < bb.energyToReachableDistance(ourEnergy) {
			reachableBoxes = append(reachableBoxes, loot.GetID())
		}
	}
	return reachableBoxes
}

// Checks whether a box of the desired colour is within our reachable distance from a given box
func (bb *Biker1) checkBoxNearColour(box uuid.UUID, energy float64) bool {
	lootBoxes := bb.GetGameState().GetLootBoxes()
	boxPos := lootBoxes[box].GetPosition()
	var currDist float64
	for _, loot := range lootBoxes {
		lootPos := loot.GetPosition()
		currDist = physics.ComputeDistance(boxPos, lootPos)
		if currDist < bb.energyToReachableDistance(energy) && loot.GetColour() == bb.GetColour() {
			return true
		}
	}
	return false
}

// returns the nearest lootbox with respect to the agent's bike current position
// in the MVP this is used to determine the pedalling forces as all agent will be
// aiming to get to the closest lootbox by default
func (bb *Biker1) nearestLoot() uuid.UUID {
	currLocation := bb.GetLocation()
	shortestDist := math.MaxFloat64
	var nearestBox uuid.UUID
	var currDist float64
	for _, loot := range bb.GetGameState().GetLootBoxes() {
		x, y := loot.GetPosition().X, loot.GetPosition().Y
		currDist = math.Sqrt(math.Pow(currLocation.X-x, 2) + math.Pow(currLocation.Y-y, 2))
		if currDist < shortestDist {
			nearestBox = loot.GetID()
			shortestDist = currDist
		}
	}
	return nearestBox
}

// Finds the nearest reachable box
func (bb *Biker1) getNearestReachableBox() uuid.UUID {
	currLocation := bb.GetLocation()
	reachableBoxes := bb.getAllReachableBoxes()
	shortestDist := math.MaxFloat64
	//default to nearest box
	nearestBox := bb.nearestLoot()
	for _, loot := range reachableBoxes {
		lootPos := bb.GetLootLocation(loot)
		currDist := physics.ComputeDistance(currLocation, lootPos)
		if currDist < shortestDist {
			nearestBox = loot
		}
	}
	return nearestBox
}

// Finds the nearest lootbox of agent's colour
func (bb *Biker1) nearestLootColour() (uuid.UUID, float64) {
	currLocation := bb.GetLocation()
	shortestDist := math.MaxFloat64
	//default to nearest lootbox
	nearestBox := bb.nearestLoot()

	var currDist float64
	for id, loot := range bb.GetGameState().GetLootBoxes() {
		lootPos := loot.GetPosition()
		currDist = physics.ComputeDistance(currLocation, lootPos)
		if (currDist < shortestDist) && (loot.GetColour() == bb.GetColour()) {
			nearestBox = id
			shortestDist = currDist
		}
	}

	if shortestDist == math.MaxFloat64 {
		shortestDist = physics.ComputeDistance(bb.GetLootLocation(nearestBox), currLocation)
	}
	return nearestBox, shortestDist
}

func (bb *Biker1) ProposeDirection() uuid.UUID {
	fmt.Printf("agent %s Proposing direction \n", bb.GetID())
	// all logic for nominations goes in here
	// find nearest coloured box
	// if we can reach it, nominate it
	// if a box exists but we can't reach it, we nominate the box closest to that that we can reach
	// else, nominate nearest box TODO

	// necessary functions:
	// find nearest coloured box: DONE
	// for a box, see if we can reach it -> distance to box from us, our energy level -> function verifies if our energy means we can travel far enough to reach box
	// to do the above, need a function that converts energy to reachable distance
	// function to return nearest box in our reach to a box (our colour) that is out of reach
	// function that returns all the boxes we can reach

	nearestBox, distanceToNearestBox := bb.nearestLootColour()
	// TODO: check if nearestBox actually exists
	reachableDistance := bb.energyToReachableDistance(bb.GetEnergyLevel()) // TODO add all other biker energies
	if distanceToNearestBox < reachableDistance {
		return nearestBox
	}

	nearestReachableBox := bb.getNearestReachableBox()

	return nearestReachableBox
}
func (bb *Biker1) distanceToReachableBox(box uuid.UUID) float64 {
	currLocation := bb.GetLocation()
	boxPos := bb.GetGameState().GetLootBoxes()[box].GetPosition()
	currDist := physics.ComputeDistance(currLocation, boxPos)
	if currDist < bb.energyToReachableDistance(bb.GetEnergyLevel()) {
		return currDist
	}
	return -1.
}

func (bb *Biker1) findRemainingEnergyAfterReachingBox(box uuid.UUID) float64 {
	dist := physics.ComputeDistance(bb.GetLocation(), bb.GetGameState().GetLootBoxes()[box].GetPosition())
	remainingEnergy := bb.distanceToEnergy(dist, bb.GetEnergyLevel())
	return remainingEnergy
}

// this function will contain the agent's strategy on deciding which direction to go to
// the default implementation returns an equal distribution over all options
// this will also be tried as returning a rank of options
func (bb *Biker1) FinalDirectionVote(proposals []uuid.UUID) voting.LootboxVoteMap {
	fmt.Printf("agent %s FinalDirectionVote \n", bb.GetID())
	// add in voting logic using knowledge of everyone's nominations:

	// for all boxes, rule out any that you can't reach
	// if no boxes left, go for nearest one
	// else if you can reach a box, if someone else can't reach any boxes, vote the box nearest to them (altruistic - add later?)
	// else for every reachable box:
	// calculate energy left if you went there
	// function: calculate energy left given distance moved
	// scan area around box for other boxes based on energy left after reaching it
	// function: given energy and a coordinate on the map, get all boxes that are reachable from that coordinate
	// if our colour is in those boxes, assign the number of people who voted for that box as the score, else assign, 0
	// set highest score box to 1, rest to 0 (subject to change)

	votes := make(voting.LootboxVoteMap)
	maxDist := bb.energyToReachableDistance(bb.GetEnergyLevel())
	// pseudocode:
	// loop through proposals
	// for each box, add 1 to value of key=box_id in dict
	proposalVotes := make(map[uuid.UUID]int)
	for _, proposal := range proposals {
		_, ok := proposalVotes[proposal]
		if !ok {
			proposalVotes[proposal] = 1
		} else {
			proposalVotes[proposal] += 1
		}
	}

	for _, proposal := range proposals {
		distToBox := bb.distanceToReachableBox(proposal)
		if distToBox <= maxDist { //if reachable
			// if box is our colour and number of proposals is majority, make it 1, rest 0, return
			if bb.GetGameState().GetLootBoxes()[proposal].GetColour() == bb.GetColour() {
				if proposalVotes[proposal] > len(proposals)/2 {
					for _, proposal1 := range proposals {
						if proposal1 == proposal {
							votes[proposal1] = 1
						} else {
							votes[proposal1] = 0
						}
					}
					return votes
				}
			}
			// calculate energy left if we went here
			remainingEnergy := bb.findRemainingEnergyAfterReachingBox(proposal)
			// find nearest reachable boxes from current coordinate
			isColorNear := bb.checkBoxNearColour(proposal, remainingEnergy)
			// assign score of number of votes for this box if our colour is nearby
			//TODO FIX THIS
			if isColorNear {
				votes[proposal] = float64(proposalVotes[proposal])
			} else {
				votes[proposal] = 0.0
			}
		}
	}
	fmt.Printf("agent %s FinalDirectionVote: %v \n", bb.GetID(), votes)
	return votes
}

// -----------------END OF DIRECTION DECISION FUNCTIONS------------------

func (bb *Biker1) DecideAction() obj.BikerAction {
	fellowBikers := bb.GetFellowBikers()
	avg_opinion := 1.0
	for _, agent := range fellowBikers {
		avg_opinion = avg_opinion + bb.opinions[agent.GetID()].opinion
	}
	if (avg_opinion < leaveThreshold) || bb.dislikeVote {
		bb.dislikeVote = false
		return 1 //change bike
	} else {
		return 0 //pedal
	}
}

// -----------------PEDALLING FORCE FUNCTIONS------------------
func (bb *Biker1) getPedalForce() float64 {
	//can be made more complex
	return utils.BikerMaxForce * bb.GetEnergyLevel()
}

// determine the forces (pedalling, breaking and turning)
// in the MVP the pedalling force will be 1, the breaking 0 and the tunring is determined by the
// location of the nearest lootboX
// the function is passed in the id of the voted lootbox, for now ignored
func (bb *Biker1) DecideForce(direction uuid.UUID) {

	if bb.recentVote != nil {
		if bb.recentVote[direction] < leavingThreshold {
			bb.dislikeVote = true
		} else {
			bb.dislikeVote = false
		}
	}

	//agent doesn't rebel, just decides to leave next round if dislike vote
	lootBoxes := bb.GetGameState().GetLootBoxes()
	currLocation := bb.GetLocation()
	if len(lootBoxes) > 0 {
		targetPos := lootBoxes[direction].GetPosition()
		deltaX := targetPos.X - currLocation.X
		deltaY := targetPos.Y - currLocation.Y
		angle := math.Atan2(deltaX, deltaY)
		normalisedAngle := angle / math.Pi

		turningDecision := utils.TurningDecision{
			SteerBike:     true,
			SteeringForce: normalisedAngle,
		}
		boxForces := utils.Forces{
			Pedal:   bb.getPedalForce(),
			Brake:   0.0,
			Turning: turningDecision,
		}
		bb.SetForces(boxForces)
	} else { //shouldnt happen, but would just run from audi
		audiPos := bb.GetGameState().GetAudi().GetPosition()
		deltaX := audiPos.X - currLocation.X
		deltaY := audiPos.Y - currLocation.Y
		// Steer in opposite direction to audi
		angle := math.Atan2(-deltaX, -deltaY)
		normalisedAngle := angle / math.Pi
		turningDecision := utils.TurningDecision{
			SteerBike:     true,
			SteeringForce: normalisedAngle,
		}

		escapeAudiForces := utils.Forces{
			Pedal:   bb.getPedalForce(),
			Brake:   0.0,
			Turning: turningDecision,
		}
		bb.SetForces(escapeAudiForces)
	}

}

// -----------------END OF PEDALLING FORCE FUNCTIONS------------------

// -----------------OPINION FUNCTIONS------------------

func (bb *Biker1) UpdateEffort(agentID uuid.UUID) {
	agent := bb.GetAgentFromId(agentID)
	fellowBikers := bb.GetFellowBikers()
	totalPedalForce := 0.0
	for _, agent := range fellowBikers {
		totalPedalForce = totalPedalForce + agent.GetForces().Pedal
	}
	avgForce := totalPedalForce / float64(len(fellowBikers))
	//effort expectation is scaled by their energy level -- should it be? (*agent.GetEnergyLevel())
	finalEffort := bb.opinions[agent.GetID()].effort + (agent.GetForces().Pedal-avgForce)*effortScaling

	if finalEffort > 1 {
		finalEffort = 1
	}
	if finalEffort < 0 {
		finalEffort = 0
	}
	newOpinion := Opinion{
		effort:   finalEffort,
		fairness: bb.opinions[agentID].fairness,
		trust:    bb.opinions[agentID].trust,
		opinion:  bb.opinions[agentID].opinion,
	}
	bb.opinions[agent.GetID()] = newOpinion
}

func (bb *Biker1) UpdateTrust(agentID uuid.UUID) {
	id := agentID
	agent := bb.GetAgentFromId(agentID)
	finalTrust := 0.5
	if agent.GetForces().Turning.SteeringForce == bb.GetForces().Turning.SteeringForce {
		finalTrust = bb.opinions[id].trust + deviatePositive
		if finalTrust > 1 {
			finalTrust = 1
		}
	} else {
		finalTrust := bb.opinions[id].trust + deviateNegative
		if finalTrust < 0 {
			finalTrust = 0
		}
	}
	newOpinion := Opinion{
		effort:   bb.opinions[id].effort,
		fairness: bb.opinions[id].fairness,
		trust:    finalTrust,
		opinion:  bb.opinions[id].opinion,
	}
	bb.opinions[id] = newOpinion
}

// func (bb *Biker1) UpdateFairness(agent obj.IBaseBiker) {
// 	difference := 0.0
// 	agentVote := agent.DecideAllocation()
// 	fairVote := bb.DecideAllocation()
// 	//If anyone has a better solution fo this please do it, couldn't find a better way to substract two maps in go
// 	for i, theirVote := range agentVote {
// 		for j, ourVote := range fairVote {
// 			if i == j {
// 				difference = difference + math.Abs(ourVote - theirVote)
// 			}
// 		}
// 	}
// 	finalFairness := bb.opinions[agent.GetID()].fairness + (fairnessDifference - difference/2)*fairnessScaling

// 	if finalFairness > 1 {
// 		finalFairness = 1
// 	}
// 	if finalFairness < 0 {
// 		finalFairness = 0
// 	}
// 	agentID := agent.GetID()
// 	newOpinion := Opinion{
// 		effort:   bb.opinions[agentID].effort,
// 		fairness: finalFairness,
// 		trust:    bb.opinions[agentID].trust,
// 		opinion:  bb.opinions[agentID].opinion,
// 	}
// 	bb.opinions[agent.GetID()] = newOpinion
// }

// how well does agent 1 like agent 2 according to objective metrics
func (bb *Biker1) GetObjectiveOpinion(id1 uuid.UUID, id2 uuid.UUID) float64 {
	agent1 := bb.GetAgentFromId(id1)
	agent2 := bb.GetAgentFromId(id2)
	objOpinion := 0.0
	if agent1.GetColour() == agent2.GetColour() {
		objOpinion = objOpinion + colorOpinionConstant
	}
	objOpinion = objOpinion + (agent1.GetEnergyLevel() - agent2.GetEnergyLevel())
	gs := bb.GetGameState()
	megabikes := gs.GetMegaBikes()
	maxpoints := 0
	for _, bike := range megabikes {
		for _, agent := range bike.GetAgents() {
			if agent.GetPoints() > maxpoints {
				maxpoints = agent.GetPoints()
			}
		}
	}
	objOpinion = objOpinion + float64((agent1.GetPoints()-agent2.GetPoints())/maxpoints)
	objOpinion = math.Abs(objOpinion / (2.0 + colorOpinionConstant)) //normalise to 0-1
	return objOpinion
}

func (bb *Biker1) UpdateOpinions() {
	fellowBikers := bb.GetFellowBikers()
	for _, agent := range fellowBikers {
		id := agent.GetID()
		_, ok := bb.opinions[agent.GetID()]

		if !ok {
			agentId := agent.GetID()
			//if we have no data on an agent, initialise to neutral
			newOpinion := Opinion{
				effort:   0.5,
				trust:    0.5,
				fairness: 0.5,
				opinion:  0.5,
			}
			bb.opinions[agentId] = newOpinion
		}
		bb.UpdateTrust(id)
		bb.UpdateEffort(id)
		//bb.UpdateFairness(agent)

		//Sorry no youre right, keep it, silly me
		newOpinion := Opinion{
			effort:   bb.opinions[id].effort,
			trust:    bb.opinions[id].trust,
			fairness: bb.opinions[id].fairness,
			opinion:  (bb.opinions[id].trust*trustconstant+bb.opinions[id].effort*effortConstant+bb.opinions[id].fairness*fairnessConstant)/trustconstant + effortConstant + fairnessConstant,
		}
		bb.opinions[id] = newOpinion
	}
}

// Reputation = average opinion of all other agents
func (bb *Biker1) ourReputation() float64 {
	fellowBikers := bb.GetFellowBikers()
	reputation := 0.0
	for _, agent := range fellowBikers {
		reputation = reputation + bb.GetObjectiveOpinion(bb.GetID(), agent.GetID())

	}
	reputation = reputation / float64(len(fellowBikers))
	return reputation
}

// ----------------END OF OPINION FUNCTIONS--------------

// -----------------MESSAGING FUNCTIONS------------------

// Agent receives a who to kick off message
func (bb *Biker1) HandleKickOffMessage(msg KickOffAgentMessage) {
	sender := msg.sender

}

// Agent receives a reputation of another agent
func (bb *Biker1) HandleReputationMessage(msg ReputationOfAgentMessage) {
	sender := msg.sender

}

// Agent receives a message from another agent to join
func (bb *Biker1) HandleJoiningMessage(msg JoiningAgentMessage) {
	sender := msg.sender
}

// Agent receives a message from another agent say what lootbox they want to go to
func (bb *Biker1) HandleLootboxMessage(msg LootboxMessage) {
	sender := msg.sender
}

// Agent receives a message from another agent saying what Governance they want
func (bb *Biker1) HandleGovernanceMessage(msg GovernanceMessage) {
	sender := msg.sender
}

// Agent sending messages to other agents
func GetAllMessages([]IBaseBiker) []messaging.IMessage[IBaseBiker] {

}

// ----------------CHANGE BIKE FUNCTIONS-----------------
// define a sorter for bikes -> used to change bikes
type BikeSorter struct {
	bikes []bikeDistance
	by    func(b1, b2 *bikeDistance) bool
}

func (sorter *BikeSorter) Len() int {
	return len(sorter.bikes)
}
func (sorter *BikeSorter) Swap(i, j int) {
	sorter.bikes[i], sorter.bikes[j] = sorter.bikes[j], sorter.bikes[i]
}
func (sorter *BikeSorter) Less(i, j int) bool {
	return sorter.by(&sorter.bikes[i], &sorter.bikes[j])
}

type bikeDistance struct {
	bikeID   uuid.UUID
	bike     obj.IMegaBike
	distance float64
}
type By func(b1, b2 *bikeDistance) bool

func (by By) Sort(bikes []bikeDistance) {
	ps := &BikeSorter{
		bikes: bikes,
		by:    by,
	}
	sort.Sort(ps)
}

// Calculate how far we can jump for another bike -> based on energy level
func (bb *Biker1) GetMaxJumpDistance() float64 {
	//default to half grid size
	//TODO implement this
	return utils.GridHeight / 2
}
func (bb *Biker1) BikeOurColour(bike obj.IMegaBike) bool {
	matchCounter := 0
	totalAgents := len(bike.GetAgents())
	for _, agent := range bike.GetAgents() {
		bbColour := bb.GetColour()
		agentColour := agent.GetColour()
		if agentColour != bbColour {
			matchCounter++
		}
	}
	if matchCounter > totalAgents/2 {
		return true
	} else {
		return false
	}
}

// decide which bike to go to
func (bb *Biker1) ChangeBike() uuid.UUID {
	distance := func(b1, b2 *bikeDistance) bool {
		return b1.distance < b2.distance
	}
	gs := bb.GetGameState()
	allBikes := gs.GetMegaBikes()
	var bikeDistances []bikeDistance
	for id, bike := range allBikes {
		if len(bike.GetAgents()) < 8 {
			dist := physics.ComputeDistance(bb.GetLocation(), bike.GetPosition())
			if dist < bb.GetMaxJumpDistance() {
				bikeDistances = append(bikeDistances, bikeDistance{
					bikeID:   id,
					bike:     bike,
					distance: dist,
				})
			}

		}
	}

	By(distance).Sort(bikeDistances)
	for _, bike := range bikeDistances {
		if bb.BikeOurColour(bike.bike) {
			return bike.bikeID
		}
	}
	return bikeDistances[0].bikeID
}

// -------------------END OF CHANGE BIKE FUNCTIONS----------------------
func (bb *Biker1) GetAgentFromId(agentId uuid.UUID) obj.IBaseBiker {
	gs := bb.GetGameState()
	bikes := gs.GetMegaBikes()
	for bikeId := range bikes {
		for _, agents := range gs.GetMegaBikes()[bikeId].GetAgents() {
			if agents.GetID() == agentId {
				agent := agents
				return agent
			}
		}
	}
	return bb
}

// -------------------BIKER ACCEPTANCE FUNCTIONS------------------------
// an agent will have to rank the agents that are trying to join and that they will try to
func (bb *Biker1) DecideJoining(pendingAgents []uuid.UUID) map[uuid.UUID]bool {
	//gs.GetMegaBikes()[bikeId].GetAgents()

	decision := make(map[uuid.UUID]bool)

	for _, agentId := range pendingAgents {
		//TODO FIX
		agent := bb.GetAgentFromId(agentId)

		bbColour := bb.GetColour()
		agentColour := agent.GetColour()
		if agentColour == bbColour {
			decision[agentId] = true
		} else {
			decision[agentId] = false
		}
	}
	return decision
}

//--------------------END OF BIKER ACCEPTANCE FUNCTIONS-------------------

// -------------------GOVERMENT CHOICE FUNCTIONS--------------------------

// Not implemented on Server yet so this is just a placeholder
func (bb *Biker1) DecideGovernace() int {
	if bb.DecideDictatorship() {
		return 2
	} else if bb.DecideLeadership() {
		return 1
	} else {
		return 0
	}
}

// Might be unnecesary as this is the default goverment choice for us
func (bb *Biker1) DecideDemocracy() bool {
	fellowBikers := bb.GetFellowBikers()
	totalOpinion := 0.0
	reputation := bb.ourReputation()
	for _, agent := range fellowBikers {
		opinion, ok := bb.opinions[agent.GetID()]
		if ok {
			totalOpinion = totalOpinion + opinion.opinion
		}
	}
	normOpinion := totalOpinion / float64(len(fellowBikers))
	if (normOpinion > democracyOpinonThreshold) || (reputation > democracyReputationThreshold) {
		return true
	} else {
		return false
	}
}

func (bb *Biker1) DecideLeadership() bool {
	fellowBikers := bb.GetFellowBikers()
	totalOpinion := 0.0
	reputation := bb.ourReputation()
	for _, agent := range fellowBikers {
		opinion, ok := bb.opinions[agent.GetID()]
		if ok {
			totalOpinion = totalOpinion + opinion.opinion
		}
	}
	normOpinion := totalOpinion / float64(len(fellowBikers))
	if (normOpinion > leadershipOpinionThreshold) || (reputation > leadershipReputationThreshold) {
		return true
	} else {
		return false
	}
}

func (bb *Biker1) DecideDictatorship() bool {
	fellowBikers := bb.GetFellowBikers()
	totalOpinion := 0.0
	reputation := bb.ourReputation()
	for _, agent := range fellowBikers {
		opinion, ok := bb.opinions[agent.GetID()]
		if ok {
			totalOpinion = totalOpinion + opinion.opinion
		}
	}
	normOpinion := totalOpinion / float64(len(fellowBikers))
	if (normOpinion > dictatorshipOpinionThreshold) || (reputation > dictatorshipReputationThreshold) {
		return true
	} else {
		return false
	}
}

//--------------------END OF GOVERMENT CHOICE FUNCTIONS------------------

// -------------------INSTANTIATION FUNCTIONS----------------------------
func GetBiker1(colour utils.Colour, id uuid.UUID) *Biker1 {
	return &Biker1{
		BaseBiker: obj.GetBaseBiker(colour, id),
	}
}

// -------------------END OF INSTANTIATION FUNCTIONS---------------------
