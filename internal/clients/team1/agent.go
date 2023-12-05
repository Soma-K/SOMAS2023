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
const deviateNegative = -0.2         // trust loss on deviation
const deviatePositive = 0.1          // trust gain on non deviation
const effortScaling = 0.1            // scaling factor for effort, highr it is the more effort chages each round
const fairnessScaling = 0.1          // scaling factor for fairness, higher it is the more fairness changes each round
const relativeSuccessScaling = 0.1   // scaling factor for relative success, higher it is the more relative success changes each round
const leaveThreshold = 0.15          // threshold for leaving
const votingAlignmentThreshold = 0.1 // threshold for voting alignment
const kickThreshold = 0.0            // threshold for kicking
const trustThreshold = 0.7           // threshold for trusting (need to tune)
const fairnessConstant = 1           // weight of fairness in opinion
const joinThreshold = 0.8            // opinion threshold for joining if not same colour
const leaderThreshold = 0.95         // opinion threshold for becoming leader
const trustconstant = 1              // weight of trust in opinion
const effortConstant = 1             // weight of effort in opinion
const fairnessDifference = 0.5       // modifies how much fairness increases of decreases, higher is more increase, 0.5 is fair
const lowEnergyLevel = 0.3           // energy level below which the agent will try to get a lootbox of the desired colour
const leavingThreshold = 0.3         // how low the agent's vote must be to leave bike
const colorOpinionConstant = 0.2     // how much any agent likes any other of the same colour in the objective function

// Governance decision constants
const democracyOpinonThreshold = 0.5
const democracyReputationThreshold = 0.3
const leadershipOpinionThreshold = 0.7
const leadershipReputationThreshold = 0.5
const dictatorshipOpinionThreshold = 0.9
const dictatorshipReputationThreshold = 0.7

type Opinion struct {
	effort          float64
	trust           float64
	fairness        float64
	relativeSuccess float64
	// forgiveness float64
	opinion float64 // cumulative result of all the above
}

type Biker1 struct {
	*obj.BaseBiker                       // BaseBiker inherits functions from BaseAgent such as GetID(), GetAllMessages() and UpdateAgentInternalState()
	recentVote     voting.LootboxVoteMap // the agent's most recent vote
	recentDecided  uuid.UUID             // the most recent decision
	dislikeVote    bool                  // whether the agent disliked the most recent vote
	opinions       map[uuid.UUID]Opinion
	prevEnergy     map[uuid.UUID]float64 // energy level of each agent in the previous round

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

// func (bb *Biker1) GetAverageOpinionOfAgent(biker uuid.UUID) float64 {
// 	fellowBikers := bb.GetFellowBikers()
// 	opinionSum := 0.0
// 	for _, agent := range fellowBikers {
// 		opinionSum += agent.QueryReputation(biker)
// 	}
// 	return opinionSum / float64(len(fellowBikers))
// }

// -------------------END OF SETTERS AND GETTERS----------------------
// part 1:
// the biker itself doesn't technically have a location (as it's on the map only when it's on a bike)
// in fact this function is only called when the biker needs to make a decision about the pedaling forces
func (bb *Biker1) GetLocation() utils.Coordinates {
	gs := bb.GetGameState()
	bikeId := bb.GetBike()
	megaBikes := gs.GetMegaBikes()
	position := megaBikes[bikeId].GetPosition()
	if math.IsNaN(position.X) {
		fmt.Printf("agent %v has no position\n", bb.GetID())
	}
	return position
}

// // Success-Relationship algo for calculating selfishness score
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

func (bb *Biker1) GetSelfishness(agent obj.IBaseBiker) float64 {
	maxPoints := 0
	for _, agents := range bb.GetFellowBikers() {
		if agents.GetPoints() > maxPoints {
			maxPoints = agents.GetPoints()
		}
	}
	var relativeSuccess float64
	if maxPoints == 0 {
		relativeSuccess = 0.5
	} else {
		relativeSuccess = float64((agent.GetPoints() - bb.GetPoints()) / (maxPoints)) //-1 to 1
		relativeSuccess = (relativeSuccess + 1.0) / 2.0                               //shift to 0 to 1
	}
	id := agent.GetID()
	ourRelationship := bb.opinions[id].opinion
	return calculateSelfishnessScore(relativeSuccess, ourRelationship)
}

// ---------------LOOT ALLOCATION FUNCTIONS------------------
// through this function the agent submits their desired allocation of resources
// in the MVP each agent returns 1 whcih will cause the distribution to be equal across all of them

func (bb *Biker1) getHelpfulAllocation() map[uuid.UUID]float64 {
	fellowBikers := bb.GetFellowBikers()

	sumEnergyNeeds := 0.0
	helpfulAllocation := make(map[uuid.UUID]float64)
	for _, agent := range fellowBikers {
		// energyNeed := 1.0 - agent.GetEnergyLevel()
		energyNeed := 1.0 - bb.prevEnergy[agent.GetID()]
		helpfulAllocation[agent.GetID()] = energyNeed
		sumEnergyNeeds = sumEnergyNeeds + energyNeed
		for agentId, _ := range helpfulAllocation {
			helpfulAllocation[agentId] /= sumEnergyNeeds
		}
	}
	return helpfulAllocation
}

func (bb *Biker1) DecideAllocation() voting.IdVoteMap {
	fellowBikers := bb.GetFellowBikers()
	bb.UpdateAllAgentsEffort(fellowBikers)
	bb.UpdateAllAgentsTrust(fellowBikers)

	if len(fellowBikers) == 1 {
		return voting.IdVoteMap{bb.GetID(): 1}
	}

	sumEnergyNeeds := 0.0
	helpfulAllocation := make(map[uuid.UUID]float64)
	selfishAllocation := make(map[uuid.UUID]float64)

	for _, agent := range fellowBikers {
		energy := agent.GetEnergyLevel()
		energyNeed := 1.0 - energy
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
	selfishnessScore := make(map[uuid.UUID]float64)
	runningScore := 0.0

	for _, agent := range fellowBikers {
		if agent.GetID() != bb.GetID() {
			score := bb.GetSelfishness(agent)
			id := agent.GetID()
			selfishnessScore[id] = score
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
	if math.IsNaN(distribution[bb.GetID()]) {
		fmt.Println("Distribution is NaN")
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
	extraDist := 0.0
	for totalDistance < distance {
		extraDist, remainingEnergy = bb.simulateGameStep(remainingEnergy, bb.GetBikeInstance().GetPhysicalState().Mass, utils.BikerMaxForce*remainingEnergy)
		totalDistance = totalDistance + extraDist
	}

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
	shortestDist := math.MaxFloat64
	//default to nearest lootbox
	nearestBox := bb.GetID()
	var currDist float64
	initialized := false
	for id, loot := range bb.GetGameState().GetLootBoxes() {
		if !initialized {
			nearestBox = id
			initialized = true
		}
		lootPos := loot.GetPosition()
		currDist = physics.ComputeDistance(currLocation, lootPos)
		if currDist < shortestDist {
			nearestBox = id
			shortestDist = currDist
		}
	}

	return nearestBox
}

// Finds the nearest lootbox of agent's colour
func (bb *Biker1) nearestLootColour() (uuid.UUID, float64) {
	currLocation := bb.GetLocation()
	shortestDist := math.MaxFloat64
	//default to nearest lootbox
	nearestBox := bb.GetID()
	initialized := false
	var currDist float64
	for id, loot := range bb.GetGameState().GetLootBoxes() {
		if !initialized {
			nearestBox = id
			initialized = true
		}
		lootPos := loot.GetPosition()
		currDist = physics.ComputeDistance(currLocation, lootPos)
		if (currDist < shortestDist) && (loot.GetColour() == bb.GetColour()) {
			nearestBox = id
			shortestDist = currDist
		}
	}

	return nearestBox, shortestDist
}

func (bb *Biker1) ProposeDirection() uuid.UUID {
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
func (bb *Biker1) distanceToBox(box uuid.UUID) float64 {
	currLocation := bb.GetLocation()
	boxPos := bb.GetGameState().GetLootBoxes()[box].GetPosition()
	currDist := physics.ComputeDistance(currLocation, boxPos)
	return currDist
}

func (bb *Biker1) findRemainingEnergyAfterReachingBox(box uuid.UUID) float64 {
	dist := physics.ComputeDistance(bb.GetLocation(), bb.GetGameState().GetLootBoxes()[box].GetPosition())
	remainingEnergy := bb.distanceToEnergy(dist, bb.GetEnergyLevel())
	return remainingEnergy
}

// this function will contain the agent's strategy on deciding which direction to go to
// the default implementation returns an equal distribution over all options
// this will also be tried as returning a rank of options
func (bb *Biker1) FinalDirectionVote(proposals map[uuid.UUID]uuid.UUID) voting.LootboxVoteMap {
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
	// for each box, add 1 to value of key=box_id in dic
	proposalVotes := make(map[uuid.UUID]int)

	maxVotes := 1
	curVotes := 1
	for _, proposal := range proposals {
		_, ok := proposalVotes[proposal]
		if !ok {
			proposalVotes[proposal] = 1
		} else {
			proposalVotes[proposal] += 1
			if proposal != proposals[bb.GetID()] {
				curVotes = proposalVotes[proposal]
				if curVotes > maxVotes {
					maxVotes = curVotes
				}
			}
		}
	}
	distToBoxMap := make(map[uuid.UUID]float64)
	for _, proposal := range proposals {
		distToBoxMap[proposal] = bb.distanceToBox(proposal)
		if distToBoxMap[proposal] <= maxDist { //if reachable
			// if box is our colour and number of proposals is majority, make it 1, rest 0, return
			if bb.GetGameState().GetLootBoxes()[proposal].GetColour() == bb.GetColour() {
				if proposalVotes[proposal] >= maxVotes { // to c
					for _, proposal1 := range proposals {
						if proposal1 == proposal {
							votes[proposal1] = 1
						} else {
							votes[proposal1] = 0
						}
					}
					break
				} else {
					votes[proposal] = float64(proposalVotes[proposal])
				}
			}
			// calculate energy left if we went here
			remainingEnergy := bb.findRemainingEnergyAfterReachingBox(proposal)
			// find nearest reachable boxes from current coordinate
			isColourNear := bb.checkBoxNearColour(proposal, remainingEnergy)
			// assign score of number of votes for this box if our colour is nearby
			if isColourNear {
				votes[proposal] = float64(proposalVotes[proposal])
			} else {
				votes[proposal] = 0.0
			}
		} else {
			votes[proposal] = 0.0
		}
	}

	// Check if all votes are 0
	allVotesZero := true
	for _, value := range votes {
		if value != 0 {
			allVotesZero = false
			break
		}
	}

	// If all votes are 0, nominate the nearest box
	// Maybe nominate our box?
	if allVotesZero {
		minDist := math.MaxFloat64
		var nearestBox uuid.UUID
		for _, proposal := range proposals {
			if distToBoxMap[proposal] < minDist {
				minDist = distToBoxMap[proposal]
				nearestBox = proposal
			}
		}
		votes[nearestBox] = 1
		return votes
	}

	// Normalize the values in votes so that the values sum to 1
	sum := 0.0
	for _, value := range votes {
		sum += value
	}
	for key := range votes {
		votes[key] /= sum
	}
	bb.recentVote = votes
	return votes
}

// -----------------END OF DIRECTION DECISION FUNCTIONS------------------

func (bb *Biker1) updatePrevEnergy() {
	fellowBikers := bb.GetFellowBikers()
	for _, agent := range fellowBikers {
		bb.prevEnergy[agent.GetID()] = agent.GetEnergyLevel()
	}
}

func (bb *Biker1) DecideAction() obj.BikerAction {
	fellowBikers := bb.GetFellowBikers()
	bb.UpdateAllAgentOpinions(fellowBikers)
	bb.UpdateAllAgentsFairness(fellowBikers)
	avg_opinion := 0.0
	for _, agent := range fellowBikers {
		avg_opinion = avg_opinion + bb.opinions[agent.GetID()].opinion
	}
	if len(fellowBikers) > 0 {
		avg_opinion /= float64(len(fellowBikers))
	} else {
		avg_opinion = 1.0
	}
	if (avg_opinion < leaveThreshold) || bb.dislikeVote {
		// need to refresh prevEnergy map
		bb.prevEnergy = make(map[uuid.UUID]float64)
		bb.dislikeVote = false
		return 1
	} else {
		// add all energies of fellow bikers to prevEnergy map
		bb.updatePrevEnergy()
		return 0
	}
}

// // -----------------PEDALLING FORCE FUNCTIONS------------------
func (bb *Biker1) getPedalForce() float64 {
	//can be made more complex
	return utils.BikerMaxForce * bb.GetEnergyLevel()
}

// determine the forces (pedalling, breaking and turning)
// in the MVP the pedalling force will be 1, the breaking 0 and the tunring is determined by the
// location of the nearest lootboX
// the function is passed in the id of the voted lootbox, for now ignored
func (bb *Biker1) DecideForce(direction uuid.UUID) {
	bb.recentDecided = direction
	if bb.recentVote != nil {
		result, ok := bb.recentVote[direction]
		if ok && result < votingAlignmentThreshold {
			fmt.Printf("agent %v dislikes vote\n", bb.GetID())
			bb.dislikeVote = true
		} else {
			bb.dislikeVote = false
		}
	}

	//agent doesn't rebel, just decides to leave next round if dislike vote
	lootBoxes := bb.GetGameState().GetLootBoxes()
	currLocation := bb.GetLocation()
	targetPos := lootBoxes[direction].GetPosition()
	deltaX := targetPos.X - currLocation.X
	deltaY := targetPos.Y - currLocation.Y
	angle := math.Atan2(deltaY, deltaX)
	normalisedAngle := angle / math.Pi

	turningDecision := utils.TurningDecision{
		SteerBike:     true,
		SteeringForce: normalisedAngle - bb.GetBikeInstance().GetOrientation(),
	}
	boxForces := utils.Forces{
		Pedal:   bb.getPedalForce(),
		Brake:   0.0,
		Turning: turningDecision,
	}
	bb.SetForces(boxForces)
	// } else { //shouldnt happen, but would just run from audi
	// 	audiPos := bb.GetGameState().GetAudi().GetPosition()
	// 	deltaX := audiPos.X - currLocation.X
	// 	deltaY := audiPos.Y - currLocation.Y
	// 	// Steer in opposite direction to audi
	// 	angle := math.Atan2(-deltaY, -deltaX)
	// 	normalisedAngle := angle / math.Pi
	// 	turningDecision := utils.TurningDecision{
	// 		SteerBike:     true,
	// 		SteeringForce: normalisedAngle - bb.GetBikeInstance().GetOrientation(),
	// 	}

	// 	escapeAudiForces := utils.Forces{
	// 		Pedal:   bb.getPedalForce(),
	// 		Brake:   0.0,
	// 		Turning: turningDecision,
	// 	}
	// 	bb.SetForces(escapeAudiForces)
	// }
}

// -----------------END OF PEDALLING FORCE FUNCTIONS------------------

// -----------------OPINION FUNCTIONS------------------

func (bb *Biker1) UpdateEffort(agentID uuid.UUID) {
	agent := bb.GetAgentFromId(agentID)
	fellowBikers := bb.GetFellowBikers()
	bikeId := bb.GetBike()
	gs := bb.GetGameState()
	totalMass := utils.MassBike + float64(len(fellowBikers)+1)*utils.MassBiker
	totalPedalForce := gs.GetMegaBikes()[bikeId].GetPhysicalState().Acceleration * totalMass

	// Calculate force pedalled by everyone else
	remainingForce := totalPedalForce - bb.getPedalForce()
	effortProbability := make(map[uuid.UUID]float64) //probability that they are exc
	lootBoxes := bb.GetGameState().GetLootBoxes()
	totalEffort := 0.0
	for _, agent := range fellowBikers {
		colourProb := 0.0
		fmt.Printf("recently decided\n", bb.recentDecided)
		fmt.Printf("lootbox colour\n", lootBoxes[bb.recentDecided].GetColour())
		if agent.GetColour() != lootBoxes[bb.recentDecided].GetColour() {
			//probability should be high
			//for now set to 0.5 but later change based on how close the lootbox is to their colour lootbox
			colourProb += 0.5
		}
		energyProb := 1 - agent.GetEnergyLevel()
		//Will add weightings to this so that energy probability has a lower weighting than difference in colour for example
		//also plus reputation
		effortProb := 1 - (colourProb+energyProb)/2 //scales between 0 and 1 and then negative so that higher probabilities mean you are less likely to contribute to pedal force

		effortProbability[agent.GetID()] = effortProb
		totalEffort += effortProb
		//totalPedalForce = totalPedalForce + agent.GetForces().Pedal
		//if bike colour = agent colour, p of not pedalling = 0
	}
	for agentId := range effortProbability {
		effortProbability[agentId] /= totalEffort
		effortProbability[agentId] *= remainingForce
	}
	//
	//effort expectation is scaled by their energy and compare to our effort
	finalEffort := bb.opinions[agent.GetID()].effort + (effortProbability[agent.GetID()] - bb.getPedalForce()) //bb.relationships[agent.GetID()].effort + (agent.GetForces().Pedal-avgForce)*effortScaling

	if finalEffort > 1 {
		finalEffort = 1
	}
	if finalEffort < 0 {
		finalEffort = 0
	}
	newOpinion := Opinion{
		effort:          finalEffort,
		fairness:        bb.opinions[agentID].fairness,
		trust:           bb.opinions[agentID].trust,
		relativeSuccess: bb.opinions[agentID].relativeSuccess,
		opinion:         bb.opinions[agentID].opinion,
	}
	bb.opinions[agent.GetID()] = newOpinion
}

func (bb *Biker1) UpdateTrust(agentID uuid.UUID) {
	id := agentID
	finalTrust := bb.opinions[id].trust //nothing changes
	lootBoxes := bb.GetGameState().GetLootBoxes()
	targetPos := lootBoxes[bb.recentDecided].GetPosition()
	currLocation := bb.GetLocation()
	deltaX := targetPos.X - currLocation.X
	deltaY := targetPos.Y - currLocation.Y
	angle := math.Atan2(deltaY, deltaX)
	normalisedAngle := angle / math.Pi
	steeringForce := normalisedAngle - bb.GetBikeInstance().GetOrientation()
	if steeringForce == 0.0 { //we are headed in direction towards lootbox
		finalTrust = bb.opinions[id].trust + deviatePositive //will change to be based on weighting
	} else {
		//	need to estimate likelihood of each agent deviating from the correct steeringforce
		fellowBikers := bb.GetFellowBikers()
		for _, agent := range fellowBikers {
			if agent.GetColour() != lootBoxes[bb.recentDecided].GetColour() {
				//currently if its not the agent's colour then trust in them decreases
				//needs to include reputation somehow
				//needs to calculate orientation to their colour (is it closer to or further than (orientation wise) voted lootbox)
				finalTrust = bb.opinions[id].trust - deviateNegative
			}
		}
	}

	if finalTrust > 1 {
		finalTrust = 1
	} else if finalTrust < 0 {
		finalTrust = 0
	}
	newOpinion := Opinion{
		effort:          bb.opinions[id].effort,
		fairness:        bb.opinions[id].fairness,
		trust:           finalTrust,
		relativeSuccess: bb.opinions[id].relativeSuccess,
		opinion:         bb.opinions[id].opinion,
	}
	bb.opinions[id] = newOpinion
}

func (bb *Biker1) UpdateFairness(agentID uuid.UUID) {
	helpfulAllocation := bb.getHelpfulAllocation()
	//for now just implement for democracy
	agent := bb.GetAgentFromId(agentID)
	energyChange := agent.GetEnergyLevel() - bb.prevEnergy[agentID] //how much of lootx distribution they got
	finalFairness := bb.opinions[agent.GetID()].fairness
	if energyChange-helpfulAllocation[agentID] > 0 { //they have more than they should have fairly got
		finalFairness -= (energyChange - helpfulAllocation[agentID]) * fairnessScaling
	} else {
		finalFairness += ((1 - (energyChange - helpfulAllocation[agentID])) / 2) * fairnessScaling
	}

	if finalFairness > 1 {
		finalFairness = 1
	} else if finalFairness < 0 {
		finalFairness = 0
	}

	newOpinion := Opinion{
		effort:          bb.opinions[agentID].effort,
		fairness:        finalFairness,
		trust:           bb.opinions[agentID].trust,
		relativeSuccess: bb.opinions[agentID].relativeSuccess,
		opinion:         bb.opinions[agentID].opinion,
	}
	bb.opinions[agentID] = newOpinion
}

func (bb *Biker1) UpdateRelativeSuccess(agentID uuid.UUID) {
	relativeSuccess := bb.GetRelativeSuccess(bb.GetID(), agentID)
	finalRelativeSuccess := bb.opinions[agentID].relativeSuccess + (relativeSuccess-bb.opinions[agentID].relativeSuccess)*relativeSuccessScaling
	if finalRelativeSuccess > 1 {
		finalRelativeSuccess = 1
	}
	if finalRelativeSuccess < 0 {
		finalRelativeSuccess = 0
	}
	newOpinion := Opinion{
		effort:          bb.opinions[agentID].effort,
		fairness:        bb.opinions[agentID].fairness,
		trust:           bb.opinions[agentID].trust,
		relativeSuccess: finalRelativeSuccess,
		opinion:         bb.opinions[agentID].opinion,
	}
	bb.opinions[agentID] = newOpinion
}

// how well does agent 1 like agent 2 according to objective metrics
func (bb *Biker1) GetRelativeSuccess(id1 uuid.UUID, id2 uuid.UUID) float64 {
	agent1 := bb.GetAgentFromId(id1)
	agent2 := bb.GetAgentFromId(id2)
	relativeSuccess := 0.0
	if agent1.GetColour() == agent2.GetColour() {
		relativeSuccess = relativeSuccess + colorOpinionConstant
	}
	relativeSuccess = relativeSuccess + (agent1.GetEnergyLevel() - agent2.GetEnergyLevel())
	all_agents := bb.GetAllAgents()
	maxpoints := 0
	for _, agent := range all_agents {
		if agent.GetPoints() > maxpoints {
			maxpoints = agent.GetPoints()
		}
	}
	if maxpoints != 0 {
		relativeSuccess = relativeSuccess + float64((agent1.GetPoints()-agent2.GetPoints())/maxpoints)
	}
	relativeSuccess = math.Abs(relativeSuccess / (2.0 + colorOpinionConstant)) //normalise to 0-1
	return relativeSuccess
}

func (bb *Biker1) UpdateOpinion(id uuid.UUID, multiplier float64) {
	_, ok := bb.opinions[id]
	if !ok {
		//if we have no data on an agent, initialise to neutral
		newOpinion := Opinion{
			effort:          0.5,
			trust:           0.5,
			fairness:        0.5,
			relativeSuccess: 0.5,
			opinion:         0.5,
		}
		bb.opinions[id] = newOpinion
	}

	newOpinion := Opinion{
		effort:          bb.opinions[id].effort,
		trust:           bb.opinions[id].trust,
		fairness:        bb.opinions[id].fairness,
		relativeSuccess: bb.opinions[id].relativeSuccess,
		opinion:         ((bb.opinions[id].trust*trustconstant + bb.opinions[id].effort*effortConstant + bb.opinions[id].fairness*fairnessConstant) / (trustconstant + effortConstant + fairnessConstant)) * multiplier,
	}

	if newOpinion.opinion > 1 {
		newOpinion.opinion = 1
	} else if newOpinion.opinion < 0 {
		newOpinion.opinion = 0
	}
	bb.opinions[id] = newOpinion

}

// infer our reputation from the average relative success of agents in the current context
func (bb *Biker1) DetermineOurReputation() float64 {
	var agentsInContext []obj.IBaseBiker
	if bb.GetBike() == uuid.Nil {
		agentsInContext = bb.GetAllAgents()
	} else {
		agentsInContext = bb.GetFellowBikers()
	}

	reputation := 0.0
	for _, agent := range agentsInContext {
		reputation = reputation + bb.GetRelativeSuccess(bb.GetID(), agent.GetID())
	}
	fmt.Printf("Reputation: %v\n", reputation)
	reputation = reputation / float64(len(agentsInContext))
	return reputation
}

func (bb *Biker1) setOpinions() {
	if bb.opinions == nil {
		bb.opinions = make(map[uuid.UUID]Opinion)
	}
}

func (bb *Biker1) UpdateAllAgentOpinions(agents_to_update []obj.IBaseBiker) {
	bb.setOpinions()
	for _, agent := range agents_to_update {
		id := agent.GetID()
		_, ok := bb.opinions[agent.GetID()]

		if !ok {
			agentId := agent.GetID()
			//if we have no data on an agent, initialise to neutral
			newRelationship := Opinion{
				effort:          0.5,
				trust:           0.5,
				fairness:        0.5,
				relativeSuccess: 0.5,
				opinion:         0.5,
			}
			bb.opinions[agentId] = newRelationship
		}
		bb.UpdateOpinion(id, 1.0)
	}
}

func (bb *Biker1) UpdateAllAgentsEffort(agents_to_update []obj.IBaseBiker) {
	bb.setOpinions()
	for _, agent := range agents_to_update {
		id := agent.GetID()
		_, ok := bb.opinions[agent.GetID()]

		if !ok {
			agentId := agent.GetID()
			//if we have no data on an agent, initialise to neutral
			newRelationship := Opinion{
				effort:          0.5,
				trust:           0.5,
				fairness:        0.5,
				relativeSuccess: 0.5,
				opinion:         0.5,
			}
			bb.opinions[agentId] = newRelationship
		}
		bb.UpdateEffort(id)
	}
}

func (bb *Biker1) UpdateAllAgentsTrust(agents_to_update []obj.IBaseBiker) {
	bb.setOpinions()
	for _, agent := range agents_to_update {
		id := agent.GetID()
		_, ok := bb.opinions[agent.GetID()]

		if !ok {
			agentId := agent.GetID()
			//if we have no data on an agent, initialise to neutral
			newRelationship := Opinion{
				effort:          0.5,
				trust:           0.5,
				fairness:        0.5,
				relativeSuccess: 0.5,
				opinion:         0.5,
			}
			bb.opinions[agentId] = newRelationship
		}
		bb.UpdateTrust(id)
	}
}

func (bb *Biker1) UpdateAllAgentsFairness(agents_to_update []obj.IBaseBiker) {
	bb.setOpinions()
	for _, agent := range agents_to_update {
		id := agent.GetID()
		_, ok := bb.opinions[agent.GetID()]

		if !ok {
			agentId := agent.GetID()
			//if we have no data on an agent, initialise to neutral
			newRelationship := Opinion{
				effort:          0.5,
				trust:           0.5,
				fairness:        0.5,
				relativeSuccess: 0.5,
				opinion:         0.5,
			}
			bb.opinions[agentId] = newRelationship
		}
		bb.UpdateFairness(id)
	}
}

func (bb *Biker1) UpdateAllAgentsRelativeSuccess(agents_to_update []obj.IBaseBiker) {
	bb.setOpinions()
	for _, agent := range agents_to_update {
		id := agent.GetID()
		_, ok := bb.opinions[agent.GetID()]

		if !ok {
			agentId := agent.GetID()
			//if we have no data on an agent, initialise to neutral
			newRelationship := Opinion{
				effort:          0.5,
				trust:           0.5,
				fairness:        0.5,
				relativeSuccess: 0.5,
				opinion:         0.5,
			}
			bb.opinions[agentId] = newRelationship
		}
		bb.UpdateRelativeSuccess(id)
	}
}

// ----------------END OF OPINION FUNCTIONS--------------

// -----------------MESSAGING FUNCTIONS------------------

// Handle a message received from anyone, ensuring they are trustworthy and come from the right place (e.g. our bike)
func (bb *Biker1) VerifySender(sender obj.IBaseBiker) bool {
	// check if sender is on our bike
	if sender.GetBike() == bb.GetBike() {
		// check if sender is trustworthy
		if bb.opinions[sender.GetID()].trust > trustThreshold {
			return true
		}
	}
	return false
}

// Agent receives a who to kick off message
func (bb *Biker1) HandleKickOffMessage(msg obj.KickOffAgentMessage) {
	sender := msg.GetSender()
	verified := bb.VerifySender(sender)
	if verified {
		// slightly penalise view of person who sent message
		penalty := 0.9
		bb.UpdateOpinion(sender.GetID(), penalty)
	}

}

// Agent receives a reputation of another agent
func (bb *Biker1) HandleReputationMessage(msg obj.ReputationOfAgentMessage) {
	sender := msg.GetSender()
	verified := bb.VerifySender(sender)
	if verified {
		// TODO: SOME FORMULA TO UPDATE OPINION BASED ON REPUTATION given
	}
}

// Agent receives a message from another agent to join
func (bb *Biker1) HandleJoiningMessage(msg obj.JoiningAgentMessage) {
	sender := msg.GetSender()
	// check if sender is trustworthy
	if bb.opinions[sender.GetID()].trust > trustThreshold {
		// TODO: some update on opinon maybe???
	}

}

// Agent receives a message from another agent say what lootbox they want to go to
func (bb *Biker1) HandleLootboxMessage(msg obj.LootboxMessage) {
	sender := msg.GetSender()
	verified := bb.VerifySender(sender)
	if verified {
		// TODO: some update on lootbox decision maybe??
	}
}

// Agent receives a message from another agent saying what Governance they want
func (bb *Biker1) HandleGovernanceMessage(msg obj.GovernanceMessage) {
	sender := msg.GetSender()
	verified := bb.VerifySender(sender)
	if verified {
		// TODO: some update on governance decision maybe??
	}
}

func (bb *Biker1) GetTrustedRecepients() []obj.IBaseBiker {
	fellowBikers := bb.GetFellowBikers()
	var trustedRecepients []obj.IBaseBiker
	for _, agent := range fellowBikers {
		if bb.opinions[agent.GetID()].trust > trustThreshold {
			trustedRecepients = append(trustedRecepients, agent)
		}
	}
	return trustedRecepients
}

// CREATING MESSAGES
func (bb *Biker1) CreateKickOffMessage() obj.KickOffAgentMessage {
	// Receipients = fellowBikers
	agentToKick := bb.lowestOpinionKick()
	var kickDecision bool
	// send kick off message if we have a low opinion of someone
	if agentToKick != uuid.Nil {
		kickDecision = true
	} else {
		kickDecision = false
	}

	return obj.KickOffAgentMessage{
		BaseMessage: messaging.CreateMessage[obj.IBaseBiker](bb, bb.GetTrustedRecepients()),
		AgentId:     agentToKick,
		KickOff:     kickDecision,
	}
}

func (bb *Biker1) CreateReputationMessage() obj.ReputationOfAgentMessage {
	// Tell the truth (for now)
	// TODO: receipients = fellowBikers that we trust?
	return obj.ReputationOfAgentMessage{
		BaseMessage: messaging.CreateMessage[obj.IBaseBiker](bb, bb.GetTrustedRecepients()),
		AgentId:     uuid.Nil,
		Reputation:  1.0,
	}
}

func (bb *Biker1) CreateJoiningMessage() obj.JoiningAgentMessage {
	// Tell the truth (for now)
	// receipients = fellowBikers
	biketoJoin := bb.ChangeBike()
	gs := bb.GetGameState()
	joiningBike := gs.GetMegaBikes()[biketoJoin]
	return obj.JoiningAgentMessage{
		BaseMessage: messaging.CreateMessage[obj.IBaseBiker](bb, joiningBike.GetAgents()),
		AgentId:     bb.GetID(),
		BikeId:      biketoJoin,
	}
}
func (bb *Biker1) CreateLootboxMessage() obj.LootboxMessage {
	// Tell the truth (for now)
	// receipients = fellowBikers
	chosenLootbox := bb.ProposeDirection()
	return obj.LootboxMessage{
		BaseMessage: messaging.CreateMessage[obj.IBaseBiker](bb, bb.GetTrustedRecepients()),
		LootboxId:   chosenLootbox,
	}
}

func (bb *Biker1) CreateGoverenceMessage() obj.GovernanceMessage {
	// Tell the truth (using same logic as deciding governance for voting) (for now)
	// receipients = fellowBikers
	chosenGovernance := bb.DecideGovernance()
	// convert to int for now
	return obj.GovernanceMessage{
		BaseMessage:  messaging.CreateMessage[obj.IBaseBiker](bb, bb.GetTrustedRecepients()),
		BikeId:       bb.GetBike(),
		GovernanceId: int(chosenGovernance),
	}
}

// Agent sending messages to other agents
func (bb *Biker1) GetAllMessages([]obj.IBaseBiker) []messaging.IMessage[obj.IBaseBiker] {
	var sendKickMessage, sendReputationMessage, sendJoiningMessage, sendLootboxMessage, sendGovernanceMessage bool

	// TODO: add logic to decide which messages to send and when

	var messageList []messaging.IMessage[obj.IBaseBiker]
	if sendKickMessage {
		messageList = append(messageList, bb.CreateKickOffMessage())
	}
	if sendReputationMessage {
		messageList = append(messageList, bb.CreateReputationMessage())
	}
	if sendJoiningMessage {
		messageList = append(messageList, bb.CreateJoiningMessage())
	}
	if sendLootboxMessage {
		messageList = append(messageList, bb.CreateLootboxMessage())

	}
	if sendGovernanceMessage {
		messageList = append(messageList, bb.CreateGoverenceMessage())

	}
	return messageList
}

// -----------------END MESSAGING FUNCTIONS------------------

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
		if len(bike.GetAgents()) < 8 && id != bb.GetBike() {
			dist := physics.ComputeDistance(bb.GetLocation(), bike.GetPosition())
			bikeDistances = append(bikeDistances, bikeDistance{
				bikeID:   id,
				bike:     bike,
				distance: dist,
			})

		}
	}
	By(distance).Sort(bikeDistances)
	for _, bike := range bikeDistances {
		if bb.BikeOurColour(bike.bike) {
			return bike.bikeID
		}
	}
	if len(bikeDistances) == 0 {
		return bb.GetBike()
	}
	return bikeDistances[0].bikeID
}

// -------------------END OF CHANGE BIKE FUNCTIONS----------------------

// Find an agent from their id
func (bb *Biker1) GetAgentFromId(agentId uuid.UUID) obj.IBaseBiker {
	agents := bb.GetAllAgents()
	for _, agent := range agents {
		if agent.GetID() == agentId {
			return agent
		}
	}
	return nil
}

// Get all agents in the game
func (bb *Biker1) GetAllAgents() []obj.IBaseBiker {
	gs := bb.GetGameState()
	// get all agents
	agents := make([]obj.IBaseBiker, 0)
	for _, agent := range gs.GetAgents() {
		agents = append(agents, agent)
	}
	return agents
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
			sameColourReward := 1.05
			bb.UpdateOpinion(agentId, sameColourReward)
		} else {
			if bb.opinions[agentId].opinion > joinThreshold {
				decision[agentId] = true
				// penalise for accepting them without same colour
				penalty := 0.9
				bb.UpdateOpinion(agentId, penalty)
			}
			decision[agentId] = false
		}
		bb.UpdateRelativeSuccess(agentId)

	}

	for _, agentId := range pendingAgents {
		decision[agentId] = true
	}
	return decision
}

func (bb *Biker1) lowestOpinionKick() uuid.UUID {
	fellowBikers := bb.GetFellowBikers()
	lowestOpinion := kickThreshold
	var worstAgent uuid.UUID
	for _, agent := range fellowBikers {
		if bb.opinions[agent.GetID()].opinion < lowestOpinion {
			lowestOpinion = bb.opinions[agent.GetID()].opinion
			worstAgent = agent.GetID()
		}
	}
	// if we want to kick someone based on our threshold, return their id, else return nil
	if lowestOpinion < kickThreshold {
		return worstAgent
	}
	return uuid.Nil
}

func (bb *Biker1) DecideKick(agent uuid.UUID) int {
	if bb.opinions[agent].opinion < kickThreshold {
		return 1
	}
	return 0
}

func (bb *Biker1) VoteForKickout() map[uuid.UUID]int {
	voteResults := make(map[uuid.UUID]int)
	fellowBikers := bb.GetFellowBikers()
	for _, agent := range fellowBikers {
		agentID := agent.GetID()
		if agentID != bb.GetID() {
			// random votes to other agents
			voteResults[agentID] = bb.DecideKick(agentID)
		}
	}
	return voteResults
}

//--------------------END OF BIKER ACCEPTANCE FUNCTIONS-------------------

// -------------------GOVERMENT CHOICE FUNCTIONS--------------------------

// Not implemented on Server yet so this is just a placeholder
func (bb *Biker1) DecideGovernance() utils.Governance {
	if bb.DecideDictatorship() {
		return 2
	} else if bb.DecideLeadership() {
		return 1
	} else {
		// Democracy
		return 0
	}
}

// Might be unnecesary as this is the default goverment choice for us
func (bb *Biker1) DecideDemocracy() bool {
	founding_agents := bb.GetAllAgents()
	totalOpinion := 0.0
	reputation := bb.DetermineOurReputation()
	for _, agent := range founding_agents {
		opinion, ok := bb.opinions[agent.GetID()]
		if ok {
			totalOpinion = totalOpinion + opinion.opinion
		}
	}
	normOpinion := totalOpinion / float64(len(founding_agents))
	if (normOpinion > democracyOpinonThreshold) || (reputation > democracyReputationThreshold) {
		return true
	} else {
		return false
	}
}

func (bb *Biker1) DecideLeadership() bool {
	founding_agents := bb.GetAllAgents()
	totalOpinion := 0.0
	reputation := bb.DetermineOurReputation()
	for _, agent := range founding_agents {
		opinion, ok := bb.opinions[agent.GetID()]
		if ok {
			totalOpinion = totalOpinion + opinion.opinion
		}
	}
	normOpinion := totalOpinion / float64(len(founding_agents))
	if (normOpinion > leadershipOpinionThreshold) || (reputation > leadershipReputationThreshold) {
		return true
	} else {
		return false
	}
}

func (bb *Biker1) DecideDictatorship() bool {
	founding_agents := bb.GetAllAgents()
	totalOpinion := 0.0
	reputation := bb.DetermineOurReputation()
	for _, agent := range founding_agents {
		opinion, ok := bb.opinions[agent.GetID()]
		if ok {
			totalOpinion = totalOpinion + opinion.opinion
		}
	}
	normOpinion := totalOpinion / float64(len(founding_agents))
	if (normOpinion > dictatorshipOpinionThreshold) || (reputation > dictatorshipReputationThreshold) {
		return true
	} else {
		return false
	}
}

// ----------------------LEADER/DICTATOR VOTING FUNCTIONS------------------
func (bb *Biker1) VoteLeader() voting.IdVoteMap {

	votes := make(voting.IdVoteMap)
	fellowBikers := bb.GetFellowBikers()

	maxOpinion := 0.8
	leaderVote := bb.GetID()
	for _, agent := range fellowBikers {
		votes[agent.GetID()] = 0.0
		avgOp := 0.0
		if agent.GetID() != bb.GetID() {
			val, ok := bb.opinions[agent.GetID()]
			if ok {
				avgOp = (avgOp + val.opinion) / 2
			}
		}
		if avgOp > maxOpinion {
			maxOpinion = avgOp
			leaderVote = agent.GetID()
		}
	}
	votes[leaderVote] = 1.0
	return votes
}

func (bb *Biker1) VoteDictator() voting.IdVoteMap {
	votes := make(voting.IdVoteMap)
	fellowBikers := bb.GetFellowBikers()

	maxOpinion := 0.0
	leaderVote := bb.GetID()
	for _, agent := range fellowBikers {
		votes[agent.GetID()] = 0.0
		// avgOp := bb.GetAverageOpinionOfAgent(agent.GetID())
		avgOp := 0.0
		if agent.GetID() != bb.GetID() {
			val, ok := bb.opinions[agent.GetID()]
			if ok {
				avgOp = (avgOp + 3*val.opinion) / 4
			}
		}
		if avgOp > maxOpinion {
			maxOpinion = avgOp
			leaderVote = agent.GetID()
		}
	}
	votes[leaderVote] = 1.0
	return votes
}

//--------------------END OF LEADER/DICTATOR VOTING FUNCTIONS------------------

//--------------------DICTATOR FUNCTIONS------------------

// ** called only when the agent is the dictator
func (bb *Biker1) DictateDirection() uuid.UUID {
	// TODO: make more sophisticated
	tmp, _ := bb.nearestLootColour()
	return tmp
}

// ** decide which agents to kick out (dictator)
func (bb *Biker1) DecideKickOut() []uuid.UUID {

	// TODO: make more sophisticated
	tmp := []uuid.UUID{}
	tmp = append(tmp, bb.lowestOpinionKick())
	return tmp
}

// ** decide the allocation (dictator)
func (bb *Biker1) DecideDictatorAllocation() voting.IdVoteMap {
	return bb.DecideAllocation()
}

//--------------------END OF DICTATOR FUNCTIONS------------------

// --------------------LEADER FUNCTIONS------------------
func (bb *Biker1) DecideWeights(action utils.Action) map[uuid.UUID]float64 {
	// decides the weights of other peoples votes
	// Leadership democracy
	// takes in proposed action as a parameter
	// only run for the leader after everyone's proposeDirection is run
	// assigns vector of weights to everyone's proposals, 0.5 is neutral

	//consider adding weights for agents with low points
	fellowBikers := bb.GetFellowBikers()
	weights := map[uuid.UUID]float64{}

	for _, agent := range fellowBikers {
		op, ok := bb.opinions[agent.GetID()]
		if !ok {
			weights[agent.GetID()] = 0.5
		} else {
			weights[agent.GetID()] = op.opinion
		}
	}
	return weights
}

//--------------------END OF LEADER FUNCTIONS------------------
//--------------------END OF GOVERMENT CHOICE FUNCTIONS------------------

// ---------------------SOCIAL FUNCTIONS (ALL WRONG)------------------------
// get reputation value of all other agents
func (bb *Biker1) GetReputation() map[uuid.UUID]float64 {
	reputation := map[uuid.UUID]float64{}
	for agent, opinion := range bb.opinions {
		reputation[agent] = opinion.opinion
	}
	return reputation
}

// query for reputation value of specific agent with UUID
func (bb *Biker1) QueryReputation(agent uuid.UUID) float64 {
	val, ok := bb.opinions[agent]
	if ok {
		return val.opinion
	} else {
		return 0.5
	}
}

// set reputation value of specific agent with UUID
func (bb *Biker1) SetReputation(agent uuid.UUID, reputation float64) {
	bb.opinions[agent] = Opinion{
		effort:   bb.opinions[agent].effort,
		trust:    bb.opinions[agent].trust,
		fairness: bb.opinions[agent].fairness,
		opinion:  reputation,
	}
}

//---------------------END OF SOCIAL FUNCTIONS------------------------

// -------------------INSTANTIATION FUNCTIONS----------------------------
func GetBiker1(colour utils.Colour, id uuid.UUID) *Biker1 {
	return &Biker1{
		BaseBiker:   obj.GetBaseBiker(colour, id),
		opinions:    make(map[uuid.UUID]Opinion),
		dislikeVote: false,
		prevEnergy:  make(map[uuid.UUID]float64),
	}
}

// -------------------END OF INSTANTIATION FUNCTIONS---------------------
